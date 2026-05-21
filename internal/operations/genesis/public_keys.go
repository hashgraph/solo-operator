// SPDX-License-Identifier: Apache-2.0

package genesis

import "fmt"

const (
	PublicKeysConfigMapName = "public-keys"
)

// BuildPublicKeys returns one PEM signing cert per node, keyed by
// "s-public-<description>.pem". Skips entries without cert material.
func BuildPublicKeys(materials []RosterMaterial) map[string]string {
	out := make(map[string]string, len(materials))
	for _, m := range materials {
		if len(m.SigningCertPEM) == 0 {
			continue
		}
		name := m.Description
		if name == "" {
			name = fmt.Sprintf("node%d", m.NodeID)
		}
		out[fmt.Sprintf("s-public-%s.pem", name)] = string(m.SigningCertPEM)
	}
	return out
}
