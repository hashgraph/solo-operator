// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
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
			return nil, errx.WithHints(
				errx.WithReason(errValuesBuild.Wrap(err, "unmarshalling inline values"), ReasonValuesBuild),
				"Fix the JSON/YAML in spec.values so it parses",
			)
		}
		result = mergeMaps(result, inline)
	}

	return result, nil
}

// ComputeHash returns a SHA-256 of chart spec + values, used in status.ValuesHash to skip no-op upgrades.
func ComputeHash(chart operatorv1alpha1.HelmChartSpec, values map[string]interface{}) (string, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return "", errx.WithReason(errValuesBuild.Wrap(err, "marshalling values for hash"), ReasonValuesBuild)
	}
	h := sha256.New()
	if _, err = fmt.Fprintf(h, "%s\x00%s\x00%s\x00", chart.Repo, chart.Chart, chart.Version); err != nil {
		return "", errx.WithReason(errValuesBuild.Wrap(err, "writing hash"), ReasonValuesBuild)
	}
	h.Write(valuesJSON)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ─── Per-component injection ──────────────────────────────────────────────────

func injectComponentValues(ctx context.Context, c client.Client, namespace string, spec *operatorv1alpha1.HelmCapsuleSpec) (map[string]interface{}, error) {
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, types.NamespacedName{Name: spec.OrbitRef.Name}, orbit); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errx.WithHints(
				errx.WithReason(errValuesBuild.New("orbit %q not found — check spec.orbitRef.name", spec.OrbitRef.Name), ReasonOrbitNotFound),
				"Create the Orbit named in spec.orbitRef.name",
			)
		}
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting orbit %s", spec.OrbitRef.Name), ReasonGetFailed),
			"Confirm the ServiceAccount has get permission on orbits.operator.solo.hedera.com",
		)
	}

	// A consensus-less Orbit (e.g. one that only backs a block-node HelmCapsule) has no
	// consensus block; leave ledgerID/chainID empty for it. The block-node component
	// injects neither, and the mirror/relay/explorer components only run for Orbits that
	// do declare consensus.
	var ledgerID, chainID string
	if orbit.Spec.Consensus != nil {
		ledgerID = orbit.Spec.Consensus.Genesis.AddressBook.LedgerId
		chainID = orbit.Spec.Consensus.Genesis.AddressBook.ChainId
	}

	switch *spec.Component {
	case operatorv1alpha1.HelmComponentMirror:
		capsules, err := listConsensusCapsules(ctx, c, namespace, spec.OrbitRef.Name)
		if err != nil {
			return nil, err
		}
		return mirrorValues(ctx, c, namespace, capsules, orbit)
	case operatorv1alpha1.HelmComponentRelay, operatorv1alpha1.HelmComponentRelayWebSocket:
		capsules, err := listConsensusCapsules(ctx, c, namespace, spec.OrbitRef.Name)
		if err != nil {
			return nil, err
		}
		return relayValues(ctx, capsules, namespace, chainID)
	case operatorv1alpha1.HelmComponentExplorer:
		return explorerValues(orbit, ledgerID)
	case operatorv1alpha1.HelmComponentBlockNode:
		return blockNodeValues(), nil
	default:
		return map[string]interface{}{}, nil
	}
}

func mirrorValues(ctx context.Context, c client.Client, namespace string, capsules []operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) (map[string]interface{}, error) {
	vals := map[string]interface{}{
		"hedera": map[string]interface{}{
			"mirror": map[string]interface{}{
				"importer": map[string]interface{}{
					"consensusNodeEndpoints": consensusEndpoints(capsules),
					"network":                orbit.Name,
				},
			},
		},
	}

	// addressBook is a top-level importer sub-chart value, not an application config property.
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "addressbook-bin"}, cm); err == nil {
		if bin := cm.BinaryData["addressbook.bin"]; len(bin) > 0 {
			vals["importer"] = map[string]interface{}{
				"addressBook": base64.StdEncoding.EncodeToString(bin),
			}
		}
	} else if !apierrors.IsNotFound(err) {
		common.Err(ctx, err, ReasonAddressBookReadFailed).Msg("Failed to read addressbook-bin ConfigMap")
	}

	return vals, nil
}

