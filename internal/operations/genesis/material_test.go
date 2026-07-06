// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("add clientgo scheme: %v", err)
	}
	if err := operatorv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add operator scheme: %v", err)
	}
	return s
}

func TestMaterialsFromSpec_Basic(t *testing.T) {
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)

	mats, err := MaterialsFromSpec([]operatorv1alpha1.NetworkGenesisRosterEntry{{
		NodeID:              5,
		AccountID:           "0.0.8",
		GossipHost:          "n5.svc",
		GossipPort:          50111,
		ServiceHost:         "n5.svc",
		ServicePort:         50211,
		GossipCaCertificate: string(sigPEM),
		GrpcCertificate:     string(grpcPEM),
	}})
	if err != nil {
		t.Fatalf("MaterialsFromSpec: %v", err)
	}
	if len(mats) != 1 {
		t.Fatalf("expected 1 material, got %d", len(mats))
	}
	m := mats[0]
	if m.NodeID != 5 || m.AccountID != "0.0.8" {
		t.Errorf("identity mismatch: %+v", m)
	}
	if m.Weight != 1 {
		t.Errorf("weight default: got %d, want 1", m.Weight)
	}
	if len(m.SigningCertDER) == 0 {
		t.Error("SigningCertDER not populated")
	}
	if m.SigningRSAPubKey == "" {
		t.Error("SigningRSAPubKey not populated")
	}
	if m.GrpcCertHashHex == "" {
		t.Error("GrpcCertHashHex not populated")
	}
}

func TestMaterialsFromSpec_SortedByNodeID(t *testing.T) {
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)
	mk := func(id int64) operatorv1alpha1.NetworkGenesisRosterEntry {
		return operatorv1alpha1.NetworkGenesisRosterEntry{
			NodeID:     id,
			AccountID:  "0.0.3",
			GossipHost: "h", GossipPort: 1,
			ServiceHost: "h", ServicePort: 1,
			GossipCaCertificate: string(sigPEM),
			GrpcCertificate:     string(grpcPEM),
		}
	}
	mats, err := MaterialsFromSpec([]operatorv1alpha1.NetworkGenesisRosterEntry{mk(3), mk(0), mk(1)})
	if err != nil {
		t.Fatalf("MaterialsFromSpec: %v", err)
	}
	for i, want := range []int64{0, 1, 3} {
		if mats[i].NodeID != want {
			t.Errorf("entry %d: nodeId=%d, want %d", i, mats[i].NodeID, want)
		}
	}
}

func TestMaterialsFromCluster_FromCapsules(t *testing.T) {
	scheme := testScheme(t)
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-public.pem"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-private.pem"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.crt"},
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.key"},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, capsule,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "test"},
			Data: map[string][]byte{"s-public.pem": sigPEM, "s-private.pem": []byte("x")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "test"},
			Data: map[string][]byte{"hedera.crt": grpcPEM, "hedera.key": []byte("x")}},
	).Build()

	mats, err := MaterialsFromCluster(context.Background(), c, "test", orbit)
	if err != nil {
		t.Fatalf("MaterialsFromCluster: %v", err)
	}
	if len(mats) != 1 {
		t.Fatalf("expected 1 material, got %d", len(mats))
	}
	if mats[0].NodeID != 0 || mats[0].AccountID != "0.0.3" {
		t.Errorf("unexpected material: %+v", mats[0])
	}
}

func TestMaterialsFromCluster_NoCapsulesNoExternal(t *testing.T) {
	scheme := testScheme(t)
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit).Build()
	if _, err := MaterialsFromCluster(context.Background(), c, "test", orbit); err == nil {
		t.Error("expected error when no capsules and no external addresses")
	}
}

