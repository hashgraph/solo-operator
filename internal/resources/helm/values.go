// SPDX-License-Identifier: Apache-2.0

// Package helm provides the Helm release subreconciler, client interface, and value
// injection logic for the HelmCapsule controller.
package helm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const defaultValuesKey = "values.yaml"

// MergeValues builds the final Helm values map, merging three layers in order:
//  1. Auto-injected topology values (from Orbit + ConsensusCapsules)
//  2. ValuesFrom references (ConfigMaps / Secrets)
//  3. Inline Values (highest priority, always wins)
func MergeValues(ctx context.Context, c client.Client, namespace string, spec *operatorv1alpha1.HelmCapsuleSpec) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	if spec.Component != nil && spec.OrbitRef != nil {
		injected, err := injectComponentValues(ctx, c, namespace, spec)
		if err != nil {
			return nil, fmt.Errorf("injecting component values: %w", err)
		}
		result = mergeMaps(result, injected)
	}

	for i, ref := range spec.ValuesFrom {
		loaded, err := loadValuesReference(ctx, c, namespace, ref)
		if err != nil {
			return nil, fmt.Errorf("loading valuesFrom[%d] (%s/%s): %w", i, ref.Kind, ref.Name, err)
		}
		result = mergeMaps(result, loaded)
	}

	if spec.Values != nil && len(spec.Values.Raw) > 0 {
		var inline map[string]interface{}
		if err := json.Unmarshal(spec.Values.Raw, &inline); err != nil {
			return nil, fmt.Errorf("unmarshalling inline values: %w", err)
		}
		result = mergeMaps(result, inline)
	}

	return result, nil
}

// ComputeHash returns a SHA-256 digest of the chart spec + merged values.
// Stored in status.ValuesHash so the controller can skip upgrades when nothing changed.
func ComputeHash(chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}) (string, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshalling values for hash: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(chart.Repo))
	h.Write([]byte(chart.Chart))
	h.Write([]byte(chart.Version))
	h.Write(valuesJSON)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ─── Per-component injection ──────────────────────────────────────────────────

func injectComponentValues(ctx context.Context, c client.Client, namespace string, spec *operatorv1alpha1.HelmCapsuleSpec) (map[string]interface{}, error) {
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, types.NamespacedName{Name: spec.OrbitRef.Name}, orbit); err != nil {
		return nil, fmt.Errorf("getting orbit %s: %w", spec.OrbitRef.Name, err)
	}

	ledgerID := orbit.Spec.Consensus.NetworkSettings["ledger.id"]

	capsules, err := listConsensusCapsules(ctx, c, namespace, spec.OrbitRef.Name)
	if err != nil {
		return nil, err
	}

	switch *spec.Component {
	case operatorv1alpha1.HelmComponentMirror:
		return mirrorValues(capsules, orbit), nil
	case operatorv1alpha1.HelmComponentRelay, operatorv1alpha1.HelmComponentRelayWebSocket:
		return relayValues(capsules, namespace, ledgerID), nil
	case operatorv1alpha1.HelmComponentExplorer:
		return explorerValues(orbit, ledgerID), nil
	case operatorv1alpha1.HelmComponentBlockNode:
		return blockNodeValues(capsules), nil
	default:
		return map[string]interface{}{}, nil
	}
}

// mirrorValues injects for hedera-mirror:
//   - hedera.mirror.importer.consensusNodeEndpoints
//   - hedera.mirror.importer.network
func mirrorValues(capsules []operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) map[string]interface{} {
	return map[string]interface{}{
		"hedera": map[string]interface{}{
			"mirror": map[string]interface{}{
				"importer": map[string]interface{}{
					"consensusNodeEndpoints": consensusEndpoints(capsules),
					"network":                orbit.Name,
				},
			},
		},
	}
}

// relayValues injects for hedera-json-rpc-relay and hedera-json-rpc-relay-websocket:
//   - config.HEDERA_NETWORK   JSON map {"host:port": "accountId"} from ConsensusCapsules
//   - config.MIRROR_NODE_URL  http://mirror.<namespace>.svc.cluster.local
//   - config.CHAIN_ID         from Orbit networkSettings["ledger.id"]
func relayValues(capsules []operatorv1alpha1.ConsensusCapsule, namespace, ledgerID string) map[string]interface{} {
	config := map[string]interface{}{
		"HEDERA_NETWORK":  hederaNetworkJSON(capsules),
		"MIRROR_NODE_URL": fmt.Sprintf("http://mirror.%s.svc.cluster.local", namespace),
	}
	if ledgerID != "" {
		config["CHAIN_ID"] = ledgerID
	}
	return map[string]interface{}{"config": config}
}

