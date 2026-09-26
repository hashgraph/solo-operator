// SPDX-License-Identifier: Apache-2.0

package genesis

import "github.com/hashgraph/solo-operator/internal/common"

// BuildPublicKeys returns one PEM signing cert per node, keyed by
// "s-public-<nodeName>.pem" where nodeName follows common.FormatNodeName.
func BuildPublicKeys(materials []RosterMaterial, zeroBased bool) map[string]string {
	out := make(map[string]string, len(materials))
	for _, m := range materials {
		if len(m.SigningCertPEM) == 0 {
			continue
		}
		nodeName := common.FormatNodeName(m.NodeID, zeroBased)
		out["s-public-"+nodeName+".pem"] = string(m.SigningCertPEM)
	}
	return out
}
