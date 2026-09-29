// SPDX-License-Identifier: Apache-2.0

package keystore

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
	"github.com/rs/zerolog"
	"github.com/youmark/pkcs8"
)

// secp256k1OID identifies the curve in a SEC1 "EC PRIVATE KEY" structure.
var secp256k1OID = asn1.ObjectIdentifier{1, 3, 132, 0, 10}

// ed25519PEM builds an account<N>.pem for an Ed25519 key. An empty pass yields an
// unencrypted PKCS#8 "PRIVATE KEY"; a non-empty pass yields an encrypted PKCS#8
// "ENCRYPTED PRIVATE KEY" — the two shapes the hiero SDK reads.
func ed25519PEM(t *testing.T, pass string) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	if pass == "" {
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			t.Fatalf("marshal pkcs8: %v", err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	}
	der, err := pkcs8.MarshalPrivateKey(priv, []byte(pass), nil)
	if err != nil {
		t.Fatalf("marshal encrypted pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der})
}

// ecdsaPEM builds an account<N>.pem for an ECDSA(secp256k1) key as a SEC1
// "EC PRIVATE KEY", optionally legacy-encrypted (DEK-Info) when pass is set —
// again matching what the hiero SDK reads.
func ecdsaPEM(t *testing.T, pass string) []byte {
	t.Helper()
	// Generate the secp256k1 key through the hiero SDK (rather than importing a
	// third-party curve lib directly) — BytesRaw() is the 32-byte scalar, the same
	// value btcec's Serialize() produced, which we wrap in a SEC1 structure below.
	priv, err := hiero.PrivateKeyGenerateEcdsa()
	if err != nil {
		t.Fatalf("generate ecdsa: %v", err)
	}
	ecKey := struct {
		Version       int
		PrivateKey    []byte
		NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
		PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
	}{Version: 1, PrivateKey: priv.BytesRaw(), NamedCurveOID: secp256k1OID}
	der, err := asn1.Marshal(ecKey)
	if err != nil {
		t.Fatalf("marshal sec1: %v", err)
	}
	if pass == "" {
		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	}
	//nolint:staticcheck // SDK reads legacy encrypted PEM; fixtures must match.
	blk, err := x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", der, []byte(pass), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("encrypt pem: %v", err)
	}
	return pem.EncodeToMemory(blk)
}

// writeKey materializes a key (and an optional sibling .pass) under dir.
func writeKey(t *testing.T, dir string, account uint64, pemBytes []byte, pass *string) {
	t.Helper()
	pemPath := filepath.Join(dir, fmt.Sprintf("account%d.pem", account))
	if err := os.WriteFile(pemPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	if pass != nil {
		passPath := filepath.Join(dir, fmt.Sprintf("account%d.pass", account))
		// A trailing newline is added on purpose — Load must trim it.
		if err := os.WriteFile(passPath, []byte(*pass+"\n"), 0o600); err != nil {
			t.Fatalf("write pass: %v", err)
		}
	}
}

// wantReason asserts err carries the given errx reason code.
func wantReason(t *testing.T, err error, reason errx.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	got, ok := errx.ReasonOf(err)
	if !ok || got != reason {
		t.Errorf("reason = %q (ok=%v), want %q", got, ok, reason)
	}
}

func TestLoad_BothKeyTypesWithPass(t *testing.T) {
	dir := t.TempDir()
	pass := "s3cret"
	writeKey(t, dir, 2, ed25519PEM(t, pass), &pass) // an ed25519 signing key
	writeKey(t, dir, 58, ecdsaPEM(t, pass), &pass)  // a secp256k1 signing key

	ks := New(dir)
	// A .pass is present for both, so the prompt must never fire.
	ks.Prompt = func(string) (string, error) {
		t.Fatal("prompt called despite a .pass file being present")
		return "", nil
	}

	k2, err := ks.Load(2)
	if err != nil {
		t.Fatalf("load ed25519: %v", err)
	}
	if got := Algorithm(k2); got != "Ed25519" {
		t.Errorf("account2 algorithm = %q, want Ed25519", got)
	}

	k58, err := ks.Load(58)
	if err != nil {
		t.Fatalf("load ecdsa: %v", err)
	}
	if got := Algorithm(k58); got != "ECDSA_secp256k1" {
		t.Errorf("account58 algorithm = %q, want ECDSA_secp256k1", got)
	}
}

func TestLoad_PromptsWhenNoPass(t *testing.T) {
	dir := t.TempDir()
	writeKey(t, dir, 2, ed25519PEM(t, "hunter2"), nil) // encrypted, no .pass

	called := false
	ks := New(dir)
	ks.Prompt = func(prompt string) (string, error) {
		called = true
		if !strings.Contains(prompt, "account2.pem") {
			t.Errorf("prompt text = %q, want it to name account2.pem", prompt)
		}
		return "hunter2", nil
	}

	if _, err := ks.Load(2); err != nil {
		t.Fatalf("load with prompt: %v", err)
	}
	if !called {
		t.Error("prompt was not called when .pass was absent")
	}
}

func TestLoad_UnencryptedViaEmptyPrompt(t *testing.T) {
	dir := t.TempDir()
	writeKey(t, dir, 2, ed25519PEM(t, ""), nil) // unencrypted, no .pass

	ks := New(dir)
	ks.Prompt = func(string) (string, error) { return "", nil }

	if _, err := ks.Load(2); err != nil {
		t.Fatalf("load unencrypted: %v", err)
	}
}

func TestLoad_WrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	writeKey(t, dir, 2, ed25519PEM(t, "right"), nil)

	ks := New(dir)
	ks.Prompt = func(string) (string, error) { return "wrong", nil }

	_, err := ks.Load(2)
	wantReason(t, err, reasonKeyDecodeFailed)
}

func TestLoad_MissingPEM(t *testing.T) {
	ks := New(t.TempDir())
	_, err := ks.Load(2)
	wantReason(t, err, reasonKeyReadFailed)
}

func TestLoad_NoPassphraseSource(t *testing.T) {
	dir := t.TempDir()
	writeKey(t, dir, 2, ed25519PEM(t, "x"), nil)

	// No .pass and no prompt (e.g. non-interactive) must be a typed error, not a
	// hang or a silent empty passphrase.
	ks := &Keystore{Dir: dir, Prompt: nil}
	_, err := ks.Load(2)
	wantReason(t, err, reasonNoPassphraseSource)
}

func TestLoad_NeverLogsKeyMaterial(t *testing.T) {
	// Capture logx output and prove the private key never appears in it.
	var buf bytes.Buffer
	restore := logx.As()
	restoreLevel := zerolog.GlobalLevel()
	logx.SetLogger(zerolog.New(&buf).Level(zerolog.DebugLevel))
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() {
		logx.SetLogger(*restore)
		zerolog.SetGlobalLevel(restoreLevel)
	})

	dir := t.TempDir()
	pass := "s3cret"
	writeKey(t, dir, 2, ed25519PEM(t, pass), &pass)

	ks := New(dir)
	key, err := ks.Load(2)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Loaded signing key") || !strings.Contains(out, "public_key") {
		t.Errorf("expected a debug line with the public key, got: %s", out)
	}
	for _, secret := range []string{key.String(), key.StringRaw(), pass} {
		if secret != "" && strings.Contains(out, secret) {
			t.Errorf("log leaked key material or passphrase: %s", out)
		}
	}
}
