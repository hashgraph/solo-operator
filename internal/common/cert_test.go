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
	"testing"
	"time"
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
	sum := sha512.Sum384(derBytes)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Errorf("CertHashHex mismatch: got %s, want %s", got, want)
	}
}

func TestCertHashHex_NoPEMBlock(t *testing.T) {
	_, err := CertHashHex([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for non-PEM input")
	}
}
