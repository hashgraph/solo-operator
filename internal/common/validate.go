// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateJavaProperties validates content as a Java key=value properties file.
// filename is used only in error messages.
func ValidateJavaProperties(content, filename string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%s content must not be empty", filename)
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

// ValidateJSON validates content as well-formed JSON. filename is used only in error messages.
func ValidateJSON(content, filename string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%s content must not be empty", filename)
	}
	var v interface{}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
