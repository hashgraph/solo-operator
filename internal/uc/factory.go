// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"sync"

	"github.com/automa-saga/logx"
	"k8s.io/client-go/dynamic"
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

// HandleExecuteImmediate parses consensus-node-components.yaml and
// infrastructure-versions.yaml from the deployment package and creates a
// NetworkUpgradePrepare CR.
//
// TODO(#993): implement manifest parsing + CR construction + persist-before-send.
func (f *Factory) HandleExecuteImmediate(_ context.Context) error {
	logx.As().Info().Str("reason", "MarkerHandlerPending").Str("marker", MarkerExecuteImmediate).
		Msg("execute_immediate.mf handler not yet implemented (#993)")
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
