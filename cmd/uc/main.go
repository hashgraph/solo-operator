// SPDX-License-Identifier: Apache-2.0

// Command uc is the Upgrade Controller (UC) sidecar. It runs alongside a Hedera
// consensus node, polls the host-mounted marker directory, and translates marker
// files into NetworkUpgrade* Custom Resources (see HIP XXXX2, "UC Sidecar").
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/automa-saga/logx"
	"github.com/automa-saga/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/hashgraph/solo-operator/internal/uc"
)

// envPrefix namespaces the UC environment variables, e.g. --node-id maps to
// UC_NODE_ID. This lets the sidecar be configured via container env vars
// (sourced from the CR spec, a ConfigMap, or the downward API) instead of args.
const envPrefix = "UC_"

var (
	flagLogLevel string
	flagVersion  bool

	cfg = uc.DefaultConfig()
)

func main() {
	root := &cobra.Command{
		Use:   "uc",
		Short: "Upgrade Controller sidecar for Hedera consensus nodes",
		Long: "The UC sidecar polls the marker-file directory written by the consensus node " +
			"and creates NetworkUpgrade* Custom Resources. In cluster-only mode it also " +
			"advances the Execute CR from ReadyForProvisionerDaemon to PendingNodeUpgrade.",
		PersistentPreRunE: bindEnv,
		RunE:              run,
	}

	root.AddCommand(versionCmd(), statusCmd(), healthCmd())

	f := root.Flags()
	f.BoolVarP(&flagVersion, "version", "v", false, "Print version and exit")
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

// bindEnv applies UC_-prefixed environment variables to any flag the user did
// not pass explicitly on the command line. Precedence is: CLI flag > env var >
// default. The env var name is the flag name uppercased with '-' replaced by
// '_' and the UC_ prefix, e.g. --node-id -> UC_NODE_ID, --poll-interval ->
// UC_POLL_INTERVAL.
//
// It is wired as PersistentPreRunE so it also runs for subcommands: `uc status`
// and `uc health` honour UC_SOCKET_PATH for their --socket-path flag. Each
// invocation visits only the executing command's own flag set.
func bindEnv(cmd *cobra.Command, _ []string) error {
	var bindErr error
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if bindErr != nil || f.Changed {
			return
		}
		env := envPrefix + strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")
		if v, ok := os.LookupEnv(env); ok {
			if err := f.Value.Set(v); err != nil {
				bindErr = fmt.Errorf("invalid value for %s: %w", env, err)
			}
		}
	})
	return bindErr
}

// versionCmd prints the binary's build metadata, stamped at build time via
// -ldflags -X into github.com/automa-saga/version. It honours an -o/--output
// flag (text or json); text is the default.
func versionCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information and exit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := version.Get().Format(format)
			if err != nil {
				return err
			}
			cmd.Println(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", version.FormatText, "Output format: text or json")
	return cmd
}

// statusCmd queries the running UC's control server over its Unix socket and
// prints the per-monitor status. It is the operator-facing counterpart to the
// daemonkit GET /status endpoint, so a human need not hand-roll curl.
func statusCmd() *cobra.Command {
	socketPath := uc.DefaultSocketPath
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Query the running UC sidecar's status over its control socket",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := uc.NewControlClient(socketPath).StatusPretty(cmd.Context())
			if err != nil {
				return err
			}
			cmd.Println(out)
			return nil
		},
	}
	cmd.Flags().StringVar(&socketPath, "socket-path", socketPath, "Unix socket of the running UC control server")
	return cmd
}

// healthCmd probes the running UC's GET /health endpoint over its Unix socket.
// It prints "ok" and exits 0 when the process is live, or fails with the reason
// (unreachable socket / non-200) otherwise — usable as a scripted liveness check.
func healthCmd() *cobra.Command {
	socketPath := uc.DefaultSocketPath
	cmd := &cobra.Command{
		Use:          "health",
		Short:        "Probe the running UC sidecar's liveness over its control socket",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := uc.NewControlClient(socketPath).Health(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&socketPath, "socket-path", socketPath, "Unix socket of the running UC control server")
	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	if flagVersion {
		out, err := version.Get().Format(version.FormatText)
		if err != nil {
			return err
		}
		cmd.Println(out)
		return nil
	}

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
