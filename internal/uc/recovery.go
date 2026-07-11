// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"

	"github.com/automa-saga/logx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// recoverPersistedCRs re-delivers any CR that was persisted to disk before the
// K8s API Create but whose process crashed before the send completed. This is
// the restart half of the persist-before-send crash-safety guarantee (#992).
//
// For each file found in --cr-persist-path:
//   - If the CR already exists in the cluster (source of truth) the disk copy is
//     stale — delete it and record the step in the dedup cache.
//   - If the CR is absent from the cluster, re-send it. On success delete the
//     disk copy. On transient failure leave the file for the next restart and
//     continue with the remaining files (one stuck CR must not block the others).
func (f *Factory) recoverPersistedCRs(ctx context.Context) error {
	crs, err := f.store.List()
	if err != nil {
		// Can't enumerate persisted CRs — log and continue; the per-action
		// dedup query remains authoritative so skipping the re-send list is safe.
		logx.As().Warn().Err(err).Str("reason", ReasonCRPersistListFailed.String()).
			Msg("could not list persisted CRs for restart recovery — skipping re-send pass")
		return nil
	}
	if len(crs) == 0 {
		return nil
	}

	for _, pcr := range crs {
		gvr, ok := stepToGVR[pcr.Step]
		if !ok {
			// Unknown step — CRStore.List already validates steps; this branch is
			// defensive against future store changes.
			logx.As().Warn().
				Str("reason", ReasonCRResentFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("persisted CR has unknown step — skipping")
			continue
		}

		// Check cluster (source of truth) before re-sending.
		exists, err := f.alreadyCreated(ctx, gvr, pcr.OperationID, pcr.Step)
		if err != nil {
			// Transient K8s API error — leave the file and move on; the cache was
			// already invalidated by alreadyCreated.
			logx.As().Warn().Err(err).
				Str("reason", ReasonCRResentFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("cluster query failed during restart recovery — leaving persisted CR for next restart")
			continue
		}
		if exists {
			// CR delivered before crash — delete the now-redundant disk copy.
			if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
				logx.As().Warn().Err(delErr).
					Str("reason", ReasonCRPersistCleanupFailed.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("failed to delete persisted CR that already exists in cluster")
			} else {
				logx.As().Debug().
					Str("reason", ReasonCRExists.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("persisted CR already in cluster — deleted stale disk copy")
			}
			continue
		}

		// CR is absent from the cluster — re-send it.
		resource := f.client.Resource(gvr).Namespace(f.cfg.Namespace)
		createCtx, cancel := context.WithTimeout(ctx, createTimeout)
		_, createErr := resource.Create(createCtx, pcr.Object, metav1.CreateOptions{})
		cancel()

		if createErr != nil {
			if k8serrors.IsAlreadyExists(createErr) {
				// Race: created between our query and our Create. Treat as success.
				f.markCreated(pcr.OperationID, pcr.Step)
				if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
					logx.As().Warn().Err(delErr).
						Str("reason", ReasonCRPersistCleanupFailed.String()).
						Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
						Msg("failed to delete persisted CR after AlreadyExists during recovery")
				}
				logx.As().Debug().
					Str("reason", ReasonCRExists.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("persisted CR created in a race — treated as success")
				continue
			}
			// Transient failure — leave on disk for the next restart.
			logx.As().Error().
				Err(errx.WithReason(errK8sAPI.Wrap(createErr, "re-send persisted CR %s/%s", pcr.OperationID, pcr.Step), ReasonCRResentFailed)).
				Str("reason", ReasonCRResentFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("failed to re-send persisted CR — leaving on disk for next restart")
			continue
		}

		f.markCreated(pcr.OperationID, pcr.Step)
		if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
			logx.As().Warn().Err(delErr).
				Str("reason", ReasonCRPersistCleanupFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("failed to delete persisted CR after successful re-send")
		}
		logx.As().Info().
			Str("reason", ReasonCRResent.String()).
			Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
			Msg("re-sent persisted CR that was not yet in cluster")
	}
	return nil
}

// cleanupCompletedArtifacts removes stale upgrade artifacts left on disk when
// the UC restarts after an upgrade has already reached a terminal phase.
//
// Execute CRs are never deleted (audit trail), so the cleanup pass must
// only act on artifacts that demonstrably belong to a completed operation —
// not every historical Execute CR in the namespace. Two sources of disk
// presence are cross-referenced against the set of terminal Execute CRs:
//
//  1. Persisted CRs (--cr-persist-path): keyed by operationId, so any persisted
//     file whose operationId maps to a terminal Execute CR is safe to delete.
//
//  2. Marker files (--marker-path): the current operationId is derived from the
//     execute_immediate.mf + manifest on disk. Marker files and the marker-path
//     directory are only removed when that derived operationId matches a terminal
//     Execute CR — guaranteeing we never wipe markers that belong to a newer,
//     in-flight upgrade whose Execute CR is not yet terminal.
//
// Errors during cleanup are logged and tolerated — a failed cleanup does not
// prevent the UC from entering the poll loop.
func (f *Factory) cleanupCompletedArtifacts(ctx context.Context) error {
	// Query all Execute CRs and build the set of terminal operationIds.
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	list, err := f.client.Resource(networkUpgradeExecuteGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	cancel()
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonArtifactCleanupFailed.String()).
			Msg("could not list Execute CRs for artifact cleanup — skipping cleanup pass")
		return nil
	}

	terminal := make(map[string]operatorv1alpha1.ExecutePhase, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		phaseStr, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		phase := operatorv1alpha1.ExecutePhase(phaseStr)
		if phase != operatorv1alpha1.ExecutePhaseSucceeded && phase != operatorv1alpha1.ExecutePhaseFailed {
			continue
		}
		operationID, _, _ := unstructured.NestedString(item.Object, "spec", "operationId")
		if operationID == "" {
			operationID = item.GetLabels()[operatorv1alpha1.LabelOperationID]
		}
		if operationID != "" {
			terminal[operationID] = phase
		}
	}
	if len(terminal) == 0 {
		return nil
	}

	// Pass 1: delete persisted disk CRs whose operationId is terminal.
	// Persisted files are named "<operationId>.<step>.yaml" so they are already
	// tied to a specific operationId — safe to remove without consulting markers.
	if persisted, listErr := f.store.List(); listErr == nil {
		for _, pcr := range persisted {
			if _, ok := terminal[pcr.OperationID]; ok {
				if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
					logx.As().Warn().Err(delErr).
						Str("reason", ReasonArtifactCleanupFailed.String()).
						Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
						Msg("failed to delete persisted CR during artifact cleanup")
				}
			}
		}
	}

	// Pass 2: clean marker files and (for Succeeded) the marker directory, but
	// ONLY when the on-disk execute_immediate.mf resolves to a terminal
	// operationId. This prevents wiping markers that belong to a newer upgrade
	// whose Execute CR has not yet reached a terminal phase.
	diskOpID, _, _, found, err := deriveOperationIDPrimary(f.cfg.UpgradePath)
	if err != nil || !found {
		// No current execute_immediate.mf or it can't be parsed — skip marker
		// cleanup. Persisted CRs were already handled above.
		return nil
	}
	phase, isTerminal := terminal[diskOpID]
	if !isTerminal {
		return nil
	}

	for _, marker := range AllMarkers {
		p := filepath.Join(f.cfg.UpgradePath, marker)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			logx.As().Warn().Err(err).
				Str("reason", ReasonArtifactCleanupFailed.String()).
				Str("operation_id", diskOpID).Str("marker", marker).
				Msg("failed to remove marker file during artifact cleanup")
		}
	}

	// Remove the upgrade artifacts directory only for Succeeded — the daemon
	// reads manifests/config from there until at least PendingNodeUpgrade, so
	// the directory must not be removed while the upgrade is still in progress.
	if phase == operatorv1alpha1.ExecutePhaseSucceeded {
		if err := os.RemoveAll(f.cfg.UpgradePath); err != nil {
			logx.As().Warn().Err(err).
				Str("reason", ReasonArtifactCleanupFailed.String()).
				Str("operation_id", diskOpID).
				Str("marker_path", f.cfg.UpgradePath).
				Msg("failed to remove upgrade artifacts directory during cleanup")
		} else {
			logx.As().Info().
				Str("reason", ReasonArtifactCleanup.String()).
				Str("operation_id", diskOpID).
				Str("marker_path", f.cfg.UpgradePath).
				Msg("removed upgrade artifacts directory after Succeeded Execute CR")
		}
	} else {
		logx.As().Info().
			Str("reason", ReasonArtifactCleanup.String()).
			Str("operation_id", diskOpID).Str("phase", string(phase)).
			Msg("cleaned up marker files and persisted CRs for completed upgrade (marker dir retained for diagnostics)")
	}
	return nil
}
