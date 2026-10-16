// SPDX-License-Identifier: Apache-2.0

package blocknodesconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var sha384Hex = regexp.MustCompile(`^[0-9a-fA-F]{96}$`)

type blockNodesDoc struct {
	Nodes []blockNodeEntry `json:"nodes"`
}

type blockNodeEntry struct {
	Address      string          `json:"address"`
	StreamingTLS json.RawMessage `json:"streamingTls"`
	ServiceTLS   json.RawMessage `json:"serviceTls"`
}

type tlsBlock struct {
	Enabled           bool   `json:"enabled"`
	CertificateSha384 string `json:"certificateSha384"`
}

// validateTLS checks only streamingTls/serviceTls; malformed JSON or a non-array nodes passes through for the CN to reject.
func validateTLS(content string) error {
	var doc blockNodesDoc
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	for i, node := range doc.Nodes {
		blocks := []struct {
			field string
			raw   json.RawMessage
		}{{"streamingTls", node.StreamingTLS}, {"serviceTls", node.ServiceTLS}}
		for _, b := range blocks {
			if err := validateTLSBlock(b.raw); err != nil {
				return fmt.Errorf("nodes[%d] (%s).%s: %w", i, node.Address, b.field, err)
			}
		}
	}
	return nil
}

func validateTLSBlock(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var block tlsBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return fmt.Errorf("must be an object with a boolean enabled and a string certificateSha384")
	}
	if block.CertificateSha384 == "" {
		return nil
	}
	if !block.Enabled {
		return fmt.Errorf("certificateSha384 must not be set unless enabled is true")
	}
	if !sha384Hex.MatchString(strings.ReplaceAll(block.CertificateSha384, ":", "")) {
		return fmt.Errorf("certificateSha384 must be 96 hex characters, optionally colon-separated")
	}
	return nil
}
