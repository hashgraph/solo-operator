// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	"github.com/joomcode/errorx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

// Factory constructs NetworkUpgrade* CRs from deployment-package manifests and
// creates them in the cluster, following the persist-before-send pattern. It owns
// the in-memory operation-id cache used to skip already-processed markers.
//
// Skeleton: handlers are stubs to be filled in per story. Each handler must be
// idempotent — safe to call on every poll tick while a marker file remains on
// disk (markers are removed only at final cleanup).
type Factory struct {
	cfg    Config
	client dynamic.Interface

	// events writes the per-operation JSONL audit trail (#1127). Nil-safe: a nil
	// sink no-ops, so tests and events-disabled configs need no special casing.
	events *eventSink

	// store implements persist-before-send (#992): each handler serializes its CR
	// under cfg.CRPersistPath before Create and deletes the file on success.
	store *CRStore

	// mu guards the operation-id cache (#997) and the rejected map. The cache is a
	// performance hint only; correctness rests on the K8s API existence check
	// before each create, backstopped by AlreadyExists on Create (first-detection-wins).
	mu    sync.Mutex
	cache map[string]map[string]string // operationId -> step -> phase (empty = unknown)
	// rejected records the last admission-rejection time per operationId+step so a
	// CR the VAP rejects (#1133) is retried only once per admissionRejectRetryCooldown
	// instead of on every poll tick. Retrying (rather than stopping) lets the UC
	// self-heal without a restart once a mis-configured VAP is fixed.
	rejected map[rejectKey]time.Time
}

// rejectKey identifies one operation's one step in the rejected backoff map.
type rejectKey struct {
	operationID string
	step        string
}

// admissionRejectRetryCooldown bounds how often a NetworkUpgrade CR rejected at
// admission (#1133) is re-attempted. A VAP is a cluster-scoped resource fixed
// independently of the UC (kubectl / helm upgrade of the operator), so the UC
// keeps retrying — on this cooldown — so a fixed VAP self-heals without a
// consensus-node restart, while a persistent marker cannot re-send (and
// re-reject) on every tick.
const admissionRejectRetryCooldown = 5 * time.Minute

// NewFactory constructs the CR factory.
func NewFactory(cfg Config, client dynamic.Interface) *Factory {
	return &Factory{
		cfg:      cfg,
		client:   client,
		store:    NewCRStore(cfg.CRPersistPath),
		cache:    make(map[string]map[string]string),
		rejected: make(map[rejectKey]time.Time),
	}
}

// RecoverOnStartup reconciles cluster state and persisted CRs on disk so the
// UC resumes in-flight upgrades without re-signalling.
//
// It runs two passes in sequence:
//
//  1. Seed the op-id dedup cache from all existing NetworkUpgrade* CRs in the
//     cluster, so steady-state ticks for already-handled markers are served from
//     cache. A seed failure is logged but not fatal.
//
//  2. Re-send any CR persisted to disk (--cr-persist-path) but not yet present
//     in the cluster. These are crash-window survivors: the prior process wrote
//     the file but crashed before the K8s API Create completed.
//
// Marker files and upgrade-path rotation (current→previous) are never touched
// here. Rotation is the exclusive responsibility of UpgradeOutcomeMonitor,
// which handles it on every terminal Execute or FreezeAbort CR observation —
// both on startup (list) and live (watch).
func (f *Factory) RecoverOnStartup(ctx context.Context) error {
	if err := f.seedCache(ctx); err != nil {
		common.LogErr(logx.As().Warn(), err, ReasonUCCacheSeedFailed).
			Msg("could not seed op-id dedup cache on startup; per-action queries remain authoritative")
	} else {
		logx.As().Info().Str("reason", ReasonUCCacheSeeded.String()).
			Msg("seeded op-id dedup cache from existing NetworkUpgrade CRs")
	}

	// Re-send crash-window survivors (persist-before-send, #992).
	if err := f.recoverPersistedCRs(ctx); err != nil {
		// recoverPersistedCRs is already best-effort and returns nil; this guard is
		// defensive against future refactors that promote individual errors.
		common.LogErr(logx.As().Warn(), err, ReasonUCStartupRecoveryError).
			Msg("startup CR re-send pass encountered an error — continuing")
	}

	return nil
}

