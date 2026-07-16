// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// PemToDER decodes a PEM-encoded certificate to its raw DER bytes.
// Returns an error if no PEM block is found or the bytes are not a valid X.509 certificate.
func PemToDER(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errx.WithHints(
			errx.WithReason(errCertInvalid.New("no PEM block found"), ReasonNoPEMBlock),
			"Confirm the secret holds a PEM-encoded certificate (-----BEGIN CERTIFICATE-----)",
		)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errCertInvalid.Wrap(err, "invalid X.509 certificate"), ReasonInvalidCert),
			"Confirm the PEM block contains a valid X.509 certificate",
		)
	}
	return block.Bytes, nil
}

// ExtractPubKeyHex parses a PEM certificate and returns the hex-encoded SubjectPublicKeyInfo bytes.
func ExtractPubKeyHex(pemBytes []byte) (string, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errx.WithHints(
			errx.WithReason(errCertInvalid.New("no PEM block found"), ReasonNoPEMBlock),
			"Confirm the secret holds a PEM-encoded certificate (-----BEGIN CERTIFICATE-----)",
		)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", errx.WithHints(
			errx.WithReason(errCertInvalid.Wrap(err, "parse cert"), ReasonInvalidCert),
			"Confirm the PEM block contains a valid X.509 certificate",
		)
	}
	return hex.EncodeToString(cert.RawSubjectPublicKeyInfo), nil
}

// CertHashHex returns the hex-encoded SHA-384 hash of the DER bytes of a PEM certificate.
func CertHashHex(pemBytes []byte) (string, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errx.WithHints(
			errx.WithReason(errCertInvalid.New("no PEM block found"), ReasonNoPEMBlock),
			"Confirm the secret holds a PEM-encoded certificate (-----BEGIN CERTIFICATE-----)",
		)
	}
	sum := sha512.Sum384(block.Bytes)
	return hex.EncodeToString(sum[:]), nil
}
