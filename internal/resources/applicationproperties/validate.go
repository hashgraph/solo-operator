// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"math/big"
	"strings"

	"github.com/hashgraph/solo-operator/internal/common"
)

// ParseLedgerID extracts the value of the ledger.id key from Java-properties content.
// Returns ("", nil) when the key is absent.
func ParseLedgerID(content string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		if key == "ledger.id" {
			return strings.TrimSpace(trimmed[idx+1:]), nil
		}
	}
	return "", nil
}

func ValidateContent(content string) error {
	return common.ValidateJavaProperties(content, "application.properties")
}

// canonicalLedgerID normalises a ledger id so equivalent representations compare
// equal. A ledger id is a hex value, so it parses the hex (after an optional "0x"
// prefix) to a number and returns its canonical decimal string: "0x01", "0x1",
// "01", and "1" all canonicalise to "1"; "0x00"/"0"/"" to "0". A big.Int is used
// so an unusually long value can't overflow. If the value isn't valid hex it
// falls back to the trimmed lowercased string, so a malformed id still compares
// by exact equality rather than silently matching.
//
// Both sides compared are ledger ids (application.properties ledger.id and the
// Orbit's addressBook.ledgerId) — the EVM chain id lives in its own field, so no
// hex⇔decimal chain-id mapping is involved here.
func canonicalLedgerID(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return "0"
	}
	if n, ok := new(big.Int).SetString(s, 16); ok {
		return n.String()
	}
	return s
}

// LedgerIDsEqual reports whether two ledger ids denote the same ledger,
// tolerating hex-representation differences (0x prefix, leading zeros, case).
func LedgerIDsEqual(a, b string) bool {
	return canonicalLedgerID(a) == canonicalLedgerID(b)
}
