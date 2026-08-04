// SPDX-License-Identifier: Apache-2.0

// Command beacon is the Solo HAPI control-plane CLI. It submits privileged
// HAPI transactions (the consensus-node software-upgrade flow: special-file
// upload, prepare-upgrade, freeze-upgrade, …) to a running Hedera network over
// gRPC, using hiero-sdk-go. It is the Go replacement for the Java yahcli DevOps
// tool for Solo use cases (see docs/beacon/design.md).
//
// This file is the binary skeleton: the cobra root command and the global flag
// surface (with BEACON_ env binding). The operations themselves live in the
// internal/beacon/{config,keystore,client,ops,commands} packages filled in by
// later stories.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	"github.com/automa-saga/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// envPrefix namespaces beacon's environment variables, e.g. --network maps to
// BEACON_NETWORK and --log-level to BEACON_LOG_LEVEL. This mirrors the uc
// sidecar's UC_ convention and lets every global flag be set via the
// environment in addition to the command line.
const envPrefix = "BEACON_"

// reasonInvalidEnv marks a global environment variable that failed to parse
// into its target flag (surfaced with a remediation hint via errx).
const reasonInvalidEnv errx.Reason = "InvalidEnv"

// globalOptions holds the values of the persistent flags shared by every
// beacon subcommand. Resolving these into a concrete node/payer is the job of
// internal/beacon/config (a later story); here they are only captured.
type globalOptions struct {
	Payer       string // -p/--payer: payer account (bare num or S.R.N literal)
	NodeAccount string // -a/--node-account: override target node's fee-collection account
	NodeIP      string // -i/--node-ip: target a node by IP
	Config      string // -c/--config: path to the network's config.yaml
	LogLevel    string // -v/--log-level: warn | info | debug
}

var (
	opts        globalOptions
	flagVersion bool
)

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		// Render the error with its remediation hints (errx.Format adds the
		// "Resolution:" block) instead of cobra's bare one-liner, matching the
		// uc sidecar's top-level handler.
		fmt.Fprintln(os.Stderr, errx.Format(err))
		os.Exit(1)
	}
}

// newRootCmd builds the beacon root command: the global flag surface, env
// binding, and logging initialization. Bare `beacon` prints usage.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "beacon",
		Short: "Solo HAPI control-plane CLI for driving Hedera network operations",
		Long: "beacon submits privileged HAPI transactions to a running Hedera consensus " +
			"network over gRPC. It drives the consensus-node software-upgrade flow " +
			"(special-file upload, prepare-upgrade, freeze-upgrade, freeze) and the " +
			"supporting operations around it, as the Go replacement for yahcli in Solo. " +
			"It is a protocol-layer tool only: it never touches kubeconfig, Helm, or pods.",
		// bindEnv applies BEACON_-prefixed env vars, then logging is initialized
		// from the resolved --log-level, so every subcommand shares one logger.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := bindEnv(cmd); err != nil {
				return err
			}
			return initLogging(opts.LogLevel)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagVersion {
				return printVersion(cmd, version.FormatText)
			}
			// Bare `beacon` (no subcommand) prints usage rather than acting.
			return cmd.Help()
		},
	}

	// We render errors ourselves via errx.Format, so keep cobra from printing
	// the bare error and the usage block on a runtime failure.
	root.SilenceErrors = true
	root.SilenceUsage = true

	f := root.PersistentFlags()
	f.StringVarP(&opts.Payer, "payer", "p", "", "Payer account (bare number or S.R.N literal, e.g. 2 or 0.0.2)")
	f.StringVarP(&opts.NodeAccount, "node-account", "a", "", "Override the target node's fee-collection account")
	f.StringVarP(&opts.NodeIP, "node-ip", "i", "", "Target a node by IP")
	f.StringVarP(&opts.Config, "config", "c", "config.yaml", "Path to the network's config.yaml; its directory holds keys/ and sysfiles/")
	f.StringVarP(&opts.LogLevel, "log-level", "v", "info", "Log level: warn, info, or debug")

	// --version is a plain root flag (no short: -v is --log-level); the `version`
	// subcommand offers the same output with a text/json selector.
	root.Flags().BoolVar(&flagVersion, "version", false, "Print version and exit")

	root.AddCommand(versionCmd())

	return root
}

// bindEnv applies BEACON_-prefixed environment variables to any flag the user
// did not pass explicitly on the command line. Precedence is: CLI flag > env
// var > default. The env var name is the flag name uppercased with '-' replaced
// by '_' and the BEACON_ prefix, e.g. --network -> BEACON_NETWORK, --log-level
// -> BEACON_LOG_LEVEL.
//
// Wired via PersistentPreRunE, it visits only the executing command's own flag
// set (which includes inherited persistent flags), so subcommands honour the
// same env vars.
func bindEnv(cmd *cobra.Command) error {
	var bindErr error
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if bindErr != nil || f.Changed {
			return
		}
		env := envPrefix + strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")
		v, ok := os.LookupEnv(env)
		if !ok {
			return
		}
		if err := f.Value.Set(v); err != nil {
			bindErr = errx.Decorate(
				fmt.Errorf("invalid value %q for %s: %w", v, env, err),
				reasonInvalidEnv,
				fmt.Sprintf("Set %s to a value valid for the --%s flag, or pass --%s explicitly", env, f.Name, f.Name),
			)
		}
	})
	return bindErr
}

// initLogging configures the global logx logger from the resolved log level.
func initLogging(level string) error {
	return logx.Initialize(logx.LoggingConfig{
		Level:          level,
		ConsoleLogging: true,
	})
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
			return printVersion(cmd, format)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", version.FormatText, "Output format: text or json")
	return cmd
}

// printVersion renders the build metadata in the requested format to the
// command's output stream.
func printVersion(cmd *cobra.Command, format string) error {
	out, err := version.Get().Format(format)
	if err != nil {
		return err
	}
	cmd.Println(out)
	return nil
}
