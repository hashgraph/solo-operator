// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// GenesisNode is one node's parsed "node" entry from a genesis-network.json
// document — the same file solo-operator's NetworkGenesis controller writes
// into the genesis-config ConfigMap.
type GenesisNode struct {
	// Expected is the classic address-book node ID this create must land on;
	// NodeCreateTransaction can't request an ID, so callers must verify it via
	// CheckNodeID after submission.
	Expected uint64
	// Transaction has AdminKey unset (genesis-network.json only carries the
	// public key). Callers with the matching private key (LoadAdminKey) may
	// set AdminKey and co-sign before submitting.
	Transaction *hiero.NodeCreateTransaction
}

// ParseGenesisNetwork parses a genesis-network.json document into one
// GenesisNode per "node" entry, sorted ascending by node ID. Roster-only
// entries are skipped; if every entry is roster-only that's an error rather
// than an empty result.
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

// CheckNodeID fails if the network assigned a node ID other than expected —
// the only point a mismatch can be caught, since NodeCreateTransaction can't
// request a specific ID.
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

// LoadAdminKey reads a node's admin private key from
// dir/node-node<ID>-admin.key — hashgen's raw-hex ed25519 seed format, as
// bundled into the system-account-keys Secret.
func LoadAdminKey(dir string, nodeID uint64) (hiero.PrivateKey, error) {
	path := filepath.Join(dir, fmt.Sprintf("node-node%d-admin.key", nodeID))
	data, err := os.ReadFile(path)
	if err != nil {
		return hiero.PrivateKey{}, errx.Decorate(
			errValidation.Wrap(err, "read admin key %q", path),
			reasonAdminKeyReadFailed,
			fmt.Sprintf("Confirm the admin key for node %d exists at %q (from the system-account-keys Secret)", nodeID, path),
		)
	}
	key, err := hiero.PrivateKeyFromStringEd25519(strings.TrimSpace(string(data)))
	if err != nil {
		return hiero.PrivateKey{}, errx.Decorate(
			errValidation.Wrap(err, "decode admin key %q", path),
			reasonAdminKeyReadFailed,
			"Confirm the file holds a raw hex ed25519 seed (hashgen's node-<alias>-admin.key format)",
		)
	}
	return key, nil
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

// genesisNetworkFile mirrors the proto3-JSON shape
// internal/operations/genesis/genesis_network.go writes.
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
