// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"fmt"
	"strings"
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
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("application.properties content must not be empty")
	}

	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 1 {
			return fmt.Errorf("line %d is not in key=value format: %q", i+1, trimmed)
		}
	}

	return nil
}
