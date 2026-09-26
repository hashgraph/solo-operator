// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Msg("Skipping re-send pass: failed to list persisted CRs for restart recovery")
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
				Msg("Skipping persisted CR: unknown step")
			continue
		}

		// Check cluster (source of truth) before re-sending.
		exists, _, err := f.alreadyCreated(ctx, gvr, pcr.OperationID, pcr.Step)
		if err != nil {
			// Transient K8s API error — leave the file and move on; the cache was
			// already invalidated by alreadyCreated.
			logx.As().Warn().Err(err).
				Str("reason", ReasonCRResentFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("Leaving persisted CR for next restart: cluster query failed during recovery")
			continue
		}
		if exists {
			// CR delivered before crash — delete the now-redundant disk copy.
			if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
				logx.As().Warn().Err(delErr).
					Str("reason", ReasonCRPersistCleanupFailed.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("Failed to delete persisted CR that already exists in cluster")
			} else {
				logx.As().Debug().
					Str("reason", ReasonCRExists.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("Deleted stale disk copy: persisted CR already in cluster")
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
				f.markCreated(pcr.OperationID, pcr.Step, "")
				if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
					logx.As().Warn().Err(delErr).
						Str("reason", ReasonCRPersistCleanupFailed.String()).
						Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
						Msg("Failed to delete persisted CR after AlreadyExists during recovery")
				}
				logx.As().Debug().
					Str("reason", ReasonCRExists.String()).
					Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
					Msg("Treated persisted CR as success (created in a race)")
				continue
			}
			// Transient failure — leave on disk for the next restart.
			logx.As().Error().
				Err(errx.WithReason(errK8sAPI.Wrap(createErr, "re-send persisted CR %s/%s", pcr.OperationID, pcr.Step), ReasonCRResentFailed)).
				Str("reason", ReasonCRResentFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("Failed to re-send persisted CR; leaving on disk for next restart")
			continue
		}

		f.markCreated(pcr.OperationID, pcr.Step, "")
		if delErr := f.store.Delete(pcr.OperationID, pcr.Step); delErr != nil {
			logx.As().Warn().Err(delErr).
				Str("reason", ReasonCRPersistCleanupFailed.String()).
				Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
				Msg("Failed to delete persisted CR after successful re-send")
		}
		logx.As().Info().
			Str("reason", ReasonCRResent.String()).
			Str("operation_id", pcr.OperationID).Str("step", pcr.Step).
			Msg("Re-sent persisted CR that was not yet in cluster")
	}
	return nil
}
