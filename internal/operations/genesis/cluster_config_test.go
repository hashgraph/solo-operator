// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"strings"
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestBuildClusterConfig_ContainsExpectedKeys(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{}
	materials := []RosterMaterial{{
		NodeID:          0,
		AccountID:       "0.0.3",
		Description:     "node0",
		GossipHost:      "node0.svc",
		GossipPort:      50111,
		Weight:          1,
		ExternalAddress: "node0.example",
	}}

	data := BuildClusterConfig(orbit, materials, []string{"10.0.0.1", "10.0.0.2"})

	wantKeys := []string{
		ConfigKeyConfigTxt, ConfigKeyPodIPs,
		"settings.txt", "application.properties", "api-permission.properties",
		"bootstrap.properties", "genesis-throttles.json",
	}
	for _, k := range wantKeys {
		if _, ok := data[k]; !ok {
			t.Errorf("missing expected key %q in cluster-config data", k)
		}
	}

	configTxt := data[ConfigKeyConfigTxt]
	if !strings.HasPrefix(configTxt, "swirld, ") {
		t.Errorf("config.txt should start with swirld, got: %s", configTxt)
	}
	if !strings.Contains(configTxt, "address, 0, node0, node0,") {
		t.Errorf("config.txt should contain address line for node0; got:\n%s", configTxt)
	}

	if data[ConfigKeyPodIPs] != "10.0.0.1\n10.0.0.2" {
		t.Errorf("pod-ips mismatch: got %q", data[ConfigKeyPodIPs])
	}
}

func TestBuildClusterConfig_OrbitOverridesWin(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings:          map[string]string{"showInternalStats": "0"},
				ApplicationConfiguration: map[string]string{"contracts.chainId": "9999"},
				PermissionConfiguration:  map[string]string{"freeze": "0-*"},
				Genesis: &operatorv1alpha1.OrbitGenesis{
					ThrottlesJson: `{"override": true}`,
				},
			},
		},
	}
	data := BuildClusterConfig(orbit, []RosterMaterial{{NodeID: 0, GossipHost: "h", GossipPort: 1, Weight: 1}}, nil)

	// settings.txt uses comma-padded "key, value" — the override changes the
	// default of "1" to "0".
	if !strings.Contains(data["settings.txt"], "showInternalStats,") || !strings.Contains(data["settings.txt"], " 0\n") {
		t.Errorf("settings.txt override (showInternalStats=0) not applied; got:\n%s", data["settings.txt"])
	}
	if !strings.Contains(data["application.properties"], "contracts.chainId=9999") {
		t.Errorf("application.properties override (contracts.chainId) not applied; got:\n%s", data["application.properties"])
	}
	if !strings.Contains(data["api-permission.properties"], "freeze=0-*") {
		t.Errorf("api-permission.properties override (freeze) not applied; got:\n%s", data["api-permission.properties"])
	}
	if data["genesis-throttles.json"] != `{"override": true}` {
		t.Errorf("genesis-throttles.json override not applied verbatim; got: %s", data["genesis-throttles.json"])
	}
}

func TestBuildClusterConfig_LedgerIDPropagates(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{LedgerId: "0x01"},
				},
			},
		},
	}
	data := BuildClusterConfig(orbit, []RosterMaterial{{NodeID: 0, GossipHost: "h", GossipPort: 1, Weight: 1}}, nil)

	if !strings.HasPrefix(data[ConfigKeyConfigTxt], "swirld, 298\n") {
		t.Errorf("config.txt should use ledger 298 for 0x01; got first line: %q", firstLine(data[ConfigKeyConfigTxt]))
	}
	if !strings.Contains(data["application.properties"], "contracts.chainId=298") {
		t.Errorf("application.properties chainId should be 298 for ledger 0x01")
	}
	if !strings.Contains(data["bootstrap.properties"], "contracts.chainId=298") {
		t.Errorf("bootstrap.properties chainId should be 298 for ledger 0x01")
	}
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

func TestLedgerIDToHex(t *testing.T) {
	cases := []struct{ in, wantID, wantChain string }{
		{"0x01", "0x01", "298"},
		{"0x00", "0x00", "0"},
		{"298", "0x01", "298"},
		{"0", "0x00", "0"},
		{"0xab", "0xab", "ab"},
		{"42", "0x42", "42"},
	}
	for _, tc := range cases {
		gotID, gotChain := ledgerIDToHex(tc.in)
		if gotID != tc.wantID || gotChain != tc.wantChain {
			t.Errorf("ledgerIDToHex(%q) = (%q, %q), want (%q, %q)", tc.in, gotID, gotChain, tc.wantID, tc.wantChain)
		}
	}
}
