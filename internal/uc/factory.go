// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/automa-saga/logx"
	"github.com/joomcode/errorx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
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

	// store implements persist-before-send (#992): each handler serializes its CR
	// under cfg.CRPersistPath before Create and deletes the file on success.
	store *CRStore

	// mu guards the operation-id cache (#997). The cache is a performance hint
	// only; correctness rests on the K8s API existence check before each create.
	mu    sync.Mutex
	cache map[string]map[string]struct{} // operationId -> set of created steps
}

// NewFactory constructs the CR factory.
func NewFactory(cfg Config, client dynamic.Interface) *Factory {
	return &Factory{
		cfg:    cfg,
		client: client,
		store:  NewCRStore(cfg.CRPersistPath),
		cache:  make(map[string]map[string]struct{}),
	}
}

// RecoverOnStartup reconciles cluster state, persisted CRs on disk, and marker
// files so the UC resumes in-flight upgrades without re-signalling.
//
// Today it seeds the operation-id dedup cache (#997) by querying every
// NetworkUpgrade* CR already in the namespace, so a restart does not re-create
// CRs for markers it handled before the restart. A seed query failure is logged
// but not fatal: the per-action cluster query remains authoritative, so an
// unseeded cache only costs one extra API call per operation.
//
// TODO(#998): re-send any CR persisted on disk but not yet in the cluster; clean
// up artifacts for upgrades that have already reached a terminal phase.
func (f *Factory) RecoverOnStartup(ctx context.Context) error {
	if err := f.seedCache(ctx); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonUCCacheSeedFailed.String()).
			Msg("could not seed op-id dedup cache on startup; per-action queries remain authoritative")
		return nil
	}
	logx.As().Info().Str("reason", ReasonUCCacheSeeded.String()).
		Msg("seeded op-id dedup cache from existing NetworkUpgrade CRs")
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
	markerPath := filepath.Join(f.cfg.MarkerPath, MarkerExecuteImmediate)
	info, err := os.Stat(markerPath)
	if err != nil {
		// The poller only dispatches after a successful Stat, so a failure here is
		// a genuine race/IO error worth retrying on the next tick.
		return errx.WithReason(errPersist.Wrap(err, "stat marker %s", MarkerExecuteImmediate), ReasonMarkerStatFailed)
	}

	manifestPath := filepath.Join(f.cfg.MarkerPath, manifestsSubdir, componentsManifestFile)
	manifest, err := parseComponentsManifest(manifestPath)
	if err != nil {
		if errorx.IsOfType(err, errManifestInvalid) {
			// ManifestInvalid is terminal — re-reading the same bytes will not fix
			// it. Log and halt this dispatch without creating a CR (no retry value).
			logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
				Str("marker", MarkerExecuteImmediate).Str("manifest", manifestPath).
				Msg("invalid consensus-node-components.yaml — halting prepare")
			return nil
		}
		// Already an errx-decorated transient (read) error; surface as-is.
		return err
	}

	operationID, err := mintOperationID(manifest.Images.ConsensusNode.Version, info.ModTime())
	if err != nil {
		logx.As().Error().Err(err).Str("reason", ReasonManifestInvalid.String()).
			Str("marker", MarkerExecuteImmediate).Msg("cannot derive a valid operationId — halting prepare")
		return nil
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
// failure worth retrying on the next tick. On a create failure the persisted
// file is intentionally left on disk for the restart-recovery flow (#998) and
// the cache entry is invalidated so a stale hint cannot mask a missing CR.
func (f *Factory) deliverCR(ctx context.Context, gvr schema.GroupVersionResource, operationID, step, kind string, cr *unstructured.Unstructured) error {
	exists, err := f.alreadyCreated(ctx, gvr, operationID, step)
	if err != nil {
		// Already an errx-decorated K8s API error from the dedup query.
		return err
	}
	if exists {
		logx.As().Debug().Str("reason", ReasonCRExists.String()).Str("operation_id", operationID).
			Str("step", step).Str("cr_name", cr.GetName()).Msg("CR already exists — skipping create")
		return nil
	}

	if _, err := f.store.Persist(operationID, step, cr); err != nil {
		// Already an errx-decorated persist (disk) error.
		return err
	}

	resource := f.client.Resource(gvr).Namespace(f.cfg.Namespace)
	if _, err := resource.Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// Created on a prior tick (e.g. a race past the dedup query). Record it
			// so future ticks hit the cache, drop the now-redundant persisted file,
			// and treat as success.
			f.markCreated(operationID, step)
			if delErr := f.store.Delete(operationID, step); delErr != nil {
				logx.As().Warn().Err(delErr).Str("reason", ReasonCRPersistCleanupFailed.String()).
					Str("operation_id", operationID).Msg("failed to delete persisted CR after AlreadyExists")
			}
			logx.As().Debug().Str("reason", ReasonCRExists.String()).Str("operation_id", operationID).
				Str("step", step).Str("cr_name", cr.GetName()).Msg("CR already exists — skipping create")
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
	f.markCreated(operationID, step)
	if err := f.store.Delete(operationID, step); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonCRPersistCleanupFailed.String()).
			Str("operation_id", operationID).Msg("failed to delete persisted CR after create")
	}
	logx.As().Info().Str("reason", ReasonCRDelivered.String()).Str("operation_id", operationID).
		Str("cr_name", cr.GetName()).Str("cr_kind", kind).Msg("created CR")
	return nil
}

