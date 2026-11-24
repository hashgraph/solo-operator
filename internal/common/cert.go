// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"

	"github.com/automa-saga/errx"
)

// pemTypeCertificate is the PEM block type of an X.509 certificate.
const pemTypeCertificate = "CERTIFICATE"

// parseCertPEM decodes the first PEM block and parses it as an X.509 certificate.
// It rejects input with no PEM block, a block whose type is not CERTIFICATE, and
// bytes that are not a valid X.509 certificate.
func parseCertPEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errx.WithHints(
			errx.WithReason(errCertInvalid.New("no PEM block found"), ReasonNoPEMBlock),
			"Confirm the secret holds a PEM-encoded certificate (-----BEGIN CERTIFICATE-----)",
		)
	}
	if block.Type != pemTypeCertificate {
		return nil, errx.WithHints(
			errx.WithReason(
				errCertInvalid.New("PEM block type is %q, expected %q", block.Type, pemTypeCertificate),
				ReasonNotCertPEM,
			),
			"Confirm the secret holds the certificate (-----BEGIN CERTIFICATE-----), not a key or other PEM object",
		)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errCertInvalid.Wrap(err, "invalid X.509 certificate"), ReasonInvalidCert),
			"Confirm the PEM block contains a valid X.509 certificate",
		)
	}
	return cert, nil
}

// PemToDER decodes a PEM-encoded certificate to its raw DER bytes.
// Returns an error if no CERTIFICATE PEM block is found or the bytes are not a valid X.509 certificate.
func PemToDER(pemBytes []byte) ([]byte, error) {
	cert, err := parseCertPEM(pemBytes)
	if err != nil {
		return nil, err
	}
	return cert.Raw, nil
}

// ExtractPubKeyHex parses a PEM certificate and returns the hex-encoded SubjectPublicKeyInfo bytes.
func ExtractPubKeyHex(pemBytes []byte) (string, error) {
	cert, err := parseCertPEM(pemBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(cert.RawSubjectPublicKeyInfo), nil
}

// CertHashHex returns the hex-encoded SHA-384 hash of the canonical PEM form of a certificate,
// the form that HAPI grpc_certificate_hash and the SDKs use.
// Returns an error if no CERTIFICATE PEM block is found or the bytes are not a valid X.509 certificate.
func CertHashHex(pemBytes []byte) (string, error) {
	cert, err := parseCertPEM(pemBytes)
	if err != nil {
		return "", err
	}
	sum := sha512.Sum384(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: cert.Raw}))
	return hex.EncodeToString(sum[:]), nil
}
