// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func genesisTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

// selfSignedCert generates a self-signed RSA certificate and returns
// (pemBytes, derBytes) so tests can check both representations.
func selfSignedCert(t *testing.T) (pemBytes, derBytes []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	derBytes, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	pemBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return pemBytes, derBytes
}

func newGenesisTestCapsule(name string, nodeId int64, sigSecretName, grpcSecretName string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    nodeId,
			AccountId: fmt.Sprintf("0.0.%d", nodeId+3),
			Weight:    1,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{
						SecretName: sigSecretName,
						KeyName:    "s-public.pem",
					},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{
						SecretName: grpcSecretName,
						KeyName:    "hedera.crt",
					},
					GrpcTlsPrivateKey: operatorv1alpha1.KeyReference{
						SecretName: grpcSecretName,
						KeyName:    "hedera.key",
					},
					SigningPrivateKey: operatorv1alpha1.KeyReference{
						SecretName: sigSecretName,
						KeyName:    "s-private.pem",
					},
				},
			},
		},
	}
}

func newCertSecret(name, keyField string, certPEM []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Data:       map[string][]byte{keyField: certPEM},
	}
}

// ── unit tests ────────────────────────────────────────────────────────────────

func TestParseAccountNum(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.0.3", "3"},
		{"0.0.100", "100"},
		{"42", "42"}, // already a plain number
		{"", ""},
	}
	for _, c := range cases {
		if got := common.AccountNum(c.in); got != c.want {
			t.Errorf("AccountNum(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCreateGenesisNetworkStep_Run_Success(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, sigDER := selfSignedCert(t)
	grpcPEM, grpcDER := selfSignedCert(t)

	sigSecret := newCertSecret("node0-keys", "s-public.pem", sigPEM)
	grpcSecret := newCertSecret("node0-tls", "hedera.crt", grpcPEM)

	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	orbit := simpleOrbit("test-orbit")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(orbit, capsule, sigSecret, grpcSecret, op).
		Build()

	step := NewCreateGenesisNetworkStep(c)
	status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}

	result := step.Run(ctx, op, status)

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded, got %s", result.Phase)
	}
	if !step.IsSuccessful(op, result) {
		t.Error("expected IsSuccessful=true")
	}

	// Verify the ConfigMap was created
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm); err != nil {
		t.Fatalf("genesis-config ConfigMap not found: %v", err)
	}

	jsonStr, ok := cm.Data[consensus.GenesisNetworkFileName]
	if !ok {
		t.Fatalf("genesis-network.json key missing from ConfigMap")
	}

	var parsed genesisNetworkFile
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("genesis-network.json is not valid JSON: %v", err)
	}

	if len(parsed.NodeMetadata) != 1 {
		t.Fatalf("expected 1 node, got %d", len(parsed.NodeMetadata))
	}

	node := parsed.NodeMetadata[0].Node
	roster := parsed.NodeMetadata[0].RosterEntry

	// gossipCaCertificate must be base64-encoded DER bytes of the signing cert
	wantGossipCert := base64.StdEncoding.EncodeToString(sigDER)
	if node.GossipCaCertificate != wantGossipCert {
		t.Errorf("gossipCaCertificate mismatch")
	}
	if roster.GossipCaCertificate != wantGossipCert {
		t.Errorf("roster gossipCaCertificate mismatch")
	}

	// grpcCertificateHash must be the hex-encoded SHA-384 of the gRPC cert DER bytes
	sum := sha512.Sum384(grpcDER)
	wantGrpcHash := fmt.Sprintf("%x", sum)
	if node.GrpcCertificateHash != wantGrpcHash {
		t.Errorf("grpcCertificateHash mismatch: got %s, want %s", node.GrpcCertificateHash, wantGrpcHash)
	}

	// structure checks
	if node.NodeId != 0 {
		t.Errorf("expected nodeId=0, got %d", node.NodeId)
	}
	if node.AccountId.AccountNum != "3" {
		t.Errorf("expected accountNum=3, got %s", node.AccountId.AccountNum)
	}
	if node.Description != "node0" {
		t.Errorf("expected description=node0, got %s", node.Description)
	}
	if len(node.GossipEndpoint) != 1 || node.GossipEndpoint[0].Port != consensus.GossipPort {
		t.Errorf("unexpected gossip endpoint: %+v", node.GossipEndpoint)
	}
	if len(node.ServiceEndpoint) != 1 || node.ServiceEndpoint[0].Port != consensus.GrpcPort {
		t.Errorf("unexpected service endpoint: %+v", node.ServiceEndpoint)
	}
}

