// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/logx"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/dynamic"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// UC is the upgrade-controller sidecar. It runs two supervised monitors:
//
//  1. MarkerMonitor — polls marker files and creates NetworkUpgrade* CRs (the
//     core UC behaviour, all deployment modes).
//  2. ProvisionerProxyMonitor — cluster-only mode only; stands in for the host
//     solo-provisioner daemon, advancing Execute CRs ReadyForProvisionerDaemon ->
//     PendingNodeUpgrade.
//
// A daemonkit.Server exposes a Unix-socket control plane (/health, /status).
type UC struct {
	cfg      Config
	client   dynamic.Interface
	monitors []daemonkit.MonitorRunner
}

// New builds the UC from cfg, constructing the K8s client and the enabled monitors.
func New(cfg Config) (*UC, error) {
	client, err := buildDynamicClient(cfg.KubeconfigPath)
	if err != nil {
		// Already an errx-decorated client error.
		return nil, err
	}

	factory := NewFactory(cfg, client)
	monitors := []daemonkit.MonitorRunner{
		NewMarkerMonitorWithFactory(cfg, factory),
	}
	if cfg.ProvisionerProxyEnabled() {
		pm := NewProvisionerProxyMonitorWithClient(
			ProvisionerProxyMonitorConfig{
				Namespace:           cfg.Namespace,
				NodeID:              cfg.NodeID,
				UpgradePath:         cfg.UpgradePath,
				PreviousUpgradePath: cfg.PreviousUpgradePath,
			},
			client)
		pm.onPhaseUpdate = factory.markCreated
		monitors = append(monitors, pm)
	}

	return &UC{cfg: cfg, client: client, monitors: monitors}, nil
}

// Run starts the control server and all supervised monitors, blocking until ctx
// is cancelled or a fatal error occurs. It notifies systemd via sd_notify.
func (u *UC) Run(ctx context.Context) error {
	if err := u.ensureDirs(); err != nil {
		return err
	}

	tracker := daemonkit.NewStatusTracker()
	server := daemonkit.NewServer(u.cfg.SocketPath, daemonkit.ServerOptions{
		StatusFn: func() any { return u.status(tracker) },
		Logger:   slog.Default(),
	}, daemonkit.ServerConfig{})

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return server.Start(ctx)
	})

	// a permanently-failed monitor stays up in a degraded state — surfaced via /status, not a pod crash, so the
	// operator gets an actionable signal instead of restart churn
	for _, mon := range u.monitors {
		g.Go(func() error {
			daemonkit.SupervisedMonitor(ctx, mon, daemonkit.SupervisorOptions{
				Tracker: tracker,
				Logger:  slog.Default(),
			})
			return nil
		})
	}

	if err := daemonkit.NotifyReady(); err != nil {
		logx.As().Warn().Err(err).Str("reason", ReasonSdNotifyReadyFailed.String()).Msg("Failed to send READY=1 to systemd")
	}
	logx.As().Info().
		Str("reason", ReasonUCStarted.String()).
		Str("mode", string(u.cfg.Mode)).
		Int("monitors", len(u.monitors)).
		Msg("UC started")

	err := g.Wait()
	if nerr := daemonkit.NotifyStopping(); nerr != nil {
		logx.As().Warn().Err(nerr).Str("reason", ReasonSdNotifyStoppingFailed.String()).Msg("Failed to send STOPPING=1 to systemd")
	}
	return err
}

// status builds the GET /status payload: the per-monitor liveness from the
// StatusTracker, overlaid with each monitor's ConnectivityError so a monitor
// that is alive-but-failing (e.g. an API/RBAC error in its watch loop) shows up
// rather than reporting only "running". Mirrors the overlay solo-provisioner
// daemon does in its own status handler.
func (u *UC) status(tracker *daemonkit.StatusTracker) map[string]daemonkit.MonitorState {
	snap := tracker.Snapshot()
	for _, mon := range u.monitors {
		cm, ok := mon.(daemonkit.ConnectivityMonitor)
		if !ok {
			continue
		}
		st := snap[mon.Name()]
		st.Error = cm.ConnectivityError()
		snap[mon.Name()] = st
	}
	return snap
}

// ensureDirs creates the UC working directories if they do not exist.
// Upgrade directories (current/previous) are created with permissions inherited
// from their parent so ownership matches the consensus-node mount.
func (u *UC) ensureDirs() error {
	for _, dir := range []string{
		u.cfg.EventsPath,
		u.cfg.CRPersistPath,
		u.cfg.BackupPath,
		filepath.Dir(u.cfg.SocketPath),
	} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return errx.WithHints(
				errx.WithReason(errPersist.Wrap(err, "create dir %s", dir), ReasonWorkDirCreateFailed),
				"Confirm the UC working directories (--cr-persist-path, --events-path, --backup-path, --socket-path) are on writable volumes",
			)
		}
	}

	// Upgrade dirs inherit permissions from their own parent so we don't override
	// ownership set by the consensus-node container. Each dir's walk is capped at
	// its own parent — not a shared ancestor — so the cap is always meaningful
	// regardless of whether UpgradePath and PreviousUpgradePath are siblings.
	for _, dir := range []string{u.cfg.UpgradePath, u.cfg.PreviousUpgradePath} {
		if dir == "" {
			continue
		}
		parent := filepath.Dir(dir)
		if err := mkdirInherit(dir, parent); err != nil {
			return errx.WithHints(
				errx.WithReason(errPersist.Wrap(err, "create upgrade dir %s", dir), ReasonWorkDirCreateFailed),
				"Confirm the upgrade directory parent ("+parent+") is writable and mounted",
			)
		}
	}
	return nil
}

// mkdirInherit creates dir (and any missing parents) using the permission bits
// of the nearest existing ancestor, walking no higher than root. Callers pass
// filepath.Dir(dir) as root so the walk is always capped at the direct parent
// of the directory being created — it never inherits from an unrelated ancestor.
func mkdirInherit(dir, root string) error {
	// Walk up from dir to find the nearest existing ancestor, stopping at root.
	ancestor := dir
	hitFSRoot := false
	for ancestor != root {
		if _, err := os.Stat(ancestor); err == nil {
			break // this ancestor exists — use its mode
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			hitFSRoot = true // filesystem root reached before cap — don't inherit
			break
		}
		ancestor = parent
	}
	if hitFSRoot {
		return os.MkdirAll(dir, 0o750)
	}
	info, err := os.Stat(ancestor)
	if err != nil {
		// cap root itself doesn't exist — fall back to a safe default.
		return os.MkdirAll(dir, 0o750)
	}
	return os.MkdirAll(dir, info.Mode().Perm())
}
