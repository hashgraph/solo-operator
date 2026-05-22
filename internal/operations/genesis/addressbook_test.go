// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"testing"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	"google.golang.org/protobuf/proto"
)

func TestBuildAddressBookBin_RoundTrip(t *testing.T) {
	_, sigDER := testSelfSignedCert(t)
	out, err := BuildAddressBookBin([]RosterMaterial{
		{
			NodeID:           0,
			AccountID:        "0.0.3",
			Description:      "node0",
			ServiceHost:      "node0.example",
			ServicePort:      50211,
			Weight:           1,
			SigningCertDER:   sigDER,
			SigningRSAPubKey: "deadbeef",
			GrpcCertHashHex:  "abc123",
		},
		{
			NodeID:           2,
			AccountID:        "0.0.5",
			Description:      "node2",
			ServiceHost:      "node2.example",
			ServicePort:      50211,
			Weight:           3,
			SigningRSAPubKey: "cafebabe",
			GrpcCertHashHex:  "deadbeef",
		},
	})
	if err != nil {
		t.Fatalf("BuildAddressBookBin: %v", err)
	}

	var book services.NodeAddressBook
	if err := proto.Unmarshal(out, &book); err != nil {
		t.Fatalf("proto.Unmarshal: %v", err)
	}
	if len(book.NodeAddress) != 2 {
		t.Fatalf("expected 2 NodeAddress entries, got %d", len(book.NodeAddress))
	}

	n0 := book.NodeAddress[0]
	if n0.NodeId != 0 {
		t.Errorf("entry 0 nodeId: got %d, want 0", n0.NodeId)
	}
	if n0.NodeAccountId == nil || n0.NodeAccountId.GetAccountNum() != 3 {
		t.Errorf("entry 0 accountNum: got %v", n0.NodeAccountId)
	}
	if n0.RSA_PubKey != "deadbeef" {
		t.Errorf("entry 0 RSA pubkey: got %q", n0.RSA_PubKey)
	}
	if string(n0.NodeCertHash) != "abc123" {
		t.Errorf("entry 0 NodeCertHash: got %q", n0.NodeCertHash)
	}
	//nolint:staticcheck // Stake is deprecated in the proto but still set by older mirror readers.
	if n0.Stake != 1 {
		//nolint:staticcheck // Stake is deprecated but still asserted to lock the wire-shape.
		t.Errorf("entry 0 stake: got %d, want 1", n0.Stake)
	}

	n1 := book.NodeAddress[1]
	if n1.NodeId != 2 || n1.NodeAccountId.GetAccountNum() != 5 {
		t.Errorf("entry 1 mismatch: nodeId=%d accountNum=%v", n1.NodeId, n1.NodeAccountId)
	}
}

func TestBuildAddressBookBin_InvalidAccountID(t *testing.T) {
	_, err := BuildAddressBookBin([]RosterMaterial{{
		NodeID:    0,
		AccountID: "not-an-account",
	}})
	if err == nil {
		t.Fatal("expected error for invalid AccountID")
	}
}
