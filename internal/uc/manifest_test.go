// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/errx"
)

func TestParseInfraVersionsManifest(t *testing.T) {
	t.Run("file absent returns nil", func(t *testing.T) {
		m, err := parseInfraVersionsManifest(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m != nil {
			t.Fatal("expected nil manifest when file is absent")
		}
	})

	t.Run("valid manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, manifestsSubdir)
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := `
schemaVersion: 1
provisioner:
  cli:
    version: "0.42.0"
  daemon:
    version: "0.33.0"
`
		if err := os.WriteFile(filepath.Join(manifestDir, infraVersionsFile), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		m, err := parseInfraVersionsManifest(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m == nil {
			t.Fatal("expected non-nil manifest")
		}
		if m.Provisioner.Daemon.Version != "0.33.0" {
			t.Errorf("daemon version = %q, want %q", m.Provisioner.Daemon.Version, "0.33.0")
		}
		if m.Provisioner.CLI.Version != "0.42.0" {
			t.Errorf("cli version = %q, want %q", m.Provisioner.CLI.Version, "0.42.0")
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, manifestsSubdir)
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, infraVersionsFile), []byte(":::bad"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := parseInfraVersionsManifest(dir)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("symlink rejected", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, manifestsSubdir)
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(manifestDir, infraVersionsFile)); err != nil {
			t.Fatal(err)
		}

		_, err := parseInfraVersionsManifest(dir)
		if err == nil {
			t.Fatal("expected error for symlinked manifest, got nil")
		}
		if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
			t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
		}
	})
}

func TestDeclaredProvisionerVersions(t *testing.T) {
	t.Run("absent manifest returns nil", func(t *testing.T) {
		prov, err := declaredProvisionerVersions(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prov != nil {
			t.Errorf("expected nil, got %+v", prov)
		}
	})

	t.Run("reads versions from manifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, manifestsSubdir)
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := `provisioner:
  daemon:
    version: "0.33.0"
  cli:
    version: "0.42.0"
`
		if err := os.WriteFile(filepath.Join(manifestDir, infraVersionsFile), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		prov, err := declaredProvisionerVersions(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prov == nil {
			t.Fatal("expected non-nil")
		}
		if prov.DaemonVersion != "0.33.0" {
			t.Errorf("daemon = %q, want %q", prov.DaemonVersion, "0.33.0")
		}
		if prov.CLIVersion != "0.42.0" {
			t.Errorf("cli = %q, want %q", prov.CLIVersion, "0.42.0")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		dir := t.TempDir()
		manifestDir := filepath.Join(dir, manifestsSubdir)
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, infraVersionsFile), []byte(":::bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := declaredProvisionerVersions(dir)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}

func TestInfraVersionComparison(t *testing.T) {
	tests := []struct {
		name            string
		declaredDaemon  string
		declaredCLI     string
		runningDaemon   string
		runningCLI      string
		wantInfraAccess bool
	}{
		{
			name:            "versions match — no infra access",
			declaredDaemon:  "0.33.0",
			declaredCLI:     "0.42.0",
			runningDaemon:   "0.33.0",
			runningCLI:      "0.42.0",
			wantInfraAccess: false,
		},
		{
			name:            "daemon version differs",
			declaredDaemon:  "0.34.0",
			declaredCLI:     "0.42.0",
			runningDaemon:   "0.33.0",
			runningCLI:      "0.42.0",
			wantInfraAccess: true,
		},
		{
			name:            "CLI version differs",
			declaredDaemon:  "0.33.0",
			declaredCLI:     "0.43.0",
			runningDaemon:   "0.33.0",
			runningCLI:      "0.42.0",
			wantInfraAccess: true,
		},
		{
			name:            "annotation missing — safe fallback to true",
			declaredDaemon:  "0.33.0",
			declaredCLI:     "0.42.0",
			runningDaemon:   "",
			runningCLI:      "",
			wantInfraAccess: true,
		},
		{
			name:            "both empty declared — no infra access",
			declaredDaemon:  "",
			declaredCLI:     "",
			runningDaemon:   "0.33.0",
			runningCLI:      "0.42.0",
			wantInfraAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			daemonChanged := tt.declaredDaemon != "" && tt.declaredDaemon != tt.runningDaemon
			cliChanged := tt.declaredCLI != "" && tt.declaredCLI != tt.runningCLI

			got := daemonChanged || cliChanged
			if got != tt.wantInfraAccess {
				t.Errorf("infraAccess = %v, want %v", got, tt.wantInfraAccess)
			}
		})
	}
}
