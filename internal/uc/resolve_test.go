// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// ---------------------------------------------------------------------------
// parseVersionFile
// ---------------------------------------------------------------------------

func TestParseVersionFileAbsent(t *testing.T) {
	root := t.TempDir()
	ver, mtime, found, err := parseVersionFile(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatalf("expected found=false for absent file, got true")
	}
	if ver != "" {
		t.Fatalf("expected empty version, got %q", ver)
	}
	if !mtime.IsZero() {
		t.Fatalf("expected zero mtime, got %v", mtime)
	}
}

func TestParseVersionFilePresent(t *testing.T) {
	root := t.TempDir()
	content := "VERSION=0.74.0\nCOMMIT=abc123\nDATE=2026-05-21\n"
	path := filepath.Join(root, versionFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	pinned := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, pinned, pinned); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	ver, mtime, found, err := parseVersionFile(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if ver != "0.74.0" {
		t.Fatalf("version = %q, want 0.74.0", ver)
	}
	if !mtime.Equal(pinned) {
		t.Fatalf("mtime = %v, want %v", mtime, pinned)
	}
}

func TestParseVersionFileNoVersionLine(t *testing.T) {
	root := t.TempDir()
	content := "COMMIT=abc123\nDATE=2026-05-21\n"
	if err := os.WriteFile(filepath.Join(root, versionFile), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ver, mtime, found, err := parseVersionFile(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatalf("expected found=false when no VERSION= line")
	}
	if ver != "" {
		t.Fatalf("expected empty version, got %q", ver)
	}
	if !mtime.IsZero() {
		t.Fatalf("expected zero mtime, got %v", mtime)
	}
}

func TestParseVersionFileIOError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, versionFile)
	// Create a directory where the file would be — os.ReadFile will fail.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, _, _, err := parseVersionFile(root)
	if err == nil {
		t.Fatal("expected error for unreadable file (directory), got nil")
	}
}

// ---------------------------------------------------------------------------
// versionFromOperationID
// ---------------------------------------------------------------------------

func TestVersionFromOperationIDStable(t *testing.T) {
	ver, ok := versionFromOperationID("upgrade-0.74.0-20260521T182531Z")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ver != "0.74.0" {
		t.Fatalf("version = %q, want 0.74.0", ver)
	}
}

func TestVersionFromOperationIDWithDashInVersion(t *testing.T) {
	ver, ok := versionFromOperationID("upgrade-0.74.0-rc1-20260521T182531Z")
	if !ok {
		t.Fatal("expected ok=true for version with dash")
	}
	if ver != "0.74.0-rc1" {
		t.Fatalf("version = %q, want 0.74.0-rc1", ver)
	}
}

func TestVersionFromOperationIDUnrecognised(t *testing.T) {
	ver, ok := versionFromOperationID("not-an-opid")
	if ok {
		t.Fatalf("expected ok=false for unrecognised format, got version %q", ver)
	}
}

func TestVersionFromOperationIDEmpty(t *testing.T) {
	ver, ok := versionFromOperationID("")
	if ok {
		t.Fatalf("expected ok=false for empty string, got version %q", ver)
	}
}

// ---------------------------------------------------------------------------
// deriveOperationIDPrimary — VERSION cross-check
// ---------------------------------------------------------------------------

func TestDeriveOperationIDPrimaryNoVersionFile(t *testing.T) {
	// execute_immediate.mf present, VERSION absent → no cross-check, succeeds.
	root := writePackage(t, validComponentsYAML)

	opID, _, _, found, err := deriveOperationIDPrimary(root, wantConsensusNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if opID != wantOperationID {
		t.Fatalf("operationID = %q, want %q", opID, wantOperationID)
	}
}

func TestDeriveOperationIDPrimaryVersionMatches(t *testing.T) {
	// VERSION matches manifest version (0.75.0) → succeeds.
	root := writePackage(t, validComponentsYAML)
	vfPath := filepath.Join(root, versionFile)
	if err := os.WriteFile(vfPath, []byte("VERSION=0.75.0\nCOMMIT=abc\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	opID, _, _, found, err := deriveOperationIDPrimary(root, wantConsensusNode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if opID != wantOperationID {
		t.Fatalf("operationID = %q, want %q", opID, wantOperationID)
	}
}

func TestDeriveOperationIDPrimaryVersionMismatches(t *testing.T) {
	// VERSION reports different version than manifest → ManifestInvalid error.
	root := writePackage(t, validComponentsYAML)
	vfPath := filepath.Join(root, versionFile)
	if err := os.WriteFile(vfPath, []byte("VERSION=0.99.0\nCOMMIT=xyz\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	_, _, _, found, err := deriveOperationIDPrimary(root, wantConsensusNode)
	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}
	if found {
		t.Fatal("expected found=false on mismatch")
	}
	if !errorx.IsOfType(err, errManifestInvalid) {
		t.Fatalf("expected errManifestInvalid, got %T: %v", err, err)
	}
	if reason, ok := errx.ReasonOf(err); !ok || reason != ReasonManifestInvalid {
		t.Fatalf("reason = %q (ok=%v), want %q", reason, ok, ReasonManifestInvalid)
	}
}
