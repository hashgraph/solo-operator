// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/automa-saga/errx"

	"github.com/hashgraph/solo-operator/internal/keys"
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
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, errx.WithReason(errMaterial.Wrap(err, "marshal genesis-network.json"), ReasonGenesisJSONMarshal)
	}
	return data, nil
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
			return genesisNodeMetadata{}, errx.WithReason(errMaterial.Wrap(err, "decode grpcCertHash hex"), ReasonCertHexInvalid)
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
		adminKey, err = adminKeyFromHex(m.AdminKeyHex)
		if err != nil {
			return genesisNodeMetadata{}, err
		}
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

// adminKeyFromHex builds the genesis admin Key message from a hex-encoded public
// key, choosing the proto Key field by signature scheme (Ed25519 or compressed
// ECDSA secp256k1). The scheme is detected with the shared keys predicates, which
// mirror the consensus node's own key checks. Any other key is rejected.
func adminKeyFromHex(adminHex string) (*genesisKey, error) {
	raw, err := hex.DecodeString(adminHex)
	if err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errMaterial.Wrap(err, "admin key is not valid hex text"), ReasonAdminKeyInvalid),
			"Store the admin public key as hex text: 64 characters for Ed25519 or 66 for compressed secp256k1",
		)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	switch {
	case keys.IsValidEd25519PublicKey(raw):
		return &genesisKey{Ed25519: b64}, nil
	case keys.IsValidECDSASecp256k1PublicKey(raw):
		return &genesisKey{ECDSASecp256k1: b64}, nil
	default:
		return nil, errx.WithHints(
			errx.WithReason(
				errMaterial.New("admin key is %d bytes; want 32 (Ed25519) or 33 (compressed ECDSA secp256k1)", len(raw)),
				ReasonAdminKeyInvalid),
			"Provide a 32-byte Ed25519 or 33-byte compressed secp256k1 admin public key",
		)
	}
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
	Ed25519        string `json:"ed25519,omitempty"`
	ECDSASecp256k1 string `json:"ECDSASecp256k1,omitempty"`
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

// ValidatePassthroughJSON checks that raw is valid JSON containing a non-empty
// "nodeMetadata" array — the minimum shape of the Network proto projection.
func ValidatePassthroughJSON(raw string) error {
	if !json.Valid([]byte(raw)) {
		return errx.WithHints(
			errx.WithReason(errValidation.New("not valid JSON"), ReasonGenesisJSONInvalid),
			"Provide a valid proto3-JSON Network document",
		)
	}
	var probe genesisNetworkFile
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return errx.WithHints(
			errx.WithReason(errValidation.Wrap(err, "parse failed"), ReasonGenesisJSONInvalid),
			"Provide a valid proto3-JSON Network document",
		)
	}
	if len(probe.NodeMetadata) == 0 {
		return errx.WithHints(
			errx.WithReason(errValidation.New("nodeMetadata array is missing or empty"), ReasonGenesisJSONInvalid),
			"Include a non-empty nodeMetadata array in the genesis JSON",
		)
	}
	return nil
}
