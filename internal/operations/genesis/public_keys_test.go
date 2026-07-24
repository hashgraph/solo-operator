// SPDX-License-Identifier: Apache-2.0

package genesis

import "testing"

func TestBuildPublicKeys_KeyedByNodeIdPlusOne(t *testing.T) {
	pem1, _ := testSelfSignedCert(t)
	pem2, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 0, SigningCertPEM: pem1},
		{NodeID: 1, SigningCertPEM: pem2},
	})
	// CN naming: nodeId=0 → "node1", nodeId=1 → "node2"
	if string(pem1) != out["s-public-node1.pem"] {
		t.Errorf("node0 pem mismatch: got keys %v", out)
	}
	if string(pem2) != out["s-public-node2.pem"] {
		t.Errorf("node1 pem mismatch: got keys %v", out)
	}
}

func TestBuildPublicKeys_SkipsEntriesWithoutCert(t *testing.T) {
	pem, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 0, SigningCertPEM: pem},
		{NodeID: 1}, // no SigningCertPEM
	})
	if len(out) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(out), out)
	}
	if _, ok := out["s-public-node2.pem"]; ok {
		t.Error("entry without cert should be skipped")
	}
}

func TestBuildPublicKeys_UsesNodeIdPlusOneNaming(t *testing.T) {
	pem, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 7, SigningCertPEM: pem},
	})
	// nodeId=7 → "node8"
	if _, ok := out["s-public-node8.pem"]; !ok {
		t.Errorf("expected key s-public-node8.pem, got %v", out)
	}
}