// HandleExecuteImmediate parses consensus-node-components.yaml from the
// deployment package and creates a NetworkUpgradePrepare CR following the
// persist-before-send pattern (#992).
//
// The marker directory IS the extracted package root (the consensus node unzips
// the archive and writes the marker into the same dir), so the manifest lives at
// <marker-path>/manifests/consensus-node-components.yaml. The operationId is
// minted deterministically from the marker's creation time + package version
// (HIP-1496); recomputing it on every poll tick yields the same value, so a
// re-detected marker maps to the same CR name and the Create is a safe no-op via
// AlreadyExists ("first detection wins").
//
// The UC copies spec.images verbatim and does NOT do registry selection or
// layer-hash verification — that is the operator's PrepareReconciler.
func (f *Factory) HandleExecuteImmediate(ctx context.Context) error {
	operationID, _, manifest, found, err := deriveOperationIDPrimary(f.cfg.UpgradePath, f.cfg.CapsuleName())
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			// ManifestInvalid is terminal — re-reading the same bytes will not fix
			// it. Log and halt without creating a CR (no retry value).
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerExecuteImmediate).
				Msg("invalid consensus-node-components.yaml or operationId — halting prepare")
			return nil
		}
		// Transient IO error — retry next tick.
		return err
	}
	if !found {
		// Marker vanished between the poller's Stat and this dispatch — IO race;
		// retry next tick.
		return errx.WithReason(
			errPersist.New("execute_immediate.mf absent at dispatch time"),
			ReasonMarkerStatFailed)
	}

	cr, err := buildPrepareCR(f.cfg, operationID, manifest.Images)
	if err != nil {
		return err
	}
	return f.deliverCR(ctx, networkUpgradePrepareGVR, operationID, operatorv1alpha1.StepPrepare, kindNetworkUpgradePrepare, cr)
}

