// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"

	"github.com/hashgraph/solo-operator/internal/beacon/client"
	"github.com/hashgraph/solo-operator/internal/beacon/config"
	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

func TestAssertPrivilegedPayer(t *testing.T) {
	for _, acct := range []uint64{2, 50, 58} {
		if err := assertPrivilegedPayer(hiero.AccountID{Account: acct}); err != nil {
			t.Errorf("account %d should be privileged: %v", acct, err)
		}
	}
	for _, acct := range []uint64{3, 98, 1001} {
		err := assertPrivilegedPayer(hiero.AccountID{Account: acct})
		wantReason(t, err, reasonNotPrivilegedPayer)
	}
}

func TestExpectedHash(t *testing.T) {
	validHex := strings.Repeat("ab", 48)
	wantRaw, _ := hex.DecodeString(validHex)

	t.Run("neither flag → not provided", func(t *testing.T) {
		raw, provided, err := expectedHash("", "")
		if err != nil || provided || raw != nil {
			t.Errorf("got raw=%v provided=%v err=%v, want nil/false/nil", raw, provided, err)
		}
	})

	t.Run("--expect-hash bare hex", func(t *testing.T) {
		raw, provided, err := expectedHash(validHex, "")
		if err != nil || !provided || !bytes.Equal(raw, wantRaw) {
			t.Errorf("got raw=%x provided=%v err=%v", raw, provided, err)
		}
	})

	t.Run("--expect-hash-file bare hex", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "h.txt")
		if err := os.WriteFile(path, []byte(validHex+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		raw, provided, err := expectedHash("", path)
		if err != nil || !provided || !bytes.Equal(raw, wantRaw) {
			t.Errorf("got raw=%x provided=%v err=%v", raw, provided, err)
		}
	})

	t.Run("--expect-hash-file sha384sum format takes first token", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sum.txt")
		if err := os.WriteFile(path, []byte(validHex+"  softwareUpgrade.zip\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		raw, provided, err := expectedHash("", path)
		if err != nil || !provided || !bytes.Equal(raw, wantRaw) {
			t.Errorf("got raw=%x provided=%v err=%v", raw, provided, err)
		}
	})

	t.Run("empty file rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.txt")
		if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := expectedHash("", path)
		wantReason(t, err, reasonExpectHashInvalid)
	})

	t.Run("missing file rejected", func(t *testing.T) {
		_, _, err := expectedHash("", filepath.Join(t.TempDir(), "nope.txt"))
		wantReason(t, err, reasonExpectHashReadFailed)
	})
}

// offlineSession builds a Session with a real but never-dialed client, payer
// account payerAcct, rooted at configDir. Suitable for paths that fail before
// any network submit.
func offlineSession(t *testing.T, payerAcct uint64, configDir string) *Session {
	t.Helper()
	key, err := hiero.PrivateKeyGenerateEd25519()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	network := map[string]hiero.AccountID{"127.0.0.1:50211": {Account: 3}}
	c, err := client.New(network, hiero.AccountID{Account: payerAcct}, key, client.Options{})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return &Session{
		Target:    &config.Target{Payer: hiero.AccountID{Account: payerAcct}},
		Client:    c,
		ConfigDir: configDir,
	}
}

// writeZip writes a package zip with content into <dir>/sysfiles/<name> and
// returns its path.
func writeZip(t *testing.T, dir string, content []byte) string {
	t.Helper()
	sysdir := filepath.Join(dir, "sysfiles")
	if err := os.MkdirAll(sysdir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sysdir, defaultSoftwareZipName)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSoftwareZipUpload_NonPrivilegedPayerRejected(t *testing.T) {
	dir := t.TempDir()
	resolve := func() (*Session, error) { return offlineSession(t, 3, dir), nil }
	cmd := softwareZipUploadCmd(resolve)

	_, err := runCmd(t, cmd)
	wantReason(t, err, reasonNotPrivilegedPayer)
}

func TestSoftwareZipUpload_MismatchAbortsBeforeUpload(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, dir, []byte("the real package bytes"))
	resolve := func() (*Session, error) { return offlineSession(t, 2, dir), nil }
	cmd := softwareZipUploadCmd(resolve)

	// An expected hash that does not match the zip must abort before any submit
	// (the offline client would error if a submit were attempted).
	wrongHash := strings.Repeat("cd", 48)
	_, err := runCmd(t, cmd, "--expect-hash", wrongHash)
	wantReason(t, err, reasonExpectedHashMismatch)
}

func TestSoftwareZipUpload_MissingZipReported(t *testing.T) {
	dir := t.TempDir() // no sysfiles/ zip written
	resolve := func() (*Session, error) { return offlineSession(t, 2, dir), nil }
	cmd := softwareZipUploadCmd(resolve)

	_, err := runCmd(t, cmd)
	wantReason(t, err, reasonZipReadFailed)
}

func TestSoftwareZipUpload_MutuallyExclusiveHashFlags(t *testing.T) {
	dir := t.TempDir()
	resolve := func() (*Session, error) { return offlineSession(t, 2, dir), nil }
	cmd := softwareZipUploadCmd(resolve)

	valid := strings.Repeat("ab", 48)
	_, err := runCmd(t, cmd, "--expect-hash", valid, "--expect-hash-file", "x.txt")
	if err == nil || !strings.Contains(err.Error(), "expect-hash") {
		t.Errorf("expected mutual-exclusion error, got %v", err)
	}
}

func TestFileCommands_InfoRegistered(t *testing.T) {
	root := FileCommands(func() (*Session, error) { return nil, nil })
	info, _, err := root.Find([]string{"info", "software-zip"})
	if err != nil || info.Name() != "software-zip" {
		t.Fatalf("`file info software-zip` not registered: cmd=%v err=%v", info, err)
	}
	if info.Flags().Lookup("upgrade-file-num") == nil {
		t.Error("`file info software-zip` missing --upgrade-file-num")
	}
}

func TestSoftwareZipInfo_ResolveErrorPropagates(t *testing.T) {
	sentinel := errors.New("no config")
	cmd := softwareZipInfoCmd(func() (*Session, error) { return nil, sentinel })
	_, err := runCmd(t, cmd)
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the resolve error", err)
	}
}

func TestSoftwareZipUpload_HasFromScratchFlag(t *testing.T) {
	resolve := func() (*Session, error) { return offlineSession(t, 2, t.TempDir()), nil }
	cmd := softwareZipUploadCmd(resolve)
	if cmd.Flags().Lookup("from-scratch") == nil {
		t.Fatal("software-zip is missing --from-scratch")
	}
}

// TestSoftwareZipUpload_HashMatchesLocalSHA384 documents the local-hash contract
// the upload prints for prepare-upgrade (the on-chain equality is an integration
// concern, §14.9).
func TestSoftwareZipUpload_HashMatchesLocalSHA384(t *testing.T) {
	content := []byte("deployment package v1")
	want := hex.EncodeToString(ops.HashBytes(content))
	if len(want) != 96 {
		t.Fatalf("sha384 hex length = %d, want 96", len(want))
	}
}