func TestCreateGenesisNetworkStep_Run_WithAdminKey(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)
	adminKeyBytes := []byte{0x01, 0x02, 0x03, 0xAB, 0xCD}

	sigSecret := newCertSecret("node0-keys", "s-public.pem", sigPEM)
	grpcSecret := newCertSecret("node0-tls", "hedera.crt", grpcPEM)
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "node0-admin", Namespace: "test"},
		Data:       map[string][]byte{"public-key": adminKeyBytes},
	}

	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	capsule.Spec.Secrets.SecretResources.AdminPublicKey = &operatorv1alpha1.KeyReference{
		SecretName: "node0-admin",
		KeyName:    "public-key",
	}

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(simpleOrbit("test-orbit"), capsule, sigSecret, grpcSecret, adminSecret, op).
		Build()

	step := NewCreateGenesisNetworkStep(c)
	status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}
	result := step.Run(ctx, op, status)

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded, got %s", result.Phase)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm); err != nil {
		t.Fatalf("genesis-config not found: %v", err)
	}
	var parsed genesisNetworkFile
	_ = json.Unmarshal([]byte(cm.Data[consensus.GenesisNetworkFileName]), &parsed)

	wantAdminKey := fmt.Sprintf("%x", adminKeyBytes)
	if parsed.NodeMetadata[0].Node.AdminKey != wantAdminKey {
		t.Errorf("adminKey: got %s, want %s", parsed.NodeMetadata[0].Node.AdminKey, wantAdminKey)
	}
}

func TestCreateGenesisNetworkStep_Run_NoCapsules(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(simpleOrbit("test-orbit"), op).Build()

	step := NewCreateGenesisNetworkStep(c)
	status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}
	result := step.Run(ctx, op, status)

	if result.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", result.Phase)
	}
}

func TestCreateGenesisNetworkStep_Run_MissingSecret(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	// secrets not created in the fake client

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(simpleOrbit("test-orbit"), capsule, op).Build()

	step := NewCreateGenesisNetworkStep(c)
	status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}
	result := step.Run(ctx, op, status)

	if result.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed when secret missing, got %s", result.Phase)
	}
}

func TestCreateGenesisNetworkStep_ShouldSkip(t *testing.T) {
	step := NewCreateGenesisNetworkStep(nil)
	op := &operatorv1alpha1.NetworkOperation{}

	t.Run("not yet run", func(t *testing.T) {
		status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}
		if step.ShouldSkip(op, status) {
			t.Error("should not skip when not yet successful")
		}
	})
	t.Run("prior step failed", func(t *testing.T) {
		status := &GenesisStatus{Phase: operatorv1alpha1.PhaseFailed}
		if !step.ShouldSkip(op, status) {
			t.Error("should skip when phase is Failed")
		}
	})
	t.Run("already succeeded", func(t *testing.T) {
		status := &GenesisStatus{Phase: operatorv1alpha1.PhaseSucceeded}
		status.AddCondition(metav1.Condition{
			Type:   StepConditionTypeConfigCreated,
			Status: metav1.ConditionTrue,
			Reason: StepConditionReasonGenesisNetwork,
		})
		if !step.ShouldSkip(op, status) {
			t.Error("should skip when already succeeded")
		}
	})
}

func TestCreateGenesisNetworkStep_MultiNode_Ordered(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	var objs []client.Object
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	objs = append(objs, op, simpleOrbit("test-orbit"))

	// Create 3 nodes in reverse order to verify sorting
	for _, nodeId := range []int64{2, 0, 1} {
		name := fmt.Sprintf("node%d", nodeId)
		sigPEM, _ := selfSignedCert(t)
		grpcPEM, _ := selfSignedCert(t)
		objs = append(objs,
			newGenesisTestCapsule(name, nodeId, name+"-keys", name+"-tls"),
			newCertSecret(name+"-keys", "s-public.pem", sigPEM),
			newCertSecret(name+"-tls", "hedera.crt", grpcPEM),
		)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	step := NewCreateGenesisNetworkStep(c)
	status := &GenesisStatus{Phase: operatorv1alpha1.PhasePending}
	result := step.Run(ctx, op, status)

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded, got %s", result.Phase)
	}

	cm := &corev1.ConfigMap{}
	_ = c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm)
	var parsed genesisNetworkFile
	_ = json.Unmarshal([]byte(cm.Data[consensus.GenesisNetworkFileName]), &parsed)

	if len(parsed.NodeMetadata) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(parsed.NodeMetadata))
	}
	// Verify nodes are sorted by nodeId
	for i, entry := range parsed.NodeMetadata {
		if entry.Node.NodeId != int64(i) {
			t.Errorf("expected node[%d].nodeId=%d, got %d", i, i, entry.Node.NodeId)
		}
	}
}