// deliverCR is the shared persist-before-send + dedup tail of every marker
// handler. It (1) skips when the CR already exists — cache-first, falling back
// to the authoritative cluster query (#997); (2) persists the CR to disk before
// the K8s API call so a crash in the send window leaves it recoverable (#992);
// (3) Creates it, treating AlreadyExists as success (first-detection-wins, the
// ultimate dedup guard); and (4) maintains the op-id cache and persisted file.
//
// It returns nil on success or a benign no-op, and an error only on a transient
// failure worth retrying on the next tick. On a transient create failure the
// persisted file is intentionally left on disk for the restart-recovery flow
// (#998) and the cache entry is invalidated so a stale hint cannot mask a
// missing CR. An admission-validation rejection (Invalid) is handled specially
// (#1133): it logs a loud ERROR, deletes the persisted file, and backs off for
// admissionRejectRetryCooldown before retrying — so the create does not repeat
// every tick, yet a fixed VAP still self-heals without a UC restart. Forbidden is
// treated as transient, not terminal, since it also covers RBAC gaps.
func (f *Factory) deliverCR(ctx context.Context, gvr schema.GroupVersionResource, operationID, step, kind string, cr *unstructured.Unstructured) error {
	if f.inRejectCooldown(operationID, step) {
		// A prior admission rejection (#1133) is still within its cooldown window.
		// Skip the whole persist/create cycle silently: the marker persists on disk,
		// so re-sending here would just re-reject every tick. The cooldown lapses on
		// its own, at which point we retry (so a fixed VAP recovers without a restart).
		return nil
	}

	exists, phase, err := f.alreadyCreated(ctx, gvr, operationID, step)
	if err != nil {
		// Already an errx-decorated K8s API error from the dedup query.
		return err
	}
	if exists {
		if phase == "Failed" {
			markerFile := stepToMarker[step]
			logx.As().Error().Str("reason", ReasonCRFailed.String()).Str("operation_id", operationID).
				Str("step", step).Str("cr_name", cr.GetName()).
				Msgf("%s CR has Failed — delete %s to clear", kind, markerFile)
		} else {
			logx.As().Debug().Str("reason", ReasonCRExists.String()).Str("operation_id", operationID).
				Str("step", step).Str("cr_name", cr.GetName()).Msg("CR already exists — skipping create")
		}
		return nil
	}

	// First tick that will actually create this operation's CR — record the
	// marker detection once (both channels). Emitting here rather than at the top
	// of deliverCR keeps it single-shot: re-detected markers whose CR already
	// exists return above and never reach this line.
	marker := stepToMarker[step]
	if marker == "" {
		// Defensive: an unmapped/empty step must never produce an empty marker in
		// the audit line. Fall back to the step (or a sentinel) so the record stays
		// diagnosable.
		if marker = step; marker == "" {
			marker = unknownLabel
		}
	}
	logx.As().Info().Str("reason", ReasonMarkerDetected.String()).Str("operation_id", operationID).
		Str("marker", marker).Str("step", step).Msg("upgrade marker detected")
	f.events.info(operationID, ReasonMarkerDetected.String(), "Detected "+marker)

	if _, err := f.store.Persist(operationID, step, cr); err != nil {
		// Already an errx-decorated persist (disk) error.
		return err
	}

	resource := f.client.Resource(gvr).Namespace(f.cfg.Namespace)
	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	if _, err := resource.Create(createCtx, cr, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// Created on a prior tick (e.g. a race past the dedup query). Record it
			// so future ticks hit the cache, drop the now-redundant persisted file,
			// and treat as success.
			f.markCreated(operationID, step, "")
			f.clearRejected(operationID, step)
			if delErr := f.store.Delete(operationID, step); delErr != nil {
				logx.As().Warn().Err(delErr).Str("reason", ReasonCRPersistCleanupFailed.String()).
					Str("operation_id", operationID).Msg("failed to delete persisted CR after AlreadyExists")
			}
			logx.As().Debug().Str("reason", ReasonCRExists.String()).Str("operation_id", operationID).
				Str("step", step).Str("cr_name", cr.GetName()).Msg("CR already exists — skipping create")
			return nil
		}
		if k8serrors.IsInvalid(err) {
			// A ValidatingAdmissionPolicy rejected the write deterministically
			// (#1133): the CR violates an enforced label invariant. The shipped policy
			// denies with reason=Invalid, so the rejection surfaces here as IsInvalid.
			// Because the UC always stamps the operation-id and step labels correctly,
			// this normally means a UC bug — hence the loud ERROR. Re-sending the
			// identical CR is pointless until something changes, but stopping outright
			// would strand the UC if the VAP itself were the thing misconfigured: a VAP
			// is fixed independently of the UC (kubectl / helm) and without restarting
			// it. So drop the persisted copy and back off (recordRejected) — subsequent
			// ticks short-circuit until admissionRejectRetryCooldown lapses, then retry,
			// so a fixed VAP self-heals without a consensus-node restart.
			//
			// Forbidden (403) is deliberately NOT handled here: it is also returned for
			// RBAC/authorization gaps, which ARE transient (fixable by granting the UC
			// ServiceAccount create permission). Those fall through to the retryable
			// CRCreateFailed path below, which keeps the persisted CR so restart-recovery
			// can re-send it once permissions are fixed.
			logx.As().Error().Err(err).Str("reason", ReasonCRRejected.String()).
				Str("operation_id", operationID).Str("step", step).
				Str("cr_name", cr.GetName()).Str("cr_kind", kind).
				Msg("admission rejected CR (label invariant violation) — backing off; the UC always sets the required labels, so this usually indicates a UC bug or a mis-configured VAP")
			f.events.error(operationID, ReasonCRRejected.String(), "Admission rejected "+kind+"/"+cr.GetName())
			if delErr := f.store.Delete(operationID, step); delErr != nil {
				logx.As().Warn().Err(delErr).Str("reason", ReasonCRPersistCleanupFailed.String()).
					Str("operation_id", operationID).Msg("failed to delete persisted CR after admission rejection")
			}
			f.recordRejected(operationID, step)
			return nil
		}
		f.invalidate(operationID)
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "create %s %s", kind, cr.GetName()), ReasonCRCreateFailed),
			"Confirm the UC ServiceAccount has create permission on "+gvr.Resource+"."+gvr.Group,
			"Check API server connectivity and that the CRD is installed",
		)
	}

	// Success — record in the cache so steady-state ticks skip the API query, and
	// drop the persisted copy now that the cluster owns the CR.
	f.markCreated(operationID, step, "")
	f.clearRejected(operationID, step)
	if err := f.store.Delete(operationID, step); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonCRPersistCleanupFailed.String()).
			Str("operation_id", operationID).Msg("failed to delete persisted CR after create")
	}
	logx.As().Info().Str("reason", ReasonCRDelivered.String()).Str("operation_id", operationID).
		Str("cr_name", cr.GetName()).Str("cr_kind", kind).Msg("created CR")
	f.events.info(operationID, ReasonCRDelivered.String(),
		"Created "+kind+"/"+cr.GetName())
	return nil
}

