// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ValidateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("throttles.json content must not be empty")
	}
	var v interface{}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
