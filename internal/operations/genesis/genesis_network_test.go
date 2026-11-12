// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func testSelfSignedCert(t *testing.T) (pemBytes, derBytes []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), der
}

func TestBuildGenesisNetworkJSON_ProtoShape(t *testing.T) {
	_, sigDER := testSelfSignedCert(t)
	adminKeyBytes := bytes.Repeat([]byte{0xab}, 32) // 32-byte Ed25519 public key
	grpcHashBytes := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	m := []RosterMaterial{{
		NodeID:           7,
		AccountID:        "0.0.10",
		Description:      "node7",
		GossipHost:       "node7.example",
		GossipPort:       50111,
		ServiceHost:      "node7.example",
		ServicePort:      50211,
		Weight:           2,
		AdminKeyHex:      hex.EncodeToString(adminKeyBytes),
		SigningCertDER:   sigDER,
		GrpcCertHashHex:  hex.EncodeToString(grpcHashBytes),
		SigningRSAPubKey: "deadbeef",
	}}

	out, err := BuildGenesisNetworkJSON(m)
	if err != nil {
		t.Fatalf("BuildGenesisNetworkJSON: %v", err)
	}

	var got struct {
		NodeMetadata []struct {
			RosterEntry struct {
				NodeID              string `json:"nodeId"`
				Weight              string `json:"weight"`
				GossipCaCertificate string `json:"gossipCaCertificate"`
			} `json:"rosterEntry"`
			Node struct {
				NodeID    string `json:"nodeId"`
				AccountID struct {
					ShardNum   string `json:"shardNum"`
					RealmNum   string `json:"realmNum"`
					AccountNum string `json:"accountNum"`
				} `json:"accountId"`
				Description         string `json:"description"`
				GossipCaCertificate string `json:"gossipCaCertificate"`
				GrpcCertificateHash string `json:"grpcCertificateHash"`
				Weight              string `json:"weight"`
				AdminKey            *struct {
					Ed25519 string `json:"ed25519"`
				} `json:"adminKey"`
			} `json:"node"`
		} `json:"nodeMetadata"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-parse genesis-network.json: %v\n%s", err, out)
	}
	if len(got.NodeMetadata) != 1 {
		t.Fatalf("expected 1 nodeMetadata entry, got %d", len(got.NodeMetadata))
	}
	nm := got.NodeMetadata[0]

	if nm.RosterEntry.NodeID != "7" {
		t.Errorf("rosterEntry.nodeId: got %q, want \"7\"", nm.RosterEntry.NodeID)
	}
	if nm.Node.NodeID != "7" {
		t.Errorf("node.nodeId: got %q, want \"7\"", nm.Node.NodeID)
	}
	if nm.RosterEntry.Weight != "2" {
		t.Errorf("rosterEntry.weight: got %q, want \"2\"", nm.RosterEntry.Weight)
	}
	if nm.Node.Weight != "2" {
		t.Errorf("node.weight: got %q, want \"2\"", nm.Node.Weight)
	}
	if nm.Node.AccountID.AccountNum != "10" {
		t.Errorf("node.accountId.accountNum: got %q, want \"10\"", nm.Node.AccountID.AccountNum)
	}

	want := base64.StdEncoding.EncodeToString(sigDER)
	if nm.RosterEntry.GossipCaCertificate != want {
		t.Errorf("rosterEntry.gossipCaCertificate base64 mismatch")
	}
	if nm.Node.GossipCaCertificate != want {
		t.Errorf("node.gossipCaCertificate base64 mismatch")
	}

	wantHash := base64.StdEncoding.EncodeToString(grpcHashBytes)
	if nm.Node.GrpcCertificateHash != wantHash {
		t.Errorf("grpcCertificateHash: got %q, want %q", nm.Node.GrpcCertificateHash, wantHash)
	}

	if nm.Node.AdminKey == nil {
		t.Fatal("expected adminKey to be set")
	}
	wantAdmin := base64.StdEncoding.EncodeToString(adminKeyBytes)
	if nm.Node.AdminKey.Ed25519 != wantAdmin {
		t.Errorf("adminKey.ed25519: got %q, want %q", nm.Node.AdminKey.Ed25519, wantAdmin)
	}

	// Catch regression to hex-string adminKey shape.
	if strings.Contains(string(out), `"adminKey": "`+hex.EncodeToString(adminKeyBytes)+`"`) {
		t.Error("adminKey emitted as hex string instead of proto Key message")
	}
}

func TestBuildGenesisNetworkJSON_AdminKeyECDSA(t *testing.T) {
	_, sigDER := testSelfSignedCert(t)
	// 33-byte compressed secp256k1 public key (0x02 prefix).
	adminKeyBytes := append([]byte{0x02}, bytes.Repeat([]byte{0xcd}, 32)...)

	out, err := BuildGenesisNetworkJSON([]RosterMaterial{{
		NodeID:         1,
		AccountID:      "0.0.4",
		GossipHost:     "h",
		ServiceHost:    "h",
		Weight:         1,
		AdminKeyHex:    hex.EncodeToString(adminKeyBytes),
		SigningCertDER: sigDER,
	}})
	if err != nil {
		t.Fatalf("BuildGenesisNetworkJSON: %v", err)
	}

	var got struct {
		NodeMetadata []struct {
			Node struct {
				AdminKey *genesisKey `json:"adminKey"`
			} `json:"node"`
		} `json:"nodeMetadata"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-parse: %v\n%s", err, out)
	}
	ak := got.NodeMetadata[0].Node.AdminKey
	if ak == nil {
		t.Fatal("expected adminKey to be set")
	}
	want := base64.StdEncoding.EncodeToString(adminKeyBytes)
	if ak.ECDSASecp256k1 != want {
		t.Errorf("adminKey.ECDSASecp256k1: got %q, want %q", ak.ECDSASecp256k1, want)
	}
	if ak.Ed25519 != "" {
		t.Errorf("adminKey.ed25519 should be empty for an ECDSA key, got %q", ak.Ed25519)
	}
	// The ECDSA key must serialize under the proto3-JSON field "ECDSASecp256k1".
	if !strings.Contains(string(out), `"ECDSASecp256k1"`) {
		t.Errorf("expected ECDSASecp256k1 field in output:\n%s", out)
	}
}