// HandleFreezeScheduled creates a NetworkUpgradeFreeze CR. The consensus node
// writes freeze_scheduled.mf after submitting a FREEZE_UPGRADE transaction.
//
// The handler re-derives the operationId deterministically from the co-present
// execute_immediate.mf (same inputs as HandleExecuteImmediate — identical value
// every tick). It then looks up the NetworkUpgradePrepare CR by exact operationId
// to copy spec.images and status.selectedSources.
//
// Staleness guard 1: if freeze_scheduled.mf is older than execute_immediate.mf,
// the marker is stale (leftover from a prior upgrade cycle while the CN has not
// yet cleaned it up) and is skipped until refreshed.
// Staleness guard 2: after the Prepare CR is resolved, if the marker predates
// the Prepare CR creationTimestamp, it is also stale and skipped.
//
// Gate: the Prepare CR MUST be Succeeded. If Failed, the handler halts. If
// in-progress, the handler retries on the next tick.
//
// Fallback: if execute_immediate.mf is absent, the handler attempts to resolve
// the operationId via manifest version (cluster + disk Prepare lookup).
func (f *Factory) HandleFreezeScheduled(ctx context.Context) error {
	// Stat freeze marker for mtime — needed for staleness guards.
	freezePath := filepath.Join(f.cfg.UpgradePath, MarkerFreezeScheduled)
	freezeInfo, err := os.Stat(freezePath)
	if err != nil {
		return errx.WithReason(errPersist.Wrap(err, "stat marker %s", MarkerFreezeScheduled), ReasonMarkerStatFailed)
	}
	freezeMarkerMtime := freezeInfo.ModTime()

	// Parse scheduledTime from file content (epoch seconds written by CN), not mtime.
	scheduledTime, err := parseFreezeScheduledTime(f.cfg.UpgradePath)
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerFreezeScheduled).
				Msg("cannot parse scheduled timestamp in freeze_scheduled.mf — halting; delete freeze_scheduled.mf to stop retrying")
			return nil
		}
		return err
	}

	// Primary: re-derive operationId from co-present execute_immediate.mf.
	operationID, executeMtime, _, found, err := deriveOperationIDPrimary(f.cfg.UpgradePath, f.cfg.CapsuleName())
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerFreezeScheduled).
				Msg("invalid consensus-node-components.yaml — halting freeze; delete freeze_scheduled.mf to stop retrying")
			return nil
		}
		return err
	}

	if found {
		// Staleness guard 1: marker must be newer than execute_immediate.mf.
		if !freezeMarkerMtime.After(executeMtime) {
			logx.As().Warn().Str("reason", ReasonStaleMarker.String()).
				Str("marker", MarkerFreezeScheduled).
				Time("freeze_mtime", freezeMarkerMtime).Time("execute_mtime", executeMtime).
				Msg("stale freeze_scheduled.mf — marker predates current execute_immediate.mf; skipping until refreshed")
			return nil
		}
	} else {
		// execute_immediate.mf absent — attempt fallback via manifest version.
		logx.As().Warn().Str("reason", ReasonFreezeWithoutPrepare.String()).
			Str("marker", MarkerFreezeScheduled).
			Msg("execute_immediate.mf missing — attempting fallback resolution via manifest version")

		manifestPath := filepath.Join(f.cfg.UpgradePath, manifestsSubdir, componentsManifestFile)
		manifest, mErr := parseComponentsManifest(manifestPath)
		if mErr != nil {
			logx.As().Error().Err(mErr).Str("reason", ReasonOperationIDExhausted.String()).
				Str("marker", MarkerFreezeScheduled).
				Msg("execute_immediate.mf and manifest both absent — cannot resolve operationId; delete freeze_scheduled.mf to stop retrying")
			return nil
		}

		skipFailed := map[string]struct{}{string(operatorv1alpha1.PhaseFailed): {}}
		operationID, err = f.resolveByVersion(ctx, manifest.Images.ConsensusNode.Version, skipFailed)
		if err != nil {
			return err
		}
		if operationID == "" {
			logx.As().Error().Str("reason", ReasonOperationIDExhausted.String()).
				Str("marker", MarkerFreezeScheduled).
				Msg("all operationId fallbacks exhausted — no matching Prepare found; delete freeze_scheduled.mf to stop retrying")
			return nil
		}
		logx.As().Warn().Str("reason", ReasonFreezeWithoutPrepare.String()).
			Str("operation_id", operationID).Str("marker", MarkerFreezeScheduled).
			Msg("resolved operationId via fallback (execute_immediate.mf was absent)")
	}

	images, selectedSources, prepare, ready, err := f.fetchPreparePayload(ctx, operationID)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}

	// Staleness guard 2: marker must be newer than the Prepare CR.
	if prepareCreationTime := prepare.GetCreationTimestamp().Time; !freezeMarkerMtime.After(prepareCreationTime) {
		logx.As().Warn().Str("reason", ReasonStaleMarker.String()).
			Str("marker", MarkerFreezeScheduled).Str("operation_id", operationID).
			Time("freeze_mtime", freezeMarkerMtime).Time("prepare_created", prepareCreationTime).
			Msg("stale freeze_scheduled.mf — marker predates Prepare CR; skipping until refreshed")
		return nil
	}

	cr, err := buildFreezeCR(f.cfg, operationID, scheduledTime, images, selectedSources)
	if err != nil {
		return err
	}
	return f.deliverCR(ctx, networkUpgradeFreezeGVR, operationID, operatorv1alpha1.StepFreeze, kindNetworkUpgradeFreeze, cr)
}