// explorerValues injects for hiero-explorer-chart:
//   - config  (JSON array with a single network entry)
func explorerValues(orbit *operatorv1alpha1.Orbit, ledgerID string) map[string]interface{} {
	entry := map[string]interface{}{
		"name":        orbit.Name,
		"displayName": orbit.Name,
		"url":         "/",
	}
	if ledgerID != "" {
		entry["ledgerID"] = ledgerID
	}
	configBytes, _ := json.Marshal([]interface{}{entry})
	return map[string]interface{}{"config": string(configBytes)}
}

// blockNodeValues injects for block-node-server:
//   - blockNode.config.BLOCK_NODE_ENDPOINTS
//   - blockNode.config.CONSENSUS_NODE_COUNT
func blockNodeValues(capsules []operatorv1alpha1.ConsensusCapsule) map[string]interface{} {
	return map[string]interface{}{
		"blockNode": map[string]interface{}{
			"config": map[string]interface{}{
				"BLOCK_NODE_ENDPOINTS": strings.Join(consensusEndpoints(capsules), ","),
				"CONSENSUS_NODE_COUNT": strconv.Itoa(len(capsules)),
			},
		},
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// consensusEndpoints returns "host:port" strings from ConsensusCapsule address book entries.
// Capsules without a populated entry are skipped.
func consensusEndpoints(capsules []operatorv1alpha1.ConsensusCapsule) []string {
	var endpoints []string
	for _, c := range capsules {
		entry := c.Status.AddressBookEntry
		if entry == nil || entry.InternalHostname == "" {
			continue
		}
		port := entry.InternalPort
		if port == 0 {
			port = 50211
		}
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", entry.InternalHostname, port))
	}
	return endpoints
}

// hederaNetworkJSON builds the HEDERA_NETWORK JSON string expected by the relay chart:
// {"host:port": "accountId", ...} — one entry per ConsensusCapsule with a populated address book entry.
func hederaNetworkJSON(capsules []operatorv1alpha1.ConsensusCapsule) string {
	network := map[string]string{}
	for _, c := range capsules {
		entry := c.Status.AddressBookEntry
		if entry == nil || entry.InternalHostname == "" {
			continue
		}
		port := entry.InternalPort
		if port == 0 {
			port = 50211
		}
		network[fmt.Sprintf("%s:%d", entry.InternalHostname, port)] = c.Spec.AccountId
	}
	b, _ := json.Marshal(network)
	return string(b)
}

func listConsensusCapsules(ctx context.Context, c client.Client, namespace, orbitName string) ([]operatorv1alpha1.ConsensusCapsule, error) {
	list := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("listing consensus capsules: %w", err)
	}
	var matched []operatorv1alpha1.ConsensusCapsule
	for _, cc := range list.Items {
		if cc.Spec.Orbit == orbitName {
			matched = append(matched, cc)
		}
	}
	return matched, nil
}

func loadValuesReference(ctx context.Context, c client.Client, namespace string, ref operatorv1alpha1.HelmValuesReference) (map[string]interface{}, error) {
	key := ref.ValuesKey
	if key == "" {
		key = defaultValuesKey
	}

	var raw []byte

	switch ref.Kind {
	case "ConfigMap":
		cm := &corev1.ConfigMap{}
		err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, cm)
		if apierrors.IsNotFound(err) && ref.Optional {
			return map[string]interface{}{}, nil
		}
		if err != nil {
			return nil, err
		}
		raw = []byte(cm.Data[key])
	case "Secret":
		secret := &corev1.Secret{}
		err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret)
		if apierrors.IsNotFound(err) && ref.Optional {
			return map[string]interface{}{}, nil
		}
		if err != nil {
			return nil, err
		}
		raw = secret.Data[key]
	default:
		return nil, fmt.Errorf("unsupported ValuesReference kind %q; must be ConfigMap or Secret", ref.Kind)
	}

	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	jsonBytes, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("converting values to JSON: %w", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		return nil, fmt.Errorf("unmarshalling values: %w", err)
	}
	return out, nil
}

// mergeMaps recursively merges src into dst. Keys in src override dst;
// nested maps are merged rather than replaced.
func mergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}
		srcMap, srcIsMap := srcVal.(map[string]interface{})
		dstMap, dstIsMap := dstVal.(map[string]interface{})
		if srcIsMap && dstIsMap {
			dst[k] = mergeMaps(dstMap, srcMap)
		} else {
			dst[k] = srcVal
		}
	}
	return dst
}
