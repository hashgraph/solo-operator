// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// PemToDER decodes a PEM-encoded certificate to its raw DER bytes.
// Returns an error if no PEM block is found or the bytes are not a valid X.509 certificate.
func PemToDER(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("invalid X.509 certificate: %w", err)
	}
	return block.Bytes, nil
}