// fetchPreparePayload looks up the NetworkUpgradePrepare CR for operationID and
// returns its spec.images, status.selectedSources, and the CR itself (for the
// staleness guard in the caller).
//
// ready is true only when the Prepare CR exists and is in Succeeded phase. It is
// false (and err is nil) in two cases:
//   - the Prepare CR is not yet in the cluster (very early tick; retry),
//   - the Prepare is still in-progress (PrepareReconciler not done; retry).
//
// ready is false and err is nil with a Halted log when the Prepare has Failed —
// a Failed Prepare cannot recover, so there is no retry value.
func (f *Factory) fetchPreparePayload(ctx context.Context, operationID string) (operatorv1alpha1.UpgradeImagesSpec, operatorv1alpha1.SelectedSources, *unstructured.Unstructured, bool, error) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{
			FieldSelector: "spec.operationId=" + operationID,
		})
	if err != nil {
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradePrepare for operationId %s", operationID), ReasonPrepareListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradeprepares."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	if len(list.Items) == 0 {
		logx.As().Warn().Str("reason", ReasonFreezeWithoutPrepare.String()).
			Str("operation_id", operationID).Str("marker", MarkerFreezeScheduled).
			Msg("no NetworkUpgradePrepare CR found for operationId — will retry next tick")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	}

	prepare := &list.Items[0]
	phase, _, _ := unstructured.NestedString(prepare.Object, "status", "phase")
	switch operatorv1alpha1.Phase(phase) {
	case operatorv1alpha1.PhaseFailed:
		logx.As().Error().Str("reason", ReasonPrepareFailed.String()).
			Str("operation_id", operationID).Str("marker", MarkerFreezeScheduled).
			Msg("NetworkUpgradePrepare has Failed — halting freeze; delete freeze_scheduled.mf to clear")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	case operatorv1alpha1.PhaseSucceeded:
		// fall through to copy payload
	default:
		// Still in-progress; retry next tick.
		logx.As().Debug().Str("reason", ReasonFreezeWithoutPrepare.String()).
			Str("operation_id", operationID).Str("phase", phase).Str("marker", MarkerFreezeScheduled).
			Msg("NetworkUpgradePrepare not yet Succeeded — will retry next tick")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	}

	var images operatorv1alpha1.UpgradeImagesSpec
	if raw, ok, _ := unstructured.NestedMap(prepare.Object, "spec", "images"); ok {
		if err2 := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &images); err2 != nil {
			return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false,
				errx.WithReason(errCRBuild.Wrap(err2, "decode NetworkUpgradePrepare spec.images for %s", operationID), ReasonCRBuildFailed)
		}
	}

	var selectedSources operatorv1alpha1.SelectedSources
	if raw, ok, _ := unstructured.NestedMap(prepare.Object, "status", "selectedSources"); ok && raw != nil {
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &selectedSources)
	}

	return images, selectedSources, prepare, true, nil
}

