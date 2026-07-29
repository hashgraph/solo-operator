// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"strings"

	"github.com/automa-saga/errx"
)

// ValidateJavaProperties validates content as a Java key=value properties file.
// filename is used only in error messages.
func ValidateJavaProperties(content, filename string) error {
	if strings.TrimSpace(content) == "" {
		return errx.WithHints(
			errx.WithReason(errValidation.New("%s content must not be empty", filename), ReasonContentEmpty),
			"Populate the ConfigMap key with a non-empty properties document",
		)
	}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		if strings.Index(trimmed, "=") < 1 {
			return errx.WithHints(
				errx.WithReason(errValidation.New("line %d is not in key=value format: %q", i+1, trimmed), ReasonNotKeyValue),
				"Each non-comment line must be key=value (comments start with # or !)",
			)
		}
	}
	return nil
}

// ValidateJSON validates content as well-formed JSON. filename is used only in error messages.
func ValidateJSON(content, filename string) error {
	if strings.TrimSpace(content) == "" {
		return errx.WithHints(
			errx.WithReason(errValidation.New("%s content must not be empty", filename), ReasonContentEmpty),
			"Populate the ConfigMap key with a non-empty JSON document",
		)
	}
	var v interface{}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return errx.WithHints(
			errx.WithReason(errValidation.Wrap(err, "invalid JSON in %s", filename), ReasonInvalidJSON),
			"Fix the JSON syntax so the document parses",
		)
	}
	return nil
}
