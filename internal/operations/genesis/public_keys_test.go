// SPDX-License-Identifier: Apache-2.0

package genesis

import "testing"

func TestBuildPublicKeys_KeyedByDescription(t *testing.T) {
	pem1, _ := testSelfSignedCert(t)
	pem2, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 0, Description: "node0", SigningCertPEM: pem1},
		{NodeID: 1, Description: "node1", SigningCertPEM: pem2},
	})
	if string(pem1) != out["s-public-node0.pem"] {
		t.Errorf("node0 pem mismatch")
	}
	if string(pem2) != out["s-public-node1.pem"] {
		t.Errorf("node1 pem mismatch")
	}
}

func TestBuildPublicKeys_SkipsEntriesWithoutCert(t *testing.T) {
	pem, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 0, Description: "node0", SigningCertPEM: pem},
		{NodeID: 1, Description: "node1"}, // no SigningCertPEM
	})
	if len(out) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(out), out)
	}
	if _, ok := out["s-public-node1.pem"]; ok {
		t.Error("entry without cert should be skipped")
	}
}

func TestBuildPublicKeys_FallbackKeyWhenDescriptionEmpty(t *testing.T) {
	pem, _ := testSelfSignedCert(t)
	out := BuildPublicKeys([]RosterMaterial{
		{NodeID: 7, SigningCertPEM: pem}, // no Description
	})
	if _, ok := out["s-public-node7.pem"]; !ok {
		t.Errorf("expected fallback key s-public-node7.pem, got %v", out)
	}
}