// HandleNowFrozen creates a NetworkUpgradeExecute CR. The consensus node writes
// now_frozen.mf once the network reaches the scheduled freeze — the signal to
// begin the execute phase.
//
// The handler re-derives the operationId deterministically from the co-present
// execute_immediate.mf (same inputs as HandleExecuteImmediate — identical value
// every tick), then looks up the NetworkUpgradeFreeze CR by exact operationId to
// copy spec.images and spec.selectedSources. The Freeze CR carried both verbatim
// from the Prepare CR, so the Execute CR stays self-contained and pins the
// registry selection made once at prepare.
//
// Single-writer rule: the UC creates the Execute CR and never writes its
// status.phase. The operator sets ReadyForProvisionerDaemon; in cluster-only mode
// the ProvisionerProxy advances it to PendingNodeUpgrade (the solo-provisioner
// daemon does so in mainnet mode), and the UpgradeOutcomeMonitor watches the CR
// to its terminal Succeeded/Failed outcome.
//
// Staleness guard 1: if now_frozen.mf is older than execute_immediate.mf, the
// marker is stale (leftover from a prior upgrade cycle) and is skipped until
// refreshed.
// Staleness guard 2: after the Freeze CR is resolved, if the marker predates the
// Freeze CR creationTimestamp, it is also stale and skipped.
//
// Gate: the Freeze CR MUST be Succeeded. If Failed, the handler halts at ERROR
// (reason=CRFailed) telling the operator to delete now_frozen.mf to clear — a
// failed freeze must not produce an Execute CR, and gating here surfaces the
// failure instead of the misleading DEBUG deliverCR would log on a later cycle.
// If the Freeze is still in-progress, the handler retries on the next tick.
//
// Fallback: if execute_immediate.mf is absent, the handler resolves the
// operationId via manifest version (cluster + disk Prepare lookup).
func (f *Factory) HandleNowFrozen(ctx context.Context) error {
	// Stat now_frozen.mf for mtime — needed for the staleness guards.
	frozenPath := filepath.Join(f.cfg.UpgradePath, MarkerNowFrozen)
	frozenInfo, err := os.Stat(frozenPath)
	if err != nil {
		return errx.WithReason(errPersist.Wrap(err, "stat marker %s", MarkerNowFrozen), ReasonMarkerStatFailed)
	}
	frozenMarkerMtime := frozenInfo.ModTime()

	// Primary: re-derive operationId from co-present execute_immediate.mf.
	operationID, executeMtime, _, found, err := deriveOperationIDPrimary(f.cfg.UpgradePath, f.cfg.CapsuleName())
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerNowFrozen).
				Msg("invalid consensus-node-components.yaml — halting execute; delete now_frozen.mf to stop retrying")
			return nil
		}
		return err
	}

	if found {
		// Staleness guard 1: skip only when the marker STRICTLY predates
		// execute_immediate.mf. Equal mtimes (possible at 1s filesystem resolution)
		// are not stale — treating them as stale would skip the marker forever.
		if frozenMarkerMtime.Before(executeMtime) {
			logx.As().Warn().Str("reason", ReasonStaleMarker.String()).
				Str("marker", MarkerNowFrozen).
				Time("frozen_mtime", frozenMarkerMtime).Time("execute_mtime", executeMtime).
				Msg("stale now_frozen.mf — marker predates current execute_immediate.mf; skipping until refreshed")
			return nil
		}
	} else {
		// execute_immediate.mf absent — attempt fallback via manifest version.
		logx.As().Warn().Str("reason", ReasonExecuteWithoutPrepare.String()).
			Str("marker", MarkerNowFrozen).
			Msg("execute_immediate.mf missing — attempting fallback resolution via manifest version")

		manifestPath := filepath.Join(f.cfg.UpgradePath, manifestsSubdir, componentsManifestFile)
		manifest, mErr := parseComponentsManifest(manifestPath)
		if mErr != nil {
			if errorx.IsOfType(mErr, errManifestInvalid) {
				// Manifest present but unusable (bad YAML/schema/version) — terminal;
				// re-reading the same bytes will not help. Halt with the accurate reason.
				logx.As().Error().Err(mErr).Str("reason", ReasonManifestInvalid.String()).
					Str("marker", MarkerNowFrozen).
					Msg("execute_immediate.mf absent and consensus-node-components.yaml is invalid — cannot resolve operationId; delete now_frozen.mf to stop retrying")
				return nil
			}
			// Transient read error (manifest absent or volume briefly unavailable) —
			// surface it so it is retried/reportable rather than silently swallowed.
			return mErr
		}

		skipFailed := map[string]struct{}{string(operatorv1alpha1.PhaseFailed): {}}
		operationID, err = f.resolveByVersion(ctx, manifest.Images.ConsensusNode.Version, skipFailed)
		if err != nil {
			return err
		}
		if operationID == "" {
			logx.As().Error().Str("reason", ReasonOperationIDExhausted.String()).
				Str("marker", MarkerNowFrozen).
				Msg("all operationId fallbacks exhausted — no matching Prepare found; delete now_frozen.mf to stop retrying")
			return nil
		}
		logx.As().Warn().Str("reason", ReasonExecuteWithoutPrepare.String()).
			Str("operation_id", operationID).Str("marker", MarkerNowFrozen).
			Msg("resolved operationId via fallback (execute_immediate.mf was absent)")
	}

	images, selectedSources, freeze, ready, err := f.fetchExecutePayload(ctx, operationID)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}

	// Staleness guard 2: skip only when the marker STRICTLY predates the Freeze CR.
	// Equal timestamps (1s resolution) are not stale — see guard 1.
	if freezeCreationTime := freeze.GetCreationTimestamp().Time; frozenMarkerMtime.Before(freezeCreationTime) {
		logx.As().Warn().Str("reason", ReasonStaleMarker.String()).
			Str("marker", MarkerNowFrozen).Str("operation_id", operationID).
			Time("frozen_mtime", frozenMarkerMtime).Time("freeze_created", freezeCreationTime).
			Msg("stale now_frozen.mf — marker predates Freeze CR; skipping until refreshed")
		return nil
	}

	cr, err := buildExecuteCR(f.cfg, operationID, images, selectedSources)
	if err != nil {
		return err
	}
	return f.deliverCR(ctx, networkUpgradeExecuteGVR, operationID, operatorv1alpha1.StepExecute, kindNetworkUpgradeExecute, cr)
}

