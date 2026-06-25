// SPDX-License-Identifier: Apache-2.0

package bootstrapproperties

import (
	"fmt"
	"strings"
)

func ValidateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("bootstrap.properties content must not be empty")
	}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		if strings.Index(trimmed, "=") < 1 {
			return fmt.Errorf("line %d is not in key=value format: %q", i+1, trimmed)
		}
	}
	return nil
}