func relayValues(ctx context.Context, capsules []operatorv1alpha1.ConsensusCapsule, namespace, chainID string) (map[string]interface{}, error) {
	network, err := hederaNetworkJSON(ctx, capsules)
	if err != nil {
		return nil, err
	}
	config := map[string]interface{}{
		"HEDERA_NETWORK":  network,
		"MIRROR_NODE_URL": fmt.Sprintf("http://mirror.%s.svc.cluster.local", namespace),
	}
	// The JSON-RPC relay's CHAIN_ID is the EVM chain id, not the ledger id.
	if chainID != "" {
		config["CHAIN_ID"] = chainID
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
		return nil, errx.WithReason(errValuesBuild.Wrap(err, "marshalling explorer config"), ReasonValuesBuild)
	}
	return map[string]interface{}{"config": string(configBytes)}, nil
}

// blockNodeValues injects nothing; topology comes from spec.values.
func blockNodeValues() map[string]interface{} {
	return map[string]interface{}{}
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
			common.Info(ctx, ReasonCapsuleSkipped).Str("capsule", c.Name).Msg("Skipping ConsensusCapsule from HEDERA_NETWORK: spec.accountId is empty")
			continue
		}
		network[fmt.Sprintf("%s:%d", entry.InternalHostname, port)] = c.Spec.AccountId
	}
	b, err := json.Marshal(network)
	if err != nil {
		return "", errx.WithReason(errValuesBuild.Wrap(err, "marshalling HEDERA_NETWORK"), ReasonValuesBuild)
	}
	return string(b), nil
}

func listConsensusCapsules(ctx context.Context, c client.Client, namespace, orbitName string) ([]operatorv1alpha1.ConsensusCapsule, error) {
	list := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing consensus capsules"), ReasonListFailed),
			"Confirm the ServiceAccount has list permission on consensuscapsules.operator.solo.hedera.com",
		)
	}
	var matched []operatorv1alpha1.ConsensusCapsule
	for _, cc := range list.Items {
		if cc.Spec.Orbit == orbitName {
			matched = append(matched, cc)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Spec.NodeId < matched[j].Spec.NodeId
	})
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
			return nil, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "get ConfigMap %s", ref.Name), ReasonGetFailed),
				"Confirm the referenced ConfigMap exists and the operator has get permission on configmaps",
			)
		}
		val, ok := cm.Data[key]
		if !ok && !ref.Optional {
			return nil, errx.WithHints(
				errx.WithReason(errValuesBuild.New("key %q not found in ConfigMap %s", key, ref.Name), ReasonKeyNotFound),
				"Add the referenced key to the ConfigMap, or set the ref's valuesKey/optional",
			)
		}
		raw = []byte(val)
	case "Secret":
		secret := &corev1.Secret{}
		err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret)
		if apierrors.IsNotFound(err) && ref.Optional {
			return map[string]interface{}{}, nil
		}
		if err != nil {
			return nil, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "get Secret %s", ref.Name), ReasonGetFailed),
				"Confirm the referenced Secret exists and the operator has get permission on secrets",
			)
		}
		val, ok := secret.Data[key]
		if !ok && !ref.Optional {
			return nil, errx.WithHints(
				errx.WithReason(errValuesBuild.New("key %q not found in Secret %s", key, ref.Name), ReasonKeyNotFound),
				"Add the referenced key to the Secret, or set the ref's valuesKey/optional",
			)
		}
		raw = val
	default:
		return nil, errx.WithHints(
			errx.WithReason(errValuesBuild.New("unsupported ValuesReference kind %q; must be ConfigMap or Secret", ref.Kind), ReasonUnsupportedKind),
			"Set the valuesFrom ref kind to ConfigMap or Secret",
		)
	}

	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	var out map[string]interface{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		if ref.Kind == "Secret" {
			// The parser cause is omitted: it can quote Secret content.
			return nil, errx.WithHints(
				errx.WithReason(errValuesBuild.New("values in Secret %s key %q are not a valid YAML mapping", ref.Name, key), ReasonValuesBuild),
				"Fix the YAML in the referenced Secret so it parses; parser detail is withheld because it can echo Secret content",
			)
		}
		return nil, errx.WithHints(
			errx.WithReason(errValuesBuild.Wrap(err, "parsing values"), ReasonValuesBuild),
			"Fix the YAML in the referenced ConfigMap/Secret so it parses",
		)
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
