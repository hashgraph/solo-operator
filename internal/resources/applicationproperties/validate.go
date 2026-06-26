// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
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
