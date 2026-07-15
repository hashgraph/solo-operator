// SPDX-License-Identifier: Apache-2.0

package nodesettings

import (
	"strings"

	"github.com/hashgraph/solo-operator/internal/errx"
)

func ValidateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return errx.WithHints(
			errx.WithReason(errValidation.New("settings.txt content must not be empty"), ReasonContentEmpty),
			"Populate the ConfigMap key with a non-empty settings.txt document",
		)
	}

	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, ",")
		if idx < 1 {
			return errx.WithHints(
				errx.WithReason(errValidation.New("line %d is not in key,value format: %q", i+1, trimmed), ReasonNotKeyValue),
				"Each non-comment line must be key,value (comments start with #)",
			)
		}
	}

	return nil
}
