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
// For each Execute CR in Succeeded or Failed phase it:
//   - Deletes all persisted disk CRs for that operationId (all steps).
//   - Removes every marker file from --marker-path.
//   - If the phase is Succeeded, also removes the entire marker-path directory
//     (upgrade artifacts). This is deferred past PendingNodeUpgrade per the
//     ordering constraint: the daemon reads manifests/config from there during
//     the execute phase, so the directory must remain until the upgrade fully
//     completes.
//
// Errors during cleanup are logged and tolerated — a failed cleanup does not
// prevent the UC from entering the poll loop and resuming normal operation.
func (f *Factory) cleanupCompletedArtifacts(ctx context.Context) error {
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
		if operationID == "" {
			continue
		}

		f.cleanupForOperation(operationID, phase)
	}
	return nil
}

// cleanupForOperation deletes persisted disk CRs and marker files for a
// completed operationId, and removes the marker directory when phase is
// Succeeded (safe after the daemon has finished reading upgrade artifacts).
func (f *Factory) cleanupForOperation(operationID string, phase operatorv1alpha1.ExecutePhase) {
	// Delete all persisted CRs for this operationId.
	for step := range validSteps {
		if err := f.store.Delete(operationID, step); err != nil {
			logx.As().Warn().Err(err).
				Str("reason", ReasonArtifactCleanupFailed.String()).
				Str("operation_id", operationID).Str("step", step).
				Msg("failed to delete persisted CR during artifact cleanup")
		}
	}

	// Remove marker files.
	for _, marker := range AllMarkers {
		p := filepath.Join(f.cfg.MarkerPath, marker)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			logx.As().Warn().Err(err).
				Str("reason", ReasonArtifactCleanupFailed.String()).
				Str("operation_id", operationID).Str("marker", marker).
				Msg("failed to remove marker file during artifact cleanup")
		}
	}

	// Remove the upgrade artifacts directory only when the Execute CR has
	// reached Succeeded — the daemon reads manifests/config from there until at
	// least PendingNodeUpgrade, so we must not delete earlier.
	if phase == operatorv1alpha1.ExecutePhaseSucceeded {
		if err := os.RemoveAll(f.cfg.MarkerPath); err != nil {
			logx.As().Warn().Err(err).
				Str("reason", ReasonArtifactCleanupFailed.String()).
				Str("operation_id", operationID).
				Str("marker_path", f.cfg.MarkerPath).
				Msg("failed to remove upgrade artifacts directory during cleanup")
		} else {
			logx.As().Info().
				Str("reason", ReasonArtifactCleanup.String()).
				Str("operation_id", operationID).
				Str("marker_path", f.cfg.MarkerPath).
				Msg("removed upgrade artifacts directory after Succeeded Execute CR")
		}
	} else {
		logx.As().Info().
			Str("reason", ReasonArtifactCleanup.String()).
			Str("operation_id", operationID).
			Str("phase", string(phase)).
			Msg("cleaned up marker files and persisted CRs for completed upgrade (marker dir retained for diagnostics)")
	}
}

