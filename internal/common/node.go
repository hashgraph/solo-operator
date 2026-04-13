// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strings"
)

// NodeAlias returns the canonical alias for a node given its ID (e.g. 0 → "node0").
func NodeAlias(nodeId int64) string {
	return fmt.Sprintf("node%d", nodeId)
}

// AccountIdForNode returns the default Hedera account ID for a node.
// The account number is derived as 3 + nodeId, which matches the Hedera genesis convention
// where node 0 → 0.0.3, node 1 → 0.0.4, etc.
// An explicit accountId on the capsule spec should always take precedence over this default.
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
