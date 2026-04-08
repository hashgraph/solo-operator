// SPDX-License-Identifier: Apache-2.0

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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const defaultValuesKey = "values.yaml"

// MergeValues assembles the final values by layering component injection, ValuesFrom refs, and inline values.
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

// ComputeHash returns a SHA-256 of chart spec + values, used in status.ValuesHash to skip no-op upgrades.
func ComputeHash(chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}) (string, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshalling values for hash: %w", err)
	}
	h := sha256.New()
	if _, err = fmt.Fprintf(h, "%s\x00%s\x00%s\x00", chart.Repo, chart.Chart, chart.Version); err != nil {
		return "", fmt.Errorf("writing hash: %w", err)
	}
	h.Write(valuesJSON)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ─── Per-component injection ──────────────────────────────────────────────────

func injectComponentValues(ctx context.Context, c client.Client, namespace string, spec *operatorv1alpha1.HelmCapsuleSpec) (map[string]interface{}, error) {
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, types.NamespacedName{Name: spec.OrbitRef.Name}, orbit); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("orbit %q not found — check spec.orbitRef.name", spec.OrbitRef.Name)
		}
		return nil, fmt.Errorf("getting orbit %s: %w", spec.OrbitRef.Name, err)
	}

	var ledgerID string
	if orbit.Spec.Consensus.NetworkSettings != nil {
		ledgerID = orbit.Spec.Consensus.NetworkSettings["ledger.id"]
	}

	capsules, err := listConsensusCapsules(ctx, c, namespace, spec.OrbitRef.Name)
	if err != nil {
		return nil, err
	}

	switch *spec.Component {
	case operatorv1alpha1.HelmComponentMirror:
		return mirrorValues(capsules, orbit), nil
	case operatorv1alpha1.HelmComponentRelay, operatorv1alpha1.HelmComponentRelayWebSocket:
		return relayValues(ctx, capsules, namespace, ledgerID)
	case operatorv1alpha1.HelmComponentExplorer:
		return explorerValues(orbit, ledgerID)
	case operatorv1alpha1.HelmComponentBlockNode:
		return blockNodeValues(capsules), nil
	default:
		return map[string]interface{}{}, nil
	}
}

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

func relayValues(ctx context.Context, capsules []operatorv1alpha1.ConsensusCapsule, namespace, ledgerID string) (map[string]interface{}, error) {
	network, err := hederaNetworkJSON(ctx, capsules)
	if err != nil {
		return nil, err
	}
	config := map[string]interface{}{
		"HEDERA_NETWORK":  network,
		"MIRROR_NODE_URL": fmt.Sprintf("http://mirror.%s.svc.cluster.local", namespace),
	}
	if ledgerID != "" {
		config["CHAIN_ID"] = ledgerID
	}
	return map[string]interface{}{"config": config}, nil
}

func explorerValues(orbit *operatorv1alpha1.Orbit, ledgerID string) (map[string]interface{}, error) {
	entry := map[string]interface{}{
		"name":        orbit.Name,
		"displayName": orbit.Name,
		"url":         "/",
	}
	if ledgerID != "" {
		entry["ledgerID"] = ledgerID
	}
	configBytes, err := json.Marshal([]interface{}{entry})
	if err != nil {
		return nil, fmt.Errorf("marshalling explorer config: %w", err)
	}
	return map[string]interface{}{"config": string(configBytes)}, nil
}

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

// consensusEndpoints builds "host:port" strings from capsules that have an address book entry.
func consensusEndpoints(capsules []operatorv1alpha1.ConsensusCapsule) []string {
	endpoints := make([]string, 0, len(capsules))
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

// hederaNetworkJSON formats the {"host:port": "accountId"} map expected by the relay chart.
// Capsules missing an address book entry or AccountId are skipped with a warning.
func hederaNetworkJSON(ctx context.Context, capsules []operatorv1alpha1.ConsensusCapsule) (string, error) {
	logger := log.FromContext(ctx)
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
		if c.Spec.AccountId == "" {
			logger.Info("Skipping ConsensusCapsule from HEDERA_NETWORK: spec.accountId is empty",
				"capsule", c.Name)
			continue
		}
		network[fmt.Sprintf("%s:%d", entry.InternalHostname, port)] = c.Spec.AccountId
	}
	b, err := json.Marshal(network)
	if err != nil {
		return "", fmt.Errorf("marshalling HEDERA_NETWORK: %w", err)
	}
	return string(b), nil
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
		val, ok := cm.Data[key]
		if !ok && !ref.Optional {
			return nil, fmt.Errorf("key %q not found in ConfigMap %s", key, ref.Name)
		}
		raw = []byte(val)
	case "Secret":
		secret := &corev1.Secret{}
		err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret)
		if apierrors.IsNotFound(err) && ref.Optional {
			return map[string]interface{}{}, nil
		}
		if err != nil {
			return nil, err
		}
		val, ok := secret.Data[key]
		if !ok && !ref.Optional {
			return nil, fmt.Errorf("key %q not found in Secret %s", key, ref.Name)
		}
		raw = val
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
