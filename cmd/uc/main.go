// SPDX-License-Identifier: Apache-2.0

// Command uc is the Upgrade Controller (UC) sidecar. It runs alongside a Hedera
// consensus node, polls the host-mounted marker directory, and translates marker
// files into NetworkUpgrade* Custom Resources (see HIP XXXX2, "UC Sidecar").
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/automa-saga/logx"
	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/uc"
)

var (
	flagLogLevel string

	cfg = uc.DefaultConfig()
)

func main() {
	root := &cobra.Command{
		Use:   "uc",
		Short: "Upgrade Controller sidecar for Hedera consensus nodes",
		Long: "The UC sidecar polls the marker-file directory written by the consensus node " +
			"and creates NetworkUpgrade* Custom Resources. In cluster-only mode it also " +
			"advances the Execute CR from ReadyForProvisionerDaemon to PendingNodeUpgrade.",
		RunE: run,
	}

	f := root.Flags()
	f.StringVar((*string)(&cfg.Mode), "mode", string(cfg.Mode), "Deployment mode: mainnet or cluster-only")
	f.IntVar(&cfg.NodeID, "node-id", cfg.NodeID, "Consensus node identity (a non-negative integer, e.g. 0, 1, 2)")
	f.StringVar(&cfg.Orbit, "orbit", cfg.Orbit, "Deployment orbit the upgrade CRs target")
	f.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "Kubernetes namespace to watch and create CRs in")
	f.StringVar(&cfg.KubeconfigPath, "kubeconfig", cfg.KubeconfigPath, "Path to kubeconfig (empty = in-cluster)")
	f.StringVar(&cfg.MarkerPath, "marker-path", cfg.MarkerPath, "Directory polled for marker files")
	f.StringVar(&cfg.EventsPath, "events-path", cfg.EventsPath, "JSONL event-log directory")
	f.StringVar(&cfg.CRPersistPath, "cr-persist-path", cfg.CRPersistPath, "Directory for persisted CR YAML (persist-before-send)")
	f.StringVar(&cfg.BackupPath, "backup-path", cfg.BackupPath, "Pre-upgrade backup directory")
	f.StringVar(&cfg.SocketPath, "socket-path", cfg.SocketPath, "Unix socket for the control server")
	f.DurationVar(&cfg.PollInterval, "poll-interval", cfg.PollInterval, "Normal marker polling interval")
	f.DurationVar(&cfg.FreezePollInterval, "freeze-poll-interval", cfg.FreezePollInterval, "Polling interval during the freeze window")
	f.StringVar(&flagLogLevel, "log-level", "info", "Log level: debug, info, warn, error")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	if err := logx.Initialize(logx.LoggingConfig{
		Level:          flagLogLevel,
		ConsoleLogging: true,
	}); err != nil {
		return err
	}
	// Bridge the stdlib log/slog seam to logx so the daemonkit kernel (which
	// logs via slog) emits to the same zerolog sinks logx just configured.
	slog.SetDefault(slog.New(logx.NewSlogHandler()))

	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	u, err := uc.New(cfg)
	if err != nil {
		return err
	}
	return u.Run(ctx)
}
