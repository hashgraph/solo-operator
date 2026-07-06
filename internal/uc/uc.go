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

	monitors := []daemonkit.MonitorRunner{
		NewMarkerMonitor(cfg, client),
	}
	if cfg.ProvisionerProxyEnabled() {
		monitors = append(monitors, NewProvisionerProxyMonitorWithClient(
			ProvisionerProxyMonitorConfig{Namespace: cfg.Namespace, NodeID: cfg.NodeID},
			client))
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
	return nil
}
