// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
)

func selfSignedCert(t *testing.T) (pemBytes, derBytes []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	derBytes, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	pemBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return pemBytes, derBytes
}

func TestPemToDER_Valid(t *testing.T) {
	pemBytes, derExpected := selfSignedCert(t)
	derGot, err := PemToDER(pemBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(derGot) != string(derExpected) {
		t.Error("DER bytes do not match original")
	}
}

func TestPemToDER_NoPEMBlock(t *testing.T) {
	_, err := PemToDER([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for non-PEM input")
	}
}

func TestPemToDER_InvalidCert(t *testing.T) {
	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	_, err := PemToDER(garbage)
	if err == nil {
		t.Error("expected error for invalid X.509 bytes")
	}
}

func TestExtractPubKeyHex_Valid(t *testing.T) {
	pemBytes, _ := selfSignedCert(t)
	got, err := ExtractPubKeyHex(pemBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty hex string")
	}
}

func TestExtractPubKeyHex_NoPEMBlock(t *testing.T) {
	_, err := ExtractPubKeyHex([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for non-PEM input")
	}
}

func TestCertHashHex_Valid(t *testing.T) {
	pemBytes, derBytes := selfSignedCert(t)
	got, err := CertHashHex(pemBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sum := sha512.Sum384(pemBytes)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Errorf("CertHashHex mismatch: got %s, want %s", got, want)
	}
	derSum := sha512.Sum384(derBytes)
	if got == hex.EncodeToString(derSum[:]) {
		t.Error("CertHashHex must hash the PEM form, not the DER bytes")
	}
}

// The hash must not depend on how the input PEM is formatted.
func TestCertHashHex_CanonicalPEM(t *testing.T) {
	pemBytes, _ := selfSignedCert(t)
	want, err := CertHashHex(pemBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	variants := map[string][]byte{
		"CRLF":           []byte(strings.ReplaceAll(string(pemBytes), "\n", "\r\n")),
		"leading text":   append([]byte("subject=CN=test\n"), pemBytes...),
		"trailing space": append(append([]byte{}, pemBytes...), []byte("\n\n")...),
	}
	for name, in := range variants {
		got, err := CertHashHex(in)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
}

func TestCertHashHex_NoPEMBlock(t *testing.T) {
	_, err := CertHashHex([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for non-PEM input")
	}
}

// A PEM block that is not labelled CERTIFICATE (e.g. a private key) must be
// rejected rather than relabelled and hashed.
func TestCertHashHex_WrongPEMType(t *testing.T) {
	_, derBytes := selfSignedCert(t)
	for _, typ := range []string{"PRIVATE KEY", "PUBLIC KEY", "RSA PRIVATE KEY"} {
		in := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: derBytes})
		_, err := CertHashHex(in)
		if err == nil {
			t.Fatalf("%s: expected error for non-CERTIFICATE PEM block", typ)
		}
		if r, ok := errx.ReasonOf(err); !ok || r != ReasonNotCertPEM {
			t.Errorf("%s: reason = %q, want %q", typ, r, ReasonNotCertPEM)
		}
	}
}

// A CERTIFICATE-labelled block whose bytes are not a valid X.509 certificate must be rejected.
func TestCertHashHex_InvalidCert(t *testing.T) {
	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	_, err := CertHashHex(garbage)
	if err == nil {
		t.Fatal("expected error for invalid X.509 bytes")
	}
	if r, ok := errx.ReasonOf(err); !ok || r != ReasonInvalidCert {
		t.Errorf("reason = %q, want %q", r, ReasonInvalidCert)
	}
}

func TestPemToDER_WrongPEMType(t *testing.T) {
	_, derBytes := selfSignedCert(t)
	in := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBytes})
	if _, err := PemToDER(in); err == nil {
		t.Error("expected error for non-CERTIFICATE PEM block")
	}
}
