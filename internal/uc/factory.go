// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/automa-saga/logx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
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
	store *CRStore //nolint:unused // consumed by the CR handlers in #993-#998

	// mu guards the operation-id cache (#997). The cache is a performance hint
	// only; correctness rests on the K8s API existence check before each create.
	mu    sync.Mutex                     //nolint:unused // guards cache; wired by the handlers in #993-#998
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
// TODO(#997): seed the operation-id cache by querying all NetworkUpgrade* CRs
// with a matching operation-id label.
// TODO(#998): re-send any CR persisted on disk but not yet in the cluster; clean
// up artifacts for upgrades that have already reached a terminal phase.
func (f *Factory) RecoverOnStartup(_ context.Context) error {
	logx.As().Info().Str("reason", "UCStartupRecoveryPending").
		Msg("startup recovery not yet implemented (#997, #998)")
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
		return fmt.Errorf("stat marker %s: %w", MarkerExecuteImmediate, err)
	}

	manifestPath := filepath.Join(f.cfg.MarkerPath, manifestsSubdir, componentsManifestFile)
	manifest, err := parseComponentsManifest(manifestPath)
	if err != nil {
		if errors.Is(err, errManifestInvalid) {
			// ManifestInvalid is terminal — re-reading the same bytes will not fix
			// it. Log and halt this dispatch without creating a CR (no retry value).
			logx.As().Error().Err(err).Str("reason", "ManifestInvalid").
				Str("marker", MarkerExecuteImmediate).Str("manifest", manifestPath).
				Msg("invalid consensus-node-components.yaml — halting prepare")
			return nil
		}
		return fmt.Errorf("parse %s: %w", componentsManifestFile, err)
	}

	operationID, err := mintOperationID(manifest.Images.ConsensusNode.Version, info.ModTime())
	if err != nil {
		logx.As().Error().Err(err).Str("reason", "ManifestInvalid").
			Str("marker", MarkerExecuteImmediate).Msg("cannot derive a valid operationId — halting prepare")
		return nil
	}

	cr, err := buildPrepareCR(f.cfg, operationID, manifest.Images)
	if err != nil {
		return fmt.Errorf("build NetworkUpgradePrepare for %s: %w", operationID, err)
	}

	// Persist-before-send (#992): the CR hits disk before any K8s API call so a
	// crash in the send window leaves it recoverable on the next startup.
	if _, err := f.store.Persist(operationID, operatorv1alpha1.StepPrepare, cr); err != nil {
		return fmt.Errorf("persist NetworkUpgradePrepare %s: %w", operationID, err)
	}

	resource := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace)
	if _, err := resource.Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// The CR was already created on a prior tick (first-detection-wins). The
			// persisted file is now redundant — drop it and treat as success. The
			// authoritative dedup is this API-level check (#997 only caches it).
			if delErr := f.store.Delete(operationID, operatorv1alpha1.StepPrepare); delErr != nil {
				logx.As().Warn().Err(delErr).Str("reason", "CRPersistCleanupFailed").
					Str("operation_id", operationID).Msg("failed to delete persisted CR after AlreadyExists")
			}
			logx.As().Debug().Str("reason", "PrepareCRExists").Str("operation_id", operationID).
				Str("cr_name", cr.GetName()).Msg("NetworkUpgradePrepare already exists — skipping create")
			return nil
		}
		// Leave the persisted file in place: it marks the crash/send window for the
		// restart-recovery flow (#998) to re-send.
		return fmt.Errorf("create NetworkUpgradePrepare %s: %w", cr.GetName(), err)
	}

	// Success — the cluster now owns the CR, so the persisted copy is no longer
	// needed for recovery.
	if err := f.store.Delete(operationID, operatorv1alpha1.StepPrepare); err != nil {
		logx.As().Warn().Err(err).Str("reason", "CRPersistCleanupFailed").
			Str("operation_id", operationID).Msg("failed to delete persisted CR after create")
	}
	logx.As().Info().Str("reason", "CRDelivered").Str("operation_id", operationID).
		Str("cr_name", cr.GetName()).Str("cr_kind", kindNetworkUpgradePrepare).
		Msg("created NetworkUpgradePrepare CR")
	return nil
}

// HandleFreezeScheduled creates a NetworkUpgradeFreeze CR, copying selectedSources
// from the existing NetworkUpgradePrepare CR (looked up by operation-id label).
//
// TODO(#994): implement Prepare lookup + selectedSources copy + persist-before-send.
func (f *Factory) HandleFreezeScheduled(_ context.Context) error {
	logx.As().Info().Str("reason", "MarkerHandlerPending").Str("marker", MarkerFreezeScheduled).
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
	logx.As().Info().Str("reason", "MarkerHandlerPending").Str("marker", MarkerNowFrozen).
		Msg("now_frozen.mf handler not yet implemented (#995)")
	return nil
}

// HandleFreezeAborted creates a NetworkUpgradeFreezeAbort CR.
//
// TODO(#996): implement CR construction + persist-before-send.
func (f *Factory) HandleFreezeAborted(_ context.Context) error {
	logx.As().Info().Str("reason", "MarkerHandlerPending").Str("marker", MarkerFreezeAborted).
		Msg("freeze_aborted.mf handler not yet implemented (#996)")
	return nil
}
