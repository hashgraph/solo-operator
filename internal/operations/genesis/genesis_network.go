// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// BuildGenesisNetworkJSON renders the proto3-JSON projection of
// com.hedera.node.internal.network.Network (uint64 as strings, bytes as
// base64, adminKey as a proto Key message).
func BuildGenesisNetworkJSON(materials []RosterMaterial) ([]byte, error) {
	out := genesisNetworkFile{NodeMetadata: make([]genesisNodeMetadata, 0, len(materials))}
	for _, m := range materials {
		meta, err := nodeMetadataFromMaterial(m)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", m.NodeID, err)
		}
		out.NodeMetadata = append(out.NodeMetadata, meta)
	}
	return json.MarshalIndent(out, "", "  ")
}

func nodeMetadataFromMaterial(m RosterMaterial) (genesisNodeMetadata, error) {
	gossipCA := ""
	if len(m.SigningCertDER) > 0 {
		gossipCA = base64.StdEncoding.EncodeToString(m.SigningCertDER)
	}

	grpcHash := ""
	if m.GrpcCertHashHex != "" {
		raw, err := hex.DecodeString(m.GrpcCertHashHex)
		if err != nil {
			return genesisNodeMetadata{}, fmt.Errorf("decode grpcCertHash hex: %w", err)
		}
		grpcHash = base64.StdEncoding.EncodeToString(raw)
	}

	account, err := parseAccountID(m.AccountID, m.NodeID)
	if err != nil {
		return genesisNodeMetadata{}, fmt.Errorf("accountId %q: %w", m.AccountID, err)
	}

	weight := strconvUint64(m.Weight)
	gossipEndpoints := []genesisServiceEndpoint{{DomainName: m.GossipHost, Port: m.GossipPort}}
	serviceEndpoints := []genesisServiceEndpoint{{DomainName: m.ServiceHost, Port: m.ServicePort}}

	var adminKey *genesisKey
	if m.AdminKeyHex != "" {
		raw, err := hex.DecodeString(m.AdminKeyHex)
		if err != nil {
			return genesisNodeMetadata{}, fmt.Errorf("decode adminKey hex: %w", err)
		}
		adminKey = &genesisKey{Ed25519: base64.StdEncoding.EncodeToString(raw)}
	}

	return genesisNodeMetadata{
		RosterEntry: genesisRosterEntry{
			NodeID:              strconvUint64(m.NodeID),
			Weight:              weight,
			GossipCaCertificate: gossipCA,
			GossipEndpoint:      gossipEndpoints,
		},
		Node: genesisNodeEntry{
			NodeID:              strconvUint64(m.NodeID),
			AccountID:           account,
			Description:         m.Description,
			GossipEndpoint:      gossipEndpoints,
			ServiceEndpoint:     serviceEndpoints,
			GossipCaCertificate: gossipCA,
			GrpcCertificateHash: grpcHash,
			Weight:              weight,
			AdminKey:            adminKey,
		},
	}, nil
}

func strconvUint64(v int64) string { return fmt.Sprintf("%d", v) }

func parseAccountID(accountID string, nodeID int64) (genesisAccountID, error) {
	shard, realm, account, err := splitAccountIDParts(accountID, nodeID)
	if err != nil {
		return genesisAccountID{}, err
	}
	return genesisAccountID{ShardNum: shard, RealmNum: realm, AccountNum: account}, nil
}

type genesisServiceEndpoint struct {
	DomainName string `json:"domainName,omitempty"`
	Port       int32  `json:"port"`
}

type genesisAccountID struct {
	ShardNum   string `json:"shardNum"`
	RealmNum   string `json:"realmNum"`
	AccountNum string `json:"accountNum"`
}

type genesisKey struct {
	Ed25519 string `json:"ed25519,omitempty"`
}

type genesisRosterEntry struct {
	NodeID              string                   `json:"nodeId"`
	Weight              string                   `json:"weight"`
	GossipCaCertificate string                   `json:"gossipCaCertificate"`
	GossipEndpoint      []genesisServiceEndpoint `json:"gossipEndpoint"`
}

type genesisNodeEntry struct {
	NodeID              string                   `json:"nodeId"`
	AccountID           genesisAccountID         `json:"accountId"`
	Description         string                   `json:"description"`
	GossipEndpoint      []genesisServiceEndpoint `json:"gossipEndpoint"`
	ServiceEndpoint     []genesisServiceEndpoint `json:"serviceEndpoint"`
	GossipCaCertificate string                   `json:"gossipCaCertificate"`
	GrpcCertificateHash string                   `json:"grpcCertificateHash,omitempty"`
	Weight              string                   `json:"weight"`
	AdminKey            *genesisKey              `json:"adminKey,omitempty"`
}

type genesisNodeMetadata struct {
	RosterEntry genesisRosterEntry `json:"rosterEntry"`
	Node        genesisNodeEntry   `json:"node"`
}

type genesisNetworkFile struct {
	NodeMetadata []genesisNodeMetadata `json:"nodeMetadata"`
}