func TestBuildGenesisNetworkJSON_AdminKeyBadLength(t *testing.T) {
	_, sigDER := testSelfSignedCert(t)
	for _, tc := range []struct {
		name string
		key  []byte
	}{
		{"too short", []byte{0x01, 0x02, 0x03}},
		{"31 bytes", bytes.Repeat([]byte{0x01}, 31)},
		{"33 bytes wrong prefix", append([]byte{0x04}, bytes.Repeat([]byte{0x01}, 32)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildGenesisNetworkJSON([]RosterMaterial{{
				NodeID:         0,
				AccountID:      "0.0.3",
				GossipHost:     "h",
				ServiceHost:    "h",
				Weight:         1,
				AdminKeyHex:    hex.EncodeToString(tc.key),
				SigningCertDER: sigDER,
			}})
			if err == nil {
				t.Fatal("expected an error for an invalid admin key length")
			}
		})
	}
}

func TestBuildGenesisNetworkJSON_AdminKeyOmitted(t *testing.T) {
	_, sigDER := testSelfSignedCert(t)
	out, err := BuildGenesisNetworkJSON([]RosterMaterial{{
		NodeID:         0,
		AccountID:      "0.0.3",
		GossipHost:     "h",
		ServiceHost:    "h",
		Weight:         1,
		SigningCertDER: sigDER,
	}})
	if err != nil {
		t.Fatalf("BuildGenesisNetworkJSON: %v", err)
	}
	if strings.Contains(string(out), `"adminKey"`) {
		t.Errorf("adminKey should be omitted when no admin key set; got:\n%s", out)
	}
}

func TestValidatePassthroughJSON(t *testing.T) {
	validJSON := `{"nodeMetadata":[{"rosterEntry":{"nodeId":"0","weight":"1","gossipCaCertificate":"","gossipEndpoint":[{"domainName":"h","port":50111}]},"node":{"nodeId":"0","accountId":{"shardNum":"0","realmNum":"0","accountNum":"3"},"description":"","gossipEndpoint":[{"domainName":"h","port":50111}],"serviceEndpoint":[{"domainName":"h","port":50211}],"gossipCaCertificate":"","weight":"1"}}]}`

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid JSON with nodeMetadata", validJSON, false},
		{"not valid JSON", `{bad json`, true},
		{"missing nodeMetadata key", `{"other":"value"}`, true},
		{"empty nodeMetadata array", `{"nodeMetadata":[]}`, true},
		{"empty string", ``, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassthroughJSON(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidatePassthroughJSON(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}
