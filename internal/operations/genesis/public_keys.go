// SPDX-License-Identifier: Apache-2.0

package genesis

import "fmt"

// BuildPublicKeys returns one PEM signing cert per node, keyed by
// "s-public-node<nodeId+1>.pem" — matching the CN's formatNodeName convention
// (nodeId=0 → "node1", nodeId=1 → "node2", etc.).
func BuildPublicKeys(materials []RosterMaterial) map[string]string {
	out := make(map[string]string, len(materials))
	for _, m := range materials {
		if len(m.SigningCertPEM) == 0 {
			continue
		}
		out[fmt.Sprintf("s-public-node%d.pem", m.NodeID+1)] = string(m.SigningCertPEM)
	}
	return out
}
