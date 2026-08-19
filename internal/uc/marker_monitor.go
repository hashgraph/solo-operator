// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/errx"
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
	seen            map[string]struct{}
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
	return NewMarkerMonitorWithFactory(cfg, NewFactory(cfg, client))
}

// NewMarkerMonitorWithFactory constructs the marker monitor with an injected factory.
// Use when the factory must be shared (e.g. to wire onPhaseUpdate on UpgradeOutcomeMonitor).
func NewMarkerMonitorWithFactory(cfg Config, f *Factory) *MarkerMonitor {
	return &MarkerMonitor{
		cfg:     cfg,
		factory: f,
		seen:    make(map[string]struct{}),
	}
}

// Name implements daemonkit.MonitorRunner.
func (m *MarkerMonitor) Name() string { return "marker-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (m *MarkerMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

// RequiredProbe implements daemonkit.ProbableMonitor: the upgrade directory and
// the CR-persist directory must be present and writable before polling begins.
// The previous upgrade directory is also probed for writability in all modes
// because UpgradeOutcomeMonitor rotates/discards upgrade packages into it
// regardless of whether the provisioner-proxy is enabled.
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
		&daemonkit.TaggedProbe{
			Inner:      &daemonkit.DiskWriteTestProbe{Dir: m.cfg.PreviousUpgradePath},
			Reason:     ReasonPreviousUpgradePathUnwritable.String(),
			Resolution: "Verify the previous upgrade directory is writable: ls -la " + m.cfg.PreviousUpgradePath,
		},
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
func (m *MarkerMonitor) Run(ctx context.Context) (retErr error) {
	// Convert an unexpected panic anywhere on the poll goroutine (startup
	// recovery, the poll scan, or a marker handler) into a returned error so the
	// daemonkit supervisor restarts this monitor with back-off, instead of the
	// panic unwinding to the top of its goroutine and crashing the whole UC
	// process (daemonkit does not recover; a panic in any monitor kills the
	// daemon). Mirrors PlatformStatusMonitor.Run.
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", ReasonMarkerMonitorPanic.String()).
				Str("upgrade_path", m.cfg.UpgradePath).Interface("panic", r).
				Msg("panic in marker monitor — recovering")
			retErr = errx.WithReason(errPanic.New("recovered panic in marker monitor: %v", r), ReasonMarkerMonitorPanic)
		}
	}()

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
//
// Polling (not inotify/fsnotify) is intentional and load-bearing:
//   - It is resilient to a transiently absent UpgradePath directory. During
//     upgrade-package rotation, rotateUpgradePackage atomically renames
//     current/ to previous/ and then recreates current/. Between those two
//     steps current/ does not exist. A stat-based poll simply finds nothing and
//     moves on; an inotify watcher would receive a DELETE event, lose its watch
//     descriptor, and require re-registration logic. The poll interval (60 s
//     normal, 1 s during freeze) is far longer than the mkdir round-trip, so
//     the window is invisible in practice.
//   - inotify (and FSEvents/kqueue) are unreliable on network-backed volume
//     mounts (NFS, CIFS/SMB, FUSE). Polling works correctly regardless of
//     whether the shared volume is hostPath, emptyDir, or network-backed.
//   - It also survives volume remounts and container restarts without any watch
//     re-registration plumbing.
func (m *MarkerMonitor) poll(ctx context.Context, current time.Duration) time.Duration {
	if m.seen == nil {
		m.seen = make(map[string]struct{})
	}
	next := current
	for _, name := range AllMarkers {
		info, err := os.Stat(filepath.Join(m.cfg.UpgradePath, name))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Reset dedup so reappearance is logged; skip on transient I/O errors.
				delete(m.seen, name)
			}
			continue
		}
		if _, already := m.seen[name]; !already {
			detectionLatency := time.Since(info.ModTime()).Truncate(time.Millisecond)
			logx.As().Info().
				Str("reason", ReasonMarkerDetected.String()).
				Str("marker", name).
				Dur("latency", detectionLatency).
				Msg("marker detected")
			m.seen[name] = struct{}{}
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