// fetchExecutePayload looks up the NetworkUpgradeFreeze CR for operationID and
// returns its spec.images, spec.selectedSources, and the CR itself (for the
// staleness guard in the caller). The Freeze CR carried both fields verbatim from
// the Prepare CR, so they represent the registry selection pinned at prepare;
// copying them keeps the Execute CR self-contained.
//
// ready is true only when the Freeze CR exists and is in Succeeded phase. It is
// false (and err is nil) in two cases, both worth a retry on the next tick:
//   - the Freeze CR is not yet in the cluster (early tick),
//   - the Freeze is still in-progress (FreezeReconciler not done).
//
// ready is false and err is nil with a Halted ERROR log when the Freeze has
// Failed — a Failed Freeze cannot recover, so there is no retry value and the
// Execute CR must not be created (mirrors the Prepare.Failed gate in
// fetchPreparePayload; see HandleNowFrozen's gate note).
func (f *Factory) fetchExecutePayload(ctx context.Context, operationID string) (operatorv1alpha1.UpgradeImagesSpec, operatorv1alpha1.SelectedSources, *unstructured.Unstructured, bool, error) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	list, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{
			FieldSelector: "spec.operationId=" + operationID,
		})
	if err != nil {
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradeFreeze for operationId %s", operationID), ReasonFreezeListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradefreezes."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	if len(list.Items) == 0 {
		logx.As().Warn().Str("reason", ReasonExecuteWithoutFreeze.String()).
			Str("operation_id", operationID).Str("marker", MarkerNowFrozen).
			Msg("no NetworkUpgradeFreeze CR found for operationId — will retry next tick")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	}

	freeze := &list.Items[0]
	phase, _, _ := unstructured.NestedString(freeze.Object, "status", "phase")
	switch operatorv1alpha1.Phase(phase) {
	case operatorv1alpha1.PhaseFailed:
		logx.As().Error().Str("reason", ReasonCRFailed.String()).
			Str("operation_id", operationID).Str("marker", MarkerNowFrozen).
			Msg("NetworkUpgradeFreeze has Failed — halting execute; delete now_frozen.mf to clear")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	case operatorv1alpha1.PhaseSucceeded:
		// fall through to copy payload
	default:
		// Still in-progress; retry next tick.
		logx.As().Debug().Str("reason", ReasonExecuteWithoutFreeze.String()).
			Str("operation_id", operationID).Str("phase", phase).Str("marker", MarkerNowFrozen).
			Msg("NetworkUpgradeFreeze not yet Succeeded — will retry next tick")
		return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false, nil
	}

	var images operatorv1alpha1.UpgradeImagesSpec
	if raw, ok, _ := unstructured.NestedMap(freeze.Object, "spec", "images"); ok {
		if err2 := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &images); err2 != nil {
			return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false,
				errx.WithReason(errCRBuild.Wrap(err2, "decode NetworkUpgradeFreeze spec.images for %s", operationID), ReasonCRBuildFailed)
		}
	}

	var selectedSources operatorv1alpha1.SelectedSources
	if raw, ok, _ := unstructured.NestedMap(freeze.Object, "spec", "selectedSources"); ok && raw != nil {
		if err2 := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &selectedSources); err2 != nil {
			// selectedSources pins the registry selection made at prepare; a decode
			// failure must surface, not silently yield an empty/partial pin.
			return operatorv1alpha1.UpgradeImagesSpec{}, operatorv1alpha1.SelectedSources{}, nil, false,
				errx.WithReason(errCRBuild.Wrap(err2, "decode NetworkUpgradeFreeze spec.selectedSources for %s", operationID), ReasonCRBuildFailed)
		}
	}

	return images, selectedSources, freeze, true, nil
}

