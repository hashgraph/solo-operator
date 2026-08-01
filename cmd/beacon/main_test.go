// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/automa-saga/errx"
	"github.com/spf13/cobra"
)

// resetOpts restores the package-global flag state between subtests, since the
// root command binds its flags into the shared opts struct.
func resetOpts(t *testing.T) {
	t.Helper()
	opts = globalOptions{}
	flagVersion = false
}

// parsedRoot returns a root command whose persistent flags have been merged
// into its full flag set, mirroring the state cobra guarantees by the time
// PersistentPreRunE (and thus bindEnv) runs during a real invocation.
func parsedRoot(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	root := newRootCmd()
	if err := root.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v): %v", args, err)
	}
	return root
}

func TestRootCmd_GlobalFlags(t *testing.T) {
	resetOpts(t)
	root := newRootCmd()

	// Every documented global flag must be present with the expected shorthand.
	cases := []struct {
		name  string
		short string
	}{
		{"network", "n"},
		{"payer", "p"},
		{"node-account", "a"},
		{"node-ip", "i"},
		{"config", "c"},
		{"working-dir", "w"},
		{"log-level", "v"},
	}
	for _, c := range cases {
		f := root.PersistentFlags().Lookup(c.name)
		if f == nil {
			t.Errorf("global flag --%s not registered", c.name)
			continue
		}
		if f.Shorthand != c.short {
			t.Errorf("flag --%s: shorthand = %q, want %q", c.name, f.Shorthand, c.short)
		}
	}

	// Defaults from the design (§6.1).
	if got := root.PersistentFlags().Lookup("config").DefValue; got != "config.yaml" {
		t.Errorf("--config default = %q, want %q", got, "config.yaml")
	}
	if got := root.PersistentFlags().Lookup("working-dir").DefValue; got != "." {
		t.Errorf("--working-dir default = %q, want %q", got, ".")
	}
	if got := root.PersistentFlags().Lookup("log-level").DefValue; got != "info" {
		t.Errorf("--log-level default = %q, want %q", got, "info")
	}

	// --version is a root flag with no shorthand (-v is --log-level).
	if root.Flags().Lookup("version") == nil {
		t.Error("--version flag not registered")
	}
}

func TestBindEnv_OverridesDefault(t *testing.T) {
	resetOpts(t)
	t.Setenv("BEACON_NETWORK", "mainnet")
	t.Setenv("BEACON_LOG_LEVEL", "debug")
	root := parsedRoot(t)

	if err := bindEnv(root); err != nil {
		t.Fatalf("bindEnv returned error: %v", err)
	}
	if opts.Network != "mainnet" {
		t.Errorf("BEACON_NETWORK not applied: Network = %q, want %q", opts.Network, "mainnet")
	}
	if opts.LogLevel != "debug" {
		t.Errorf("BEACON_LOG_LEVEL not applied: LogLevel = %q, want %q", opts.LogLevel, "debug")
	}
}

func TestBindEnv_FlagWinsOverEnv(t *testing.T) {
	resetOpts(t)
	t.Setenv("BEACON_NETWORK", "mainnet")
	// An explicit CLI flag marks the flag as Changed, which bindEnv must honour.
	root := parsedRoot(t, "--network", "solo-local")

	if err := bindEnv(root); err != nil {
		t.Fatalf("bindEnv returned error: %v", err)
	}
	if opts.Network != "solo-local" {
		t.Errorf("explicit flag should win over env: Network = %q, want %q", opts.Network, "solo-local")
	}
}

func TestBindEnv_InvalidValueDecorated(t *testing.T) {
	resetOpts(t)
	// --version is a bool flag; a non-bool env value must fail to parse and be
	// decorated with a reason + remediation hint.
	t.Setenv("BEACON_VERSION", "not-a-bool")
	root := parsedRoot(t)

	err := bindEnv(root)
	if err == nil {
		t.Fatal("bindEnv accepted an invalid bool env value")
	}
	if r, ok := errx.ReasonOf(err); !ok || r != reasonInvalidEnv {
		t.Errorf("error reason = %q (ok=%v), want %q", r, ok, reasonInvalidEnv)
	}
	if hints, ok := errx.Hints(err); !ok || len(hints) == 0 {
		t.Error("expected a remediation hint on the invalid-env error")
	}
}
