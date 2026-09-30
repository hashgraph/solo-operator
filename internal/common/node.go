// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strconv"
	"strings"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// NodeAlias returns the canonical alias for a node given its ID (e.g. 0 → "node0").
func NodeAlias(nodeId int64) string {
	return fmt.Sprintf("node%d", nodeId)
}

// FormatNodeName is the single encoder of the CN gossip key-file node-name convention
// (ref hiero-consensus-node #15824 / PR #17487): 1-based by default (nodeId=0 → "node1"),
// or 0-based (nodeId=0 → "node0") under the nodeNameZeroBased feature gate.
func FormatNodeName(nodeId int64, zeroBased bool) string {
	if zeroBased {
		return fmt.Sprintf("node%d", nodeId)
	}
	return fmt.Sprintf("node%d", nodeId+1)
}

// AccountIdForNode returns the default Hedera account ID for a node (3 + nodeId).
func AccountIdForNode(nodeId int64) string {
	return fmt.Sprintf("0.0.%d", 3+nodeId)
}

// AccountNum extracts the account number from a dotted "realm.shard.account" string.
// Returns the full input unchanged when it does not match the expected format.
func AccountNum(accountId string) string {
	parts := strings.Split(accountId, ".")
	if len(parts) == 3 {
		return parts[2]
	}
	return accountId
}

// ParseAccountIDParts splits a "shard.realm.account" string into its int64 components.
// Falls back to shard=0, realm=0, account=3+nodeId when the format is not recognised.
func ParseAccountIDParts(accountId string, nodeId int64) (shard, realm, account int64) {
	parts := strings.Split(accountId, ".")
	if len(parts) == 3 {
		if s, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			if r, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				if a, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
					return s, r, a
				}
			}
		}
	}
	return 0, 0, 3 + nodeId
}

// InternalHostname returns the per-capsule ClusterIP service FQDN, or InternalAddress if set.
func InternalHostname(capsule *operatorv1alpha1.ConsensusCapsule) string {
	if capsule.Spec.InternalAddress != "" {
		return capsule.Spec.InternalAddress
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", capsule.Name, capsule.Namespace)
}
