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

func TestBuildClusterConfig_DefaultsAlwaysUsed(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{
					AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
				},
			},
		},
	}
	data := BuildClusterConfig(orbit, []RosterMaterial{{NodeID: 0, GossipHost: "h", GossipPort: 1, Weight: 1}}, nil)

	if !strings.Contains(data["settings.txt"], "showInternalStats,") || !strings.Contains(data["settings.txt"], " 1\n") {
		t.Errorf("settings.txt should contain default showInternalStats=1; got:\n%s", data["settings.txt"])
	}
	if !strings.Contains(data["application.properties"], "ledger.id=0x00") {
		t.Errorf("application.properties should contain ledger.id from AddressBook; got:\n%s", data["application.properties"])
	}
	if !strings.Contains(data["api-permission.properties"], "freeze=2-58") {
		t.Errorf("api-permission.properties should contain default freeze=2-58; got:\n%s", data["api-permission.properties"])
	}
	if data["genesis-throttles.json"] != defaultThrottlesJSON {
		t.Errorf("genesis-throttles.json should be the built-in default; got: %s", data["genesis-throttles.json"])
	}
}

func TestBuildClusterConfig_LedgerIDAndChainIDPropagate(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{
					AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x01", ChainId: "298"},
				},
			},
		},
	}
	data := BuildClusterConfig(orbit, []RosterMaterial{{NodeID: 0, GossipHost: "h", GossipPort: 1, Weight: 1}}, nil)

	// config.txt swirld uses the chain id (the historical local value 298).
	if !strings.HasPrefix(data[ConfigKeyConfigTxt], "swirld, 298\n") {
		t.Errorf("config.txt swirld should be the chain id 298; got first line: %q", firstLine(data[ConfigKeyConfigTxt]))
	}
	// ledger.id comes from LedgerId verbatim; contracts.chainId from ChainId.
	if !strings.Contains(data["application.properties"], "ledger.id=0x01") {
		t.Errorf("application.properties ledger.id should be 0x01")
	}
	if !strings.Contains(data["application.properties"], "contracts.chainId=298") {
		t.Errorf("application.properties chainId should be 298")
	}
	if !strings.Contains(data["bootstrap.properties"], "contracts.chainId=298") {
		t.Errorf("bootstrap.properties chainId should be 298")
	}
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
