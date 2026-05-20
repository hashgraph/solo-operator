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