func TestCreateGenesisNetworkStep_CustomPorts(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)

	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	capsule.Spec.GossipPort = 60111
	capsule.Spec.GrpcPort = 60211

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		simpleOrbit("test-orbit"), capsule, op,
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	result := NewCreateGenesisNetworkStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})
	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded, got %s", result.Phase)
	}

	cm := &corev1.ConfigMap{}
	_ = c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm)
	var parsed genesisNetworkFile
	_ = json.Unmarshal([]byte(cm.Data[consensus.GenesisNetworkFileName]), &parsed)

	node := parsed.NodeMetadata[0].Node
	if len(node.GossipEndpoint) != 1 || node.GossipEndpoint[0].Port != 60111 {
		t.Errorf("expected gossipPort=60111, got %+v", node.GossipEndpoint)
	}
	if len(node.ServiceEndpoint) != 1 || node.ServiceEndpoint[0].Port != 60211 {
		t.Errorf("expected grpcPort=60211, got %+v", node.ServiceEndpoint)
	}
}

func TestCreateGenesisNetworkStep_ExternalAddress_WithCACert(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	// One managed node
	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)

	// An external node with a signing cert
	extSigPEM, extSigDER := selfSignedCert(t)

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test",
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{
							{
								NodeId:                   5,
								SelfName:                 "ext-node5",
								NickName:                 "ext5",
								Weight:                   200,
								ExternalHostname:         "ext.example.com",
								ExternalPort:             50111,
								NodeAccount:              "0.0.8",
								PublicSigningCertificate: string(extSigPEM),
							},
						},
					},
				},
			},
		},
	}

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, op,
		newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls"),
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	result := NewCreateGenesisNetworkStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})
	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded, got %s: %+v", result.Phase, result.Conditions)
	}

	cm := &corev1.ConfigMap{}
	_ = c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm)
	var parsed genesisNetworkFile
	_ = json.Unmarshal([]byte(cm.Data[consensus.GenesisNetworkFileName]), &parsed)

	if len(parsed.NodeMetadata) != 2 {
		t.Fatalf("expected 2 nodes (1 managed + 1 external), got %d", len(parsed.NodeMetadata))
	}

	// External node should be nodeId=5 (sorted last)
	extEntry := parsed.NodeMetadata[1]
	if extEntry.Node.NodeId != 5 {
		t.Errorf("expected external node nodeId=5, got %d", extEntry.Node.NodeId)
	}

	wantCert := base64.StdEncoding.EncodeToString(extSigDER)
	if extEntry.Node.GossipCaCertificate != wantCert {
		t.Errorf("external node gossipCaCertificate mismatch")
	}
	if extEntry.RosterEntry.GossipCaCertificate != wantCert {
		t.Errorf("external roster gossipCaCertificate mismatch")
	}
}

func TestCreateGenesisNetworkStep_ExternalAddress_NoCACert(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test",
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{
							{
								NodeId:           5,
								Weight:           1,
								ExternalHostname: "ext.example.com",
								ExternalPort:     50111,
								NodeAccount:      "0.0.8",
								// PublicSigningCertificate intentionally absent
							},
						},
					},
				},
			},
		},
	}

	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, op,
		newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls"),
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	result := NewCreateGenesisNetworkStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})
	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded when no CACert, got %s", result.Phase)
	}

	cm := &corev1.ConfigMap{}
	_ = c.Get(ctx, client.ObjectKey{Name: consensus.GenesisConfigMapName, Namespace: "test"}, cm)
	var parsed genesisNetworkFile
	_ = json.Unmarshal([]byte(cm.Data[consensus.GenesisNetworkFileName]), &parsed)

	extEntry := parsed.NodeMetadata[1]
	if extEntry.Node.GossipCaCertificate != "" {
		t.Errorf("expected empty gossipCaCertificate when no cert provided, got %s", extEntry.Node.GossipCaCertificate)
	}
}
