// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/sanity"
)

// skipTerminal defines which NetworkUpgradePrepare status.phase values
// disqualify a CR from being the resolved upgrade identity in resolveOperationID.
// Succeeded AND Failed are both excluded: a Succeeded Prepare belongs to a
// completed upgrade and must not shadow a new in-progress one.
var skipTerminal = map[string]struct{}{
	string(operatorv1alpha1.PhaseSucceeded): {},
	string(operatorv1alpha1.PhaseFailed):    {},
}

// resolveOperationID returns the operationId of the in-flight upgrade. The UC
// mints operationId exactly once, at prepare (HIP-1496); every later phase —
// Freeze, Execute, FreezeAbort — inherits it and never regenerates. This reads
// it back from the most-recent non-terminal NetworkUpgradePrepare CR, with the
// cluster as the source of truth and a fallback to a Prepare persisted on disk
// but not yet delivered (the crash window).
//
// It returns ("", nil) when no Prepare exists yet in either source: the caller
// treats that as "no identity to act on this tick". The marker persists, so a
// later tick retries once the Prepare CR appears. A query/IO failure is returned
// as an error (transient — retry next tick) rather than mistaken for "absent".
// resolveOperationID resolves the operationId of the in-flight upgrade by
// finding the most-recent NetworkUpgradePrepare CR whose phase is not in
// skipPhases, cluster-first with a disk fallback.
func (f *Factory) resolveOperationID(ctx context.Context) (string, error) {
	return f.resolveOperationIDWith(ctx, skipTerminal)
}

func (f *Factory) resolveOperationIDWith(ctx context.Context, skipPhases map[string]struct{}) (string, error) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradePrepare"), ReasonPrepareListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradeprepares."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	var (
		best   string
		bestTS time.Time
		found  bool
	)
	for i := range list.Items {
		item := &list.Items[i]
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if _, skip := skipPhases[phase]; skip {
			continue
		}
		// spec.operationId is the source of truth — it is a hard-required field on
		// every NetworkUpgrade* CR. The label is a human-facing convenience and is
		// only a fallback for a CR that somehow lacks the spec field.
		opID, _, _ := unstructured.NestedString(item.Object, "spec", "operationId")
		if opID == "" {
			opID = item.GetLabels()[operatorv1alpha1.LabelOperationID]
		}
		if opID == "" {
			continue
		}
		if ts := item.GetCreationTimestamp().Time; !found || ts.After(bestTS) {
			best, bestTS, found = opID, ts, true
		}
	}
	if found {
		return best, nil
	}

	// Disk fallback: a Prepare persisted before its Create was acknowledged.
	// Choose the most recent by the prepare timestamp embedded in the operationId
	// — the same notion of recency the cluster path uses (creationTimestamp), not
	// a lexical compare. Lexical order is wrong here because the version sorts
	// ahead of the timestamp (e.g. "upgrade-0.9.0-<old>" > "upgrade-0.75.0-<new>").
	persisted, err := f.store.List()
	if err != nil {
		// Already an errx-decorated persist (disk) error.
		return "", err
	}
	best, bestTS, found = "", time.Time{}, false
	for _, p := range persisted {
		if p.Step != operatorv1alpha1.StepPrepare {
			continue
		}
		ts, ok := operationIDTimestamp(p.OperationID)
		if !ok {
			// A persisted Prepare the UC wrote always carries a parseable timestamp
			// (mintOperationID guarantees it); skip anything that does not rather
			// than risk emitting a malformed identity.
			continue
		}
		if !found || ts.After(bestTS) {
			best, bestTS, found = p.OperationID, ts, true
		}
	}
	return best, nil
}

// deriveOperationIDPrimary attempts to re-derive the operationId from the
// co-present execute_immediate.mf marker and consensus-node-components.yaml.
// Returns (operationID, executeMtime, manifest, true, nil) on success.
// Returns ("", zero, nil, false, nil) when execute_immediate.mf is absent —
// caller should fall back to version-based resolution.
// Returns ("", zero, nil, false, err) on IO or parse error — caller should retry.
func deriveOperationIDPrimary(upgradePath string) (operationID string, executeMtime time.Time, manifest *componentsManifest, found bool, err error) {
	preparePath := filepath.Join(upgradePath, MarkerExecuteImmediate)
	prepareInfo, err := os.Stat(preparePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", time.Time{}, nil, false, nil
		}
		return "", time.Time{}, nil, false,
			errx.WithReason(errPersist.Wrap(err, "stat %s", MarkerExecuteImmediate), ReasonMarkerStatFailed)
	}

	manifestPath := filepath.Join(upgradePath, manifestsSubdir, componentsManifestFile)
	manifest, err = parseComponentsManifest(manifestPath)
	if err != nil {
		return "", time.Time{}, nil, false, err
	}

	operationID, err = mintOperationID(manifest.Images.ConsensusNode.Version, prepareInfo.ModTime())
	if err != nil {
		return "", time.Time{}, nil, false,
			errx.WithReason(errManifestInvalid.Wrap(err, "mint operationId"), ReasonManifestInvalid)
	}

	// Cross-check: VERSION file must agree with the manifest version so the
	// operationId accurately represents the package on disk. A mismatch means
	// the CN wrote a package for a different version than the manifest declares —
	// treat as a transient inconsistency and retry next tick.
	diskVersion, _, versionFound, versionErr := parseVersionFile(upgradePath)
	if versionErr != nil {
		return "", time.Time{}, nil, false, versionErr
	}
	if versionFound && diskVersion != manifest.Images.ConsensusNode.Version {
		return "", time.Time{}, nil, false,
			errx.WithHints(
				errx.WithReason(
					errManifestInvalid.New("VERSION file reports %q but manifest declares %q — package write may be in progress",
						diskVersion, manifest.Images.ConsensusNode.Version),
					ReasonManifestInvalid),
				"Wait for the consensus node to finish writing the upgrade package",
			)
	}

	return operationID, prepareInfo.ModTime(), manifest, true, nil
}