// HandleFreezeScheduled creates a NetworkUpgradeFreeze CR, copying selectedSources
// from the existing NetworkUpgradePrepare CR (looked up by operation-id label).
//
// TODO(#994): implement Prepare lookup + selectedSources copy + persist-before-send.
func (f *Factory) HandleFreezeScheduled(_ context.Context) error {
	logx.As().Info().Str("reason", ReasonMarkerHandlerPending.String()).Str("marker", MarkerFreezeScheduled).
		Msg("freeze_scheduled.mf handler not yet implemented (#994)")
	return nil
}

// HandleNowFrozen creates a NetworkUpgradeExecute CR. The operator (not the UC)
// owns the status.phase transitions; the UC creates the CR and watches its status.
// In cluster-only mode the ProvisionerProxyMonitor advances the phase to
// PendingNodeUpgrade once the operator sets ReadyForProvisionerDaemon.
//
// TODO(#995): implement CR construction + persist-before-send + status watch for
// the DaemonResult / NodeStarting conditions.
func (f *Factory) HandleNowFrozen(_ context.Context) error {
	logx.As().Info().Str("reason", ReasonMarkerHandlerPending.String()).Str("marker", MarkerNowFrozen).
		Msg("now_frozen.mf handler not yet implemented (#995)")
	return nil
}

// HandleFreezeAborted creates a NetworkUpgradeFreezeAbort CR (#996). The UC
// mints no new identity here: per HIP-1496 the operationId is resolved from the
// most-recent non-terminal NetworkUpgradePrepare CR (cluster → on-disk
// fallback), so the abort inherits the upgrade it is cancelling. The UC takes no
// further action — the operator's FreezeAbortReconciler owns the logic
// (including the AbortAfterFreeze guard); prior Prepare/Freeze CRs are left in
// place as an audit trail.
func (f *Factory) HandleFreezeAborted(ctx context.Context) error {
	operationID, err := f.resolveOperationID(ctx)
	if err != nil {
		// Already an errx-decorated K8s/persist error from the resolver.
		return err
	}
	if operationID == "" {
		// No NetworkUpgradePrepare exists in the cluster (source of truth) or on
		// disk. In the normal protocol this cannot happen: execute_immediate.mf
		// precedes and co-resides with freeze_aborted.mf and is dispatched first
		// each tick (so Prepare is re-ensured before this runs), and Prepare CRs
		// are never deleted — an abort always finds its Prepare. Reaching here is a
		// genuine anomaly: the abort has no corresponding upgrade in the operator's
		// view, so there is nothing to cancel. The UC deliberately does NOT
		// fabricate an identity — a synthesized operationId would correlate to no
		// Prepare/Freeze/Execute CR, producing an orphan CR the operator cannot act
		// on. It records the anomaly and creates no CR.
		logx.As().Warn().Str("reason", ReasonAbortWithoutPrepare.String()).Str("marker", MarkerFreezeAborted).
			Msg("freeze_aborted.mf with no corresponding NetworkUpgradePrepare — nothing to cancel; no CR created")
		return nil
	}

	cr, err := buildFreezeAbortCR(f.cfg, operationID)
	if err != nil {
		return err
	}
	return f.deliverCR(ctx, networkUpgradeFreezeAbortGVR, operationID, operatorv1alpha1.StepAbort, kindNetworkUpgradeFreezeAbort, cr)
}
