// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"fmt"
	"strconv"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	"google.golang.org/protobuf/proto"

	"github.com/hashgraph/solo-operator/internal/errx"
)

func BuildAddressBookBin(materials []RosterMaterial) ([]byte, error) {
	nodes := make([]*services.NodeAddress, 0, len(materials))
	for _, m := range materials {
		n, err := nodeAddressFromMaterial(m)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", m.NodeID, err)
		}
		nodes = append(nodes, n)
	}
	data, err := proto.Marshal(&services.NodeAddressBook{NodeAddress: nodes})
	if err != nil {
		return nil, errx.WithReason(errMaterial.Wrap(err, "marshal address book"), ReasonAddressBookMarshal)
	}
	return data, nil
}

func nodeAddressFromMaterial(m RosterMaterial) (*services.NodeAddress, error) {
	shard, realm, account, err := splitAccountID(m.AccountID, m.NodeID)
	if err != nil {
		return nil, err
	}
	return &services.NodeAddress{
		NodeId: m.NodeID,
		NodeAccountId: &services.AccountID{
			ShardNum: shard,
			RealmNum: realm,
			Account:  &services.AccountID_AccountNum{AccountNum: account},
		},
		NodeCertHash: []byte(m.GrpcCertHashHex),
		RSA_PubKey:   m.SigningRSAPubKey,
		ServiceEndpoint: []*services.ServiceEndpoint{
			{DomainName: m.ServiceHost, Port: m.ServicePort},
		},
		Description: m.Description,
		Stake:       m.Weight,
	}, nil
}

func splitAccountID(accountID string, nodeID int64) (shard, realm, account int64, _ error) {
	s, r, a, err := splitAccountIDParts(accountID, nodeID)
	if err != nil {
		return 0, 0, 0, err
	}
	if shard, err = strconv.ParseInt(s, 10, 64); err != nil {
		return 0, 0, 0, errx.WithReason(errValidation.Wrap(err, "shard"), ReasonAccountIDInvalid)
	}
	if realm, err = strconv.ParseInt(r, 10, 64); err != nil {
		return 0, 0, 0, errx.WithReason(errValidation.Wrap(err, "realm"), ReasonAccountIDInvalid)
	}
	if account, err = strconv.ParseInt(a, 10, 64); err != nil {
		return 0, 0, 0, errx.WithReason(errValidation.Wrap(err, "account"), ReasonAccountIDInvalid)
	}
	return shard, realm, account, nil
}
