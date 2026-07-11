// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/logx"
	"k8s.io/client-go/dynamic"

	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/uc/events"
)

// MarkerMonitor is the core UC behaviour: it polls the marker directory and, for
// each marker that appears, constructs and creates the corresponding
// NetworkUpgrade* CR (persist-before-send). This is the daemonkit.MonitorRunner
// that implements stories 8.1–8.8.
//
// Skeleton status: the poll loop and interval switching (8.1) are wired; CR
// construction (8.3–8.6), persist-before-send (8.2), dedup cache (8.7) and
// restart recovery (8.8) are delegated to the factory and are TODO stubs.
type MarkerMonitor struct {
	cfg     Config
	factory markerDispatcher

	connectivityErr atomic.Pointer[daemonkit.StatusError]
}

// markerDispatcher routes a detected marker to the handler that constructs the
// corresponding NetworkUpgrade* CR. *Factory is the production implementation;
// the seam exists so the poll loop can be exercised in tests without a K8s
// client (the per-marker handlers themselves are covered by #993-#996).
type markerDispatcher interface {
	RecoverOnStartup(ctx context.Context) error
	HandleExecuteImmediate(ctx context.Context) error
	HandleFreezeScheduled(ctx context.Context) error
	HandleNowFrozen(ctx context.Context) error
	HandleFreezeAborted(ctx context.Context) error
}

// NewMarkerMonitor constructs the marker monitor.
func NewMarkerMonitor(cfg Config, client dynamic.Interface) *MarkerMonitor {
	return &MarkerMonitor{
		cfg:     cfg,
		factory: NewFactory(cfg, client),
	}
}

// Name implements daemonkit.MonitorRunner.
func (m *MarkerMonitor) Name() string { return "marker-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (m *MarkerMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

// RequiredProbe implements daemonkit.ProbableMonitor: the marker directory and
// the CR-persist directory must be present and writable before polling begins.
// In cluster-only mode the previous upgrade directory is also probed for write
// access, since the provisioner-proxy rotates upgrade packages into it.
func (m *MarkerMonitor) RequiredProbe() daemonkit.Probe {
	probes := []daemonkit.Probe{
		&daemonkit.TaggedProbe{
			Inner:      &daemonkit.DiskPermissionProbe{Path: m.cfg.UpgradePath, Permission: 0o555},
			Reason:     ReasonUpgradePathUnreadable.String(),
			Resolution: "Verify the upgrade directory is mounted and readable: ls -la " + m.cfg.UpgradePath,
		},
		&daemonkit.TaggedProbe{
			Inner:      &daemonkit.DiskWriteTestProbe{Dir: m.cfg.CRPersistPath},
			Reason:     ReasonCRPersistPathUnwritable.String(),
			Resolution: "Verify the CR-persist directory exists and is writable: ls -la " + m.cfg.CRPersistPath,
		},
	}
	if m.cfg.ProvisionerProxyEnabled() {
		probes = append(probes, &daemonkit.TaggedProbe{
			Inner:      &daemonkit.DiskWriteTestProbe{Dir: m.cfg.PreviousUpgradePath},
			Reason:     ReasonPreviousUpgradePathUnwritable.String(),
			Resolution: "Verify the previous upgrade directory is writable: ls -la " + m.cfg.PreviousUpgradePath,
		})
	}
	return daemonkit.NewCompositeProbe("uc-marker", probes...)
}

// Run polls the marker directory until ctx is cancelled.
//
// Story 8.1 — polling loop: 60s normal interval, switches to 1s during the freeze
// window (after freeze_scheduled.mf is detected) and reverts after now_frozen.mf
// or freeze_aborted.mf is processed.
//
// Story 8.8 — on startup the factory reconciles cluster + persisted CRs + markers
// to resume in-flight upgrades idempotently.
func (m *MarkerMonitor) Run(ctx context.Context) error {
	logx.As().Info().
		Str("reason", ReasonMarkerMonitorStarted.String()).
		Str("upgrade_path", m.cfg.UpgradePath).
		Dur("poll_interval", m.cfg.PollInterval).
		Msg("marker monitor started")

	// Startup retention: keep only the last events.RetainCount upgrade-*.jsonl
	// event logs (story 9.1, same policy as the consensus daemon). A failure is
	// tolerated — stale audit logs must never block the monitor from starting.
	if err := events.Prune(m.cfg.EventsPath); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonEventLogPruneFailed.String()).
			Str("events_path", m.cfg.EventsPath).
			Msg("failed to prune upgrade event logs — continuing")
	}

	// TODO(#998): restart recovery — reconcile cluster (source of truth) -> disk
	// (--cr-persist-path) -> markers, resume status watching, clean up completed
	// upgrade artifacts. Then seed the in-memory operation-id cache (#997).
	if err := m.factory.RecoverOnStartup(ctx); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonUCStartupRecoveryError.String()).
			Msg("startup recovery encountered an error — continuing")
	}

	interval := m.cfg.PollInterval
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logx.As().Info().Str("reason", ReasonMarkerMonitorStopped.String()).Msg("marker monitor stopped")
			return nil
		case <-timer.C:
			next := m.poll(ctx, interval)
			if next != interval {
				logx.As().Info().Str("reason", ReasonPollIntervalChanged.String()).
					Dur("from", interval).Dur("to", next).Msg("poll interval changed")
				interval = next
			}
			timer.Reset(interval)
		}
	}
}

// poll scans the marker directory once and dispatches any markers found. It
// returns the poll interval to use for the next tick (fast during the freeze
// window, normal otherwise).
func (m *MarkerMonitor) poll(ctx context.Context, current time.Duration) time.Duration {
	next := current
	for _, name := range AllMarkers {
		if _, err := os.Stat(filepath.Join(m.cfg.UpgradePath, name)); err != nil {
			continue
		}
		if err := m.dispatch(ctx, name); err != nil {
			common.LogErr(logx.As().Error(), err, ReasonMarkerHandlerFailed).
				Str("marker", name).Msg("failed to handle marker")
			continue
		}
		switch name {
		case MarkerFreezeScheduled:
			next = m.cfg.FreezePollInterval // enter freeze window — poll fast
		case MarkerNowFrozen, MarkerFreezeAborted:
			next = m.cfg.PollInterval // freeze window over — revert
		}
	}
	return next
}

// dispatch routes a detected marker to its factory handler. The factory is
// responsible for dedup (8.7) so an already-processed marker is a no-op.
func (m *MarkerMonitor) dispatch(ctx context.Context, name string) error {
	switch name {
	case MarkerExecuteImmediate:
		return m.factory.HandleExecuteImmediate(ctx) // #993 -> NetworkUpgradePrepare
	case MarkerFreezeScheduled:
		return m.factory.HandleFreezeScheduled(ctx) // #994 -> NetworkUpgradeFreeze
	case MarkerNowFrozen:
		return m.factory.HandleNowFrozen(ctx) // #995 -> NetworkUpgradeExecute
	case MarkerFreezeAborted:
		return m.factory.HandleFreezeAborted(ctx) // #996 -> NetworkUpgradeFreezeAbort
	default:
		return nil
	}
}
