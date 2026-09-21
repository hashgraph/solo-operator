// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// GenesisNode is one node's parsed "node" entry from a genesis-network.json
// document (com.hedera.node.internal.network.Network's
// nodeMetadata[].node) — the same file solo-operator's NetworkGenesis
// controller writes into the genesis-config ConfigMap.
type GenesisNode struct {
	// Expected is the classic address-book node ID (config.txt's "address, N,
	// ...") this node create must land on. NodeCreateTransaction itself has no
	// way to request a specific ID — the network auto-assigns the next one —
	// so callers must verify the assigned ID against Expected after submission
	// (see CheckNodeID).
	Expected uint64
	// Transaction is the unfrozen, unsigned NodeCreateTransaction built from the
	// node's genesis entry. It never sets AdminKey: genesis material only ever
	// carries the node's admin PUBLIC key, and NodeCreateTransaction requires
	// the admin key itself to co-sign when set — there is no private key
	// in-cluster to provide that signature at genesis time.
	Transaction *hiero.NodeCreateTransaction
}

// ParseGenesisNetwork parses a genesis-network.json document into one
// GenesisNode per entry that carries a "node" object, sorted ascending by
// node ID. Entries with no "node" object (a roster-only passthrough) are
// skipped; if every entry is roster-only, that is reported as an error rather
// than silently returning nothing to create.
func ParseGenesisNetwork(data []byte) ([]GenesisNode, error) {
	var file genesisNetworkFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, errx.Decorate(
			errValidation.Wrap(err, "parse genesis-network.json"),
			reasonGenesisNetworkInvalid,
			"Confirm the file is the genesis-network.json emitted into the genesis-config ConfigMap",
		)
	}
	if len(file.NodeMetadata) == 0 {
		return nil, errx.Decorate(
			errValidation.New("genesis-network.json has no nodeMetadata entries"),
			reasonGenesisNetworkInvalid,
			"Confirm the file was extracted correctly and is not empty",
		)
	}

	nodes := make([]GenesisNode, 0, len(file.NodeMetadata))
	for i, meta := range file.NodeMetadata {
		if meta.Node == nil {
			continue
		}
		node, err := genesisNodeFromEntry(*meta.Node)
		if err != nil {
			return nil, fmt.Errorf("nodeMetadata[%d]: %w", i, err)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errx.Decorate(
			errValidation.New("genesis-network.json has no \"node\" entries (roster-only)"),
			reasonGenesisNetworkInvalid,
			"Every nodeMetadata entry is missing its \"node\" object; regenerate genesis-network.json via "+
				"solo-operator's NetworkGenesis controller rather than a roster-only passthrough",
		)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Expected < nodes[j].Expected })
	return nodes, nil
}

// CheckNodeID confirms the network assigned the node the classic node ID it
// was meant to mirror. NodeCreateTransaction cannot request an ID, so this is
// the only place a mismatch can be caught — and it must be caught, since a
// silently wrong DAB node ID would leave later NodeUpdateTransaction/
// NodeDeleteTransaction calls targeting the wrong entity.
func CheckNodeID(expected uint64, receipt hiero.TransactionReceipt) error {
	if receipt.NodeID != expected {
		return errx.Decorate(
			errNodeIDMismatch.New("network assigned node id %d, expected %d", receipt.NodeID, expected),
			reasonNodeIDMismatch,
			"Another NodeCreateTransaction landed on this network first; the dynamic address book cannot be "+
				"trivially reconciled to the classic roster from here — investigate before creating any further nodes",
		)
	}
	return nil
}

func genesisNodeFromEntry(e genesisNodeEntry) (GenesisNode, error) {
	nodeID, err := strconv.ParseUint(e.NodeID, 10, 64)
	if err != nil {
		return GenesisNode{}, errx.Decorate(
			errValidation.Wrap(err, "parse node id %q", e.NodeID),
			reasonGenesisNetworkInvalid,
			"nodeId must be a decimal integer",
		)
	}

	shard, err := strconv.ParseUint(e.AccountID.ShardNum, 10, 64)
	if err != nil {
		return GenesisNode{}, invalidGenesisField("accountId.shardNum", e.AccountID.ShardNum, err)
	}
	realm, err := strconv.ParseUint(e.AccountID.RealmNum, 10, 64)
	if err != nil {
		return GenesisNode{}, invalidGenesisField("accountId.realmNum", e.AccountID.RealmNum, err)
	}
	account, err := strconv.ParseUint(e.AccountID.AccountNum, 10, 64)
	if err != nil {
		return GenesisNode{}, invalidGenesisField("accountId.accountNum", e.AccountID.AccountNum, err)
	}

	gossipCert, err := decodeGenesisBase64("gossipCaCertificate", e.GossipCaCertificate)
	if err != nil {
		return GenesisNode{}, err
	}

	tx := hiero.NewNodeCreateTransaction().
		SetAccountID(hiero.AccountID{Shard: shard, Realm: realm, Account: account}).
		SetDescription(e.Description).
		SetGossipEndpoints(genesisEndpoints(e.GossipEndpoint)).
		SetServiceEndpoints(genesisEndpoints(e.ServiceEndpoint)).
		SetGossipCaCertificate(gossipCert)

	if e.GrpcCertificateHash != "" {
		hash, err := decodeGenesisBase64("grpcCertificateHash", e.GrpcCertificateHash)
		if err != nil {
			return GenesisNode{}, err
		}
		tx.SetGrpcCertificateHash(hash)
	}

	return GenesisNode{Expected: nodeID, Transaction: tx}, nil
}

func genesisEndpoints(entries []genesisServiceEndpoint) []hiero.Endpoint {
	endpoints := make([]hiero.Endpoint, 0, len(entries))
	for _, e := range entries {
		var ep hiero.Endpoint
		ep.SetDomainName(e.DomainName).SetPort(e.Port)
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

func decodeGenesisBase64(field, value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, invalidGenesisField(field, value, err)
	}
	return raw, nil
}

func invalidGenesisField(field, value string, cause error) error {
	return errx.Decorate(
		errValidation.Wrap(cause, "parse %s %q", field, value),
		reasonGenesisNetworkInvalid,
		"Confirm the file is an unmodified genesis-network.json from the genesis-config ConfigMap",
	)
}

// genesisNetworkFile and its nested types mirror exactly the proto3-JSON shape
// internal/operations/genesis/genesis_network.go writes (uint64 as strings,
// bytes as base64) — the wire format for com.hedera.node.internal.network.Network.
type genesisNetworkFile struct {
	NodeMetadata []genesisNodeMetadata `json:"nodeMetadata"`
}

type genesisNodeMetadata struct {
	Node *genesisNodeEntry `json:"node"`
}

type genesisNodeEntry struct {
	NodeID              string                   `json:"nodeId"`
	AccountID           genesisAccountID         `json:"accountId"`
	Description         string                   `json:"description"`
	GossipEndpoint      []genesisServiceEndpoint `json:"gossipEndpoint"`
	ServiceEndpoint     []genesisServiceEndpoint `json:"serviceEndpoint"`
	GossipCaCertificate string                   `json:"gossipCaCertificate"`
	GrpcCertificateHash string                   `json:"grpcCertificateHash,omitempty"`
}

type genesisAccountID struct {
	ShardNum   string `json:"shardNum"`
	RealmNum   string `json:"realmNum"`
	AccountNum string `json:"accountNum"`
}

type genesisServiceEndpoint struct {
	DomainName string `json:"domainName,omitempty"`
	Port       int32  `json:"port"`
}
