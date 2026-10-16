// SPDX-License-Identifier: Apache-2.0

package blocknodesconfig

import (
	"strings"
	"testing"
)

func TestValidateTLS(t *testing.T) {
	hash := strings.Repeat("ab", 48)
	colonHash := strings.TrimSuffix(strings.Repeat("AB:", 48), ":")

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"empty nodes", `{"nodes": []}`, ""},
		{"no tls blocks", `{"nodes":[{"address":"bn","streamingPort":40840,"priority":0}]}`, ""},
		{"legacy shape passes through", `{"nodes":[{"address":"bn","port":8080}],"blockItemBatchSize":256}`, ""},
		{"non-object document passes through", `[1,2]`, ""},
		{"null tls", `{"nodes":[{"address":"bn","streamingTls":null}]}`, ""},
		{"tls enabled without hash", `{"nodes":[{"address":"bn","streamingTls":{"enabled":true}}]}`, ""},
		{"tls disabled without hash", `{"nodes":[{"address":"bn","serviceTls":{"enabled":false}}]}`, ""},
		{"tls enabled with hash", `{"nodes":[{"address":"bn","streamingTls":{"enabled":true,"certificateSha384":"` + hash + `"}}]}`, ""},
		{"tls enabled with colon hash", `{"nodes":[{"address":"bn","serviceTls":{"enabled":true,"certificateSha384":"` + colonHash + `"}}]}`, ""},
		{"hash without enabled", `{"nodes":[{"address":"bn","streamingTls":{"certificateSha384":"` + hash + `"}}]}`, "must not be set unless enabled"},
		{"hash with enabled false", `{"nodes":[{"address":"bn","serviceTls":{"enabled":false,"certificateSha384":"` + hash + `"}}]}`, "must not be set unless enabled"},
		{"short hash", `{"nodes":[{"address":"bn","streamingTls":{"enabled":true,"certificateSha384":"abcd"}}]}`, "96 hex characters"},
		{"non-hex hash", `{"nodes":[{"address":"bn","streamingTls":{"enabled":true,"certificateSha384":"` + strings.Repeat("zz", 48) + `"}}]}`, "96 hex characters"},
		{"tls not an object", `{"nodes":[{"address":"bn","streamingTls":true}]}`, "must be an object"},
		{"enabled not a bool", `{"nodes":[{"address":"bn","serviceTls":{"enabled":"yes"}}]}`, "must be an object"},
		{"error names the node", `{"nodes":[{"address":"ok"},{"address":"bad-bn","serviceTls":{"enabled":true,"certificateSha384":"x"}}]}`, "nodes[1] (bad-bn).serviceTls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTLS(tt.content)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