// resolveByVersion finds the most-recent operationId whose version segment matches
// the given semver, among NetworkUpgradePrepare CRs whose phase is not in
// skipPhases. Checks the cluster first, then persisted-on-disk Prepare CRs as a
// fallback. Used when execute_immediate.mf is absent and identity must be inferred
// from the manifest version alone.
func (f *Factory) resolveByVersion(ctx context.Context, version string, skipPhases map[string]struct{}) (string, error) {
	prefix := "upgrade-" + version + "-"

	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradePrepare for version %s", version), ReasonPrepareListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradeprepares."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	var best string
	var bestTS time.Time
	var found bool
	for i := range list.Items {
		item := &list.Items[i]
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if _, skip := skipPhases[phase]; skip {
			continue
		}
		opID, _, _ := unstructured.NestedString(item.Object, "spec", "operationId")
		if opID == "" {
			opID = item.GetLabels()[operatorv1alpha1.LabelOperationID]
		}
		if !strings.HasPrefix(opID, prefix) {
			continue
		}
		if ts := item.GetCreationTimestamp().Time; !found || ts.After(bestTS) {
			best, bestTS, found = opID, ts, true
		}
	}
	if found {
		return best, nil
	}

	// Disk fallback: a Prepare persisted before its Create was acknowledged.
	persisted, err := f.store.List()
	if err != nil {
		return "", err
	}
	for _, p := range persisted {
		if p.Step != operatorv1alpha1.StepPrepare {
			continue
		}
		if !strings.HasPrefix(p.OperationID, prefix) {
			continue
		}
		ts, ok := operationIDTimestamp(p.OperationID)
		if !ok {
			continue
		}
		if !found || ts.After(bestTS) {
			best, bestTS, found = p.OperationID, ts, true
		}
	}
	return best, nil
}

const versionFile = "VERSION"

// parseVersionFile reads VERSION from upgradePath and returns the value of the
// VERSION= line (e.g. "0.74.0") and the file's modification time.
// Returns ("", zero, false, nil) when the file is absent.
func parseVersionFile(upgradePath string) (version string, mtime time.Time, found bool, err error) {
	path := filepath.Join(upgradePath, versionFile)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", time.Time{}, false, nil
		}
		return "", time.Time{}, false, errx.WithReason(errPersist.Wrap(err, "stat VERSION file"), ReasonUpgradePathUnreadable)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, false, errx.WithReason(errPersist.Wrap(err, "read VERSION file"), ReasonUpgradePathUnreadable)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "VERSION="); ok {
			return strings.TrimSpace(v), info.ModTime(), true, nil
		}
	}
	return "", time.Time{}, false, nil
}

// versionFromOperationID extracts the version segment from an operationId of
// the form "upgrade-<version>-<timestamp>". It strips the "upgrade-" prefix and
// the trailing "-<timestamp>" suffix. Returns ("", false) if the format is
// unrecognised.
func versionFromOperationID(operationID string) (string, bool) {
	// Must start with "upgrade-"
	rest, ok := strings.CutPrefix(operationID, "upgrade-")
	if !ok {
		return "", false
	}
	// Strip the trailing "-<timestamp>" — split on last "-"
	idx := strings.LastIndex(rest, "-")
	if idx <= 0 {
		return "", false
	}
	return rest[:idx], true
}

// operationIDTimestamp extracts and parses the prepare timestamp embedded in an
// operationId of the form "upgrade-<version>-<YYYYMMDDTHHmmssZ>". It splits on
// the LAST "-" so a version that itself contains a dash (e.g. a "0.75.0-rc1"
// pre-release) is handled correctly. Returns ok=false if the trailing segment is
// not a valid timestamp.
func operationIDTimestamp(operationID string) (time.Time, bool) {
	idx := strings.LastIndex(operationID, "-")
	if idx < 0 || idx == len(operationID)-1 {
		return time.Time{}, false
	}
	ts, err := time.Parse(sanity.TimestampLayout, operationID[idx+1:])
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}
