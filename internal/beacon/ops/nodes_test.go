// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"encoding/base64"
	"fmt"
	"testing"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// genesisNetworkFixture builds a minimal, well-formed genesis-network.json
// document with the given node ids, each a full "node" entry (never a
// roster-only one), matching what internal/operations/genesis/genesis_network.go
// actually emits.
func genesisNetworkFixture(nodeIDs ...int) []byte {
	cert := base64.StdEncoding.EncodeToString([]byte("fake-cert"))
	hash := base64.StdEncoding.EncodeToString([]byte("fake-hash"))

	entries := ""
	for i, id := range nodeIDs {
		if i > 0 {
			entries += ","
		}
		entries += fmt.Sprintf(`{
			"node": {
				"nodeId": "%d",
				"accountId": {"shardNum": "0", "realmNum": "0", "accountNum": "%d"},
				"description": "node%d",
				"gossipEndpoint": [{"domainName": "node%d.example.svc.cluster.local", "port": 50111}],
				"serviceEndpoint": [{"domainName": "node%d.example.svc.cluster.local", "port": 50211}],
				"gossipCaCertificate": "%s",
				"grpcCertificateHash": "%s"
			}
		}`, id, id+3, id, id, id, cert, hash)
	}
	return []byte(fmt.Sprintf(`{"nodeMetadata": [%s]}`, entries))
}

func TestParseGenesisNetwork_FieldMapping(t *testing.T) {
	nodes, err := ParseGenesisNetwork(genesisNetworkFixture(0))
	if err != nil {
		t.Fatalf("ParseGenesisNetwork: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	n := nodes[0]
	if n.Expected != 0 {
		t.Errorf("Expected = %d, want 0", n.Expected)
	}
	tx := n.Transaction
	if got := tx.GetAccountID(); got.Account != 3 {
		t.Errorf("account = %v, want account num 3", got)
	}
	if got := tx.GetDescription(); got != "node0" {
		t.Errorf("description = %q, want %q", got, "node0")
	}
	if eps := tx.GetGossipEndpoints(); len(eps) != 1 || eps[0].GetDomainName() != "node0.example.svc.cluster.local" || eps[0].GetPort() != 50111 {
		t.Errorf("gossip endpoints = %+v, want one node0 endpoint on 50111", eps)
	}
	if eps := tx.GetServiceEndpoints(); len(eps) != 1 || eps[0].GetPort() != 50211 {
		t.Errorf("service endpoints = %+v, want one endpoint on 50211", eps)
	}
	if string(tx.GetGossipCaCertificate()) != "fake-cert" {
		t.Errorf("gossip ca cert = %q, want %q", tx.GetGossipCaCertificate(), "fake-cert")
	}
	if string(tx.GetGrpcCertificateHash()) != "fake-hash" {
		t.Errorf("grpc cert hash = %q, want %q", tx.GetGrpcCertificateHash(), "fake-hash")
	}
	// AdminKey must never be set: no private key exists in-cluster to co-sign it.
	if got := tx.GetAdminKey(); got != nil {
		t.Errorf("admin key = %v, want unset", got)
	}
}

func TestParseGenesisNetwork_SortsAscendingByNodeID(t *testing.T) {
	nodes, err := ParseGenesisNetwork(genesisNetworkFixture(3, 1, 0, 2))
	if err != nil {
		t.Fatalf("ParseGenesisNetwork: %v", err)
	}
	var got []uint64
	for _, n := range nodes {
		got = append(got, n.Expected)
	}
	want := []uint64{0, 1, 2, 3}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("order = %v, want %v", got, want)
			break
		}
	}
}

func TestParseGenesisNetwork_SkipsRosterOnlyEntries(t *testing.T) {
	data := []byte(`{"nodeMetadata": [
		{"rosterEntry": {"nodeId": "0"}},
		{"node": {"nodeId": "1", "accountId": {"shardNum": "0", "realmNum": "0", "accountNum": "4"}, "gossipCaCertificate": ""}}
	]}`)
	nodes, err := ParseGenesisNetwork(data)
	if err != nil {
		t.Fatalf("ParseGenesisNetwork: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Expected != 1 {
		t.Errorf("got %+v, want exactly node 1", nodes)
	}
}

func TestParseGenesisNetwork_AllRosterOnlyRejected(t *testing.T) {
	data := []byte(`{"nodeMetadata": [{"rosterEntry": {"nodeId": "0"}}]}`)
	_, err := ParseGenesisNetwork(data)
	wantReason(t, err, reasonGenesisNetworkInvalid)
}

func TestParseGenesisNetwork_EmptyNodeMetadataRejected(t *testing.T) {
	_, err := ParseGenesisNetwork([]byte(`{"nodeMetadata": []}`))
	wantReason(t, err, reasonGenesisNetworkInvalid)
}

func TestParseGenesisNetwork_InvalidJSONRejected(t *testing.T) {
	_, err := ParseGenesisNetwork([]byte(`not json`))
	wantReason(t, err, reasonGenesisNetworkInvalid)
}

func TestParseGenesisNetwork_InvalidNodeIDRejected(t *testing.T) {
	data := []byte(`{"nodeMetadata": [{"node": {"nodeId": "not-a-number", "accountId": {"shardNum": "0", "realmNum": "0", "accountNum": "3"}}}]}`)
	_, err := ParseGenesisNetwork(data)
	wantReason(t, err, reasonGenesisNetworkInvalid)
}

func TestParseGenesisNetwork_InvalidCertificateBase64Rejected(t *testing.T) {
	data := []byte(`{"nodeMetadata": [{"node": {"nodeId": "0", "accountId": {"shardNum": "0", "realmNum": "0", "accountNum": "3"}, "gossipCaCertificate": "not-base64!!"}}]}`)
	_, err := ParseGenesisNetwork(data)
	wantReason(t, err, reasonGenesisNetworkInvalid)
}

func TestCheckNodeID(t *testing.T) {
	t.Run("match passes", func(t *testing.T) {
		if err := CheckNodeID(2, hiero.TransactionReceipt{NodeID: 2}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("mismatch fails", func(t *testing.T) {
		err := CheckNodeID(2, hiero.TransactionReceipt{NodeID: 5})
		wantReason(t, err, reasonNodeIDMismatch)
	})
}
