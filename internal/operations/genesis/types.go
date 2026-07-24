// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"fmt"
	"strings"

	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

const (
	GenesisConfigMapName      = consensus.GenesisConfigMapName
	GenesisNetworkFileName    = consensus.GenesisNetworkFileName
	GenesisPropertiesFileName = consensus.GenesisPropertiesFileName
	ClusterConfigMapName      = consensus.ClusterConfigMapName

	PublicKeysConfigMapName     = consensus.PublicKeysConfigMapName
	AddressBookBinConfigMapName = "addressbook-bin"
	AddressBookBinKey           = "addressbook.bin"

	ConfigKeyConfigTxt = "config.txt"
	ConfigKeyPodIPs    = "pod-ips"
)

// RosterMaterial holds one node's pre-parsed inputs shared by every builder.
type RosterMaterial struct {
	NodeID      int64
	AccountID   string
	Description string

	GossipHost  string
	GossipPort  int32
	ServiceHost string
	ServicePort int32

	Weight      int64
	AdminKeyHex string

	SigningCertPEM   []byte
	SigningCertDER   []byte
	SigningRSAPubKey string
	GrpcCertHashHex  string

	ExternalAddress string
}

// splitAccountIDParts parses "shard.realm.account"; empty falls back to 0.0.(3+nodeID).
func splitAccountIDParts(accountID string, nodeID int64) (shard, realm, account string, err error) {
	id := strings.TrimSpace(accountID)
	if id == "" {
		return "0", "0", fmt.Sprintf("%d", 3+nodeID), nil
	}
	parts := strings.Split(id, ".")
	if len(parts) != 3 {
		return "", "", "", errx.WithHints(
			errx.WithReason(errValidation.New("expected shard.realm.account, got %q", accountID), ReasonAccountIDInvalid),
			"Set accountID in shard.realm.account form (e.g. 0.0.3)",
		)
	}
	return parts[0], parts[1], parts[2], nil
}