func TestMaterialsFromCluster_MergesExternalAddresses(t *testing.T) {
	scheme := testScheme(t)
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)
	extPEM, _ := testSelfSignedCert(t)

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{
					AddressBook: operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{{
							NodeId:                   3,
							SelfName:                 "ext3",
							Weight:                   1,
							ExternalHostname:         "ext3.example",
							ExternalPort:             50111,
							NodeAccount:              "0.0.6",
							PublicSigningCertificate: string(extPEM),
						}},
					},
				},
			},
		},
	}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node1", Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 1, AccountId: "0.0.4", Weight: 1,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-public.pem"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-private.pem"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.crt"},
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.key"},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, capsule,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "test"},
			Data: map[string][]byte{"s-public.pem": sigPEM, "s-private.pem": []byte("x")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "test"},
			Data: map[string][]byte{"hedera.crt": grpcPEM, "hedera.key": []byte("x")}},
	).Build()

	mats, err := MaterialsFromCluster(context.Background(), c, "test", orbit)
	if err != nil {
		t.Fatalf("MaterialsFromCluster: %v", err)
	}
	if len(mats) != 2 {
		t.Fatalf("expected 2 materials (1 capsule + 1 external), got %d", len(mats))
	}
	// Sorted ascending by NodeID: capsule (NodeID=1), then external (NodeID=3).
	if mats[0].NodeID != 1 || mats[1].NodeID != 3 {
		t.Errorf("expected sorted [1, 3], got [%d, %d]", mats[0].NodeID, mats[1].NodeID)
	}
	if mats[1].Description != "ext3" {
		t.Errorf("external entry description: got %q, want %q", mats[1].Description, "ext3")
	}
	if mats[1].AccountID != "0.0.6" {
		t.Errorf("external entry accountID: got %q, want %q", mats[1].AccountID, "0.0.6")
	}
}

func TestMaterialsFromCluster_CapsuleWithAdminKey(t *testing.T) {
	scheme := testScheme(t)
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)
	adminBytes := []byte{0xde, 0xad, 0xbe, 0xef}

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-public.pem"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-private.pem"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.crt"},
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.key"},
					AdminPublicKey:           &operatorv1alpha1.KeyReference{SecretName: "admin", KeyName: "value"},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, capsule,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "test"},
			Data: map[string][]byte{"s-public.pem": sigPEM, "s-private.pem": []byte("x")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "test"},
			Data: map[string][]byte{"hedera.crt": grpcPEM, "hedera.key": []byte("x")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "admin", Namespace: "test"},
			Data: map[string][]byte{"value": adminBytes}},
	).Build()

	mats, err := MaterialsFromCluster(context.Background(), c, "test", orbit)
	if err != nil {
		t.Fatalf("MaterialsFromCluster: %v", err)
	}
	if mats[0].AdminKeyHex != "deadbeef" {
		t.Errorf("admin key hex: got %q, want %q", mats[0].AdminKeyHex, "deadbeef")
	}
}

func TestMaterialsFromCluster_CapsuleDefaultsApplied(t *testing.T) {
	scheme := testScheme(t)
	sigPEM, _ := testSelfSignedCert(t)
	grpcPEM, _ := testSelfSignedCert(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node7", Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:  "test-orbit",
			NodeId: 7,
			// AccountId left empty → defaults to "0.0.10"
			// Weight zero → defaults to 1
			// GossipPort/GrpcPort zero → defaults from consensus pkg
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-public.pem"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "k", KeyName: "s-private.pem"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.crt"},
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: "t", KeyName: "hedera.key"},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		orbit, capsule,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "test"},
			Data: map[string][]byte{"s-public.pem": sigPEM, "s-private.pem": []byte("x")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "test"},
			Data: map[string][]byte{"hedera.crt": grpcPEM, "hedera.key": []byte("x")}},
	).Build()

	mats, err := MaterialsFromCluster(context.Background(), c, "test", orbit)
	if err != nil {
		t.Fatalf("MaterialsFromCluster: %v", err)
	}
	m := mats[0]
	if m.AccountID != "0.0.10" { // 3 + nodeId(7)
		t.Errorf("accountID default: got %q, want %q", m.AccountID, "0.0.10")
	}
	if m.Weight != 1 {
		t.Errorf("weight default: got %d, want 1", m.Weight)
	}
	if m.GossipPort == 0 || m.ServicePort == 0 {
		t.Errorf("ports should default to non-zero, got gossip=%d service=%d", m.GossipPort, m.ServicePort)
	}
}

func TestMaterialsFromCluster_ExternalAddressMissingFieldsErrors(t *testing.T) {
	scheme := testScheme(t)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{
					AddressBook: operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{{
							NodeId:           99,
							ExternalHostname: "", // intentionally empty
							NodeAccount:      "", // intentionally empty
						}},
					},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit).Build()
	if _, err := MaterialsFromCluster(context.Background(), c, "test", orbit); err == nil {
		t.Error("expected error when external address missing required fields")
	}
}
