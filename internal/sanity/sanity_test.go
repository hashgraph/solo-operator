// SPDX-License-Identifier: Apache-2.0

package sanity

import (
	"strings"
	"testing"
)

func TestValidateOperationID(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		shouldErr bool
		errMsg    string
	}{
		// Valid operationIds.
		{name: "simple", input: "op123", shouldErr: false},
		{name: "hyphenated", input: "op-2026-06-16-abc123", shouldErr: false},
		{name: "underscore", input: "OP_1", shouldErr: false},
		{name: "single char", input: "a", shouldErr: false},
		{name: "upgrade convention", input: "upgrade-20060102T150405Z", shouldErr: false},
		{name: "migration convention", input: "migration-20060102T150405Z", shouldErr: false},
		{name: "dotted version string", input: "upgrade-v0.76.0-20060102T150405Z", shouldErr: false},
		{name: "single dot", input: "op.123", shouldErr: false},

		// Invalid - empty.
		{name: "empty", input: "", shouldErr: true, errMsg: "cannot be empty"},

		// Invalid - path traversal.
		{name: "dotdot traversal", input: "../evil", shouldErr: true, errMsg: "path traversal"},
		{name: "embedded dotdot", input: "a..b", shouldErr: true, errMsg: "path traversal"},
		{name: "forward slash", input: "a/b", shouldErr: true, errMsg: "invalid characters"},
		{name: "backslash", input: "a\\b", shouldErr: true, errMsg: "invalid characters"},

		// Invalid - shell metacharacters.
		{name: "semicolon", input: "op;rm", shouldErr: true, errMsg: "shell metacharacters"},
		{name: "dollar", input: "op$x", shouldErr: true, errMsg: "shell metacharacters"},
		{name: "backtick", input: "op`x`", shouldErr: true, errMsg: "shell metacharacters"},
		{name: "pipe", input: "op|x", shouldErr: true, errMsg: "shell metacharacters"},

		// Invalid - whitespace and control bytes.
		{name: "space", input: "op id", shouldErr: true, errMsg: "invalid characters"},
		{name: "null byte", input: "op\x00x", shouldErr: true, errMsg: "invalid characters"},
		{name: "newline", input: "op\nx", shouldErr: true, errMsg: "invalid characters"},
		{name: "tab", input: "op\tx", shouldErr: true, errMsg: "invalid characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOperationID(tc.input)
			if tc.shouldErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.input)
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
		})
	}
}