// HandleFreezeAborted creates a NetworkUpgradeFreezeAbort CR. The UC re-derives
// the operationId deterministically from the co-present execute_immediate.mf and
// manifest (same inputs as prepare — identical value every tick). No phase gate:
// a cancellation must always be deliverable regardless of Prepare/Freeze state;
// the FreezeAbortReconciler owns all state logic.
//
// Staleness guard: if freeze_aborted.mf is older than execute_immediate.mf, the
// marker is stale (leftover from a prior upgrade cycle) and is skipped until
// refreshed.
//
// Fallback: if execute_immediate.mf is absent, resolves via manifest version
// (cluster + disk Prepare lookup, non-Failed phases).
func (f *Factory) HandleFreezeAborted(ctx context.Context) error {
	// Stat abort marker for mtime — needed for the staleness guard.
	abortPath := filepath.Join(f.cfg.UpgradePath, MarkerFreezeAborted)
	abortInfo, err := os.Stat(abortPath)
	if err != nil {
		return errx.WithReason(errPersist.Wrap(err, "stat marker %s", MarkerFreezeAborted), ReasonMarkerStatFailed)
	}
	abortMarkerMtime := abortInfo.ModTime()

	// Primary: re-derive operationId from co-present execute_immediate.mf.
	operationID, executeMtime, _, found, err := deriveOperationIDPrimary(f.cfg.UpgradePath, f.cfg.CapsuleName())
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerFreezeAborted).
				Msg("invalid consensus-node-components.yaml — halting abort; delete freeze_aborted.mf to stop retrying")
			return nil
		}
		return err
	}

	if found {
		// Staleness guard: abort marker must be newer than execute_immediate.mf.
		if !abortMarkerMtime.After(executeMtime) {
			logx.As().Warn().Str("reason", ReasonStaleMarker.String()).
				Str("marker", MarkerFreezeAborted).
				Time("abort_mtime", abortMarkerMtime).Time("execute_mtime", executeMtime).
				Msg("stale freeze_aborted.mf — marker predates current execute_immediate.mf; skipping until refreshed")
			return nil
		}
	} else {
		// execute_immediate.mf absent — attempt fallback via manifest version.
		logx.As().Warn().Str("reason", ReasonAbortWithoutPrepare.String()).
			Str("marker", MarkerFreezeAborted).
			Msg("execute_immediate.mf missing — attempting fallback resolution via manifest version")

		manifestPath := filepath.Join(f.cfg.UpgradePath, manifestsSubdir, componentsManifestFile)
		manifest, mErr := parseComponentsManifest(manifestPath)
		if mErr != nil {
			logx.As().Error().Err(mErr).Str("reason", ReasonOperationIDExhausted.String()).
				Str("marker", MarkerFreezeAborted).
				Msg("execute_immediate.mf and manifest both absent — cannot resolve operationId; delete freeze_aborted.mf to stop retrying")
			return nil
		}

		skipFailed := map[string]struct{}{string(operatorv1alpha1.PhaseFailed): {}}
		operationID, err = f.resolveByVersion(ctx, manifest.Images.ConsensusNode.Version, skipFailed)
		if err != nil {
			return err
		}
		if operationID == "" {
			logx.As().Error().Str("reason", ReasonOperationIDExhausted.String()).
				Str("marker", MarkerFreezeAborted).
				Msg("all operationId fallbacks exhausted — no matching Prepare found; delete freeze_aborted.mf to stop retrying")
			return nil
		}
		logx.As().Warn().Str("reason", ReasonAbortWithoutPrepare.String()).
			Str("operation_id", operationID).Str("marker", MarkerFreezeAborted).
			Msg("resolved operationId via fallback (execute_immediate.mf was absent)")
	}

	cr, err := buildFreezeAbortCR(f.cfg, operationID)
	if err != nil {
		return err
	}
	return f.deliverCR(ctx, networkUpgradeFreezeAbortGVR, operationID, operatorv1alpha1.StepAbort, kindNetworkUpgradeFreezeAbort, cr)
}
