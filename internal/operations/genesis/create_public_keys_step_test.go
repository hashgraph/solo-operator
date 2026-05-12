// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func publicKeysOp() *operatorv1alpha1.NetworkOperation {
	return &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-op", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
}

func simpleOrbit(name string) *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

// signingCapsule builds a capsule with only the SigningPublicCertificate ref set.
func signingCapsule(name string, nodeId int64, secretName string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			NodeId: nodeId,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{
						SecretName: secretName, KeyName: "s-public.pem",
					},
				},
			},
		},
	}
}

// signingSecret creates a Secret containing a PEM cert under s-public.pem.
func signingSecret(name string, pem []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Data:       map[string][]byte{"s-public.pem": pem},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreatePublicKeysStep_Run_Success(t *testing.T) {
	certPEM, _ := selfSignedCert(t)
	capsule := signingCapsule("node0", 0, "node0-sig")
	secret := signingSecret("node0-sig", certPEM)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(simpleOrbit("test-orbit"), capsule, secret).Build()
	step := NewCreatePublicKeysStep(c)
	status := step.Run(context.Background(), publicKeysOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: PublicKeysName}, cm); err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}
	if _, ok := cm.Data["s-public-node0.pem"]; !ok {
		t.Errorf("expected key s-public-node0.pem in ConfigMap, got: %v", cm.Data)
	}
}

func TestCreatePublicKeysStep_Run_OrbitNotFound(t *testing.T) {
	certPEM, _ := selfSignedCert(t)
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(signingCapsule("node0", 0, "node0-sig"), signingSecret("node0-sig", certPEM)).Build()
	status := NewCreatePublicKeysStep(c).Run(context.Background(), publicKeysOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when orbit not found")
	}
}

func TestCreatePublicKeysStep_Run_NoCapsules(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(simpleOrbit("test-orbit")).Build()
	status := NewCreatePublicKeysStep(c).Run(context.Background(), publicKeysOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when no capsules exist")
	}
}

func TestCreatePublicKeysStep_Run_MissingSecret(t *testing.T) {
	capsule := signingCapsule("node0", 0, "node0-sig")
	// secret intentionally absent
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(simpleOrbit("test-orbit"), capsule).Build()
	status := NewCreatePublicKeysStep(c).Run(context.Background(), publicKeysOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when signing secret is missing")
	}
}

func TestCreatePublicKeysStep_Run_CapsuleWithNoSecrets(t *testing.T) {
	// capsule without any Spec.Secrets is skipped gracefully — ConfigMap created (empty)
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"},
		Spec:       operatorv1alpha1.ConsensusCapsuleSpec{NodeId: 0},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(simpleOrbit("test-orbit"), capsule).Build()
	status := NewCreatePublicKeysStep(c).Run(context.Background(), publicKeysOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Errorf("expected success when capsule has no secrets: %v", status.Conditions)
	}
}

func TestCreatePublicKeysStep_Run_UpdateExisting(t *testing.T) {
	certPEM, _ := selfSignedCert(t)
	capsule := signingCapsule("node0", 0, "node0-sig")
	secret := signingSecret("node0-sig", certPEM)
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: PublicKeysName, Namespace: "test"},
		Data:       map[string]string{"old-key": "old-value"},
	}

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(simpleOrbit("test-orbit"), capsule, secret, existing).Build()
	status := NewCreatePublicKeysStep(c).Run(context.Background(), publicKeysOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success on update: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: PublicKeysName}, cm); err != nil {
		t.Fatalf("ConfigMap not found after update: %v", err)
	}
	if _, ok := cm.Data["s-public-node0.pem"]; !ok {
		t.Error("updated ConfigMap should contain s-public-node0.pem")
	}
	if _, ok := cm.Data["old-key"]; ok {
		t.Error("updated ConfigMap should not retain old keys")
	}
}

func TestCreatePublicKeysStep_Run_ExternalAddress_PlainPEM(t *testing.T) {
	certPEM, _ := selfSignedCert(t)
	capsule := signingCapsule("node0", 0, "node0-sig")
	secret := signingSecret("node0-sig", certPEM)

	extPEM, _ := selfSignedCert(t)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test",
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{
							{SelfName: "ext-node1", PublicSigningCertificate: string(extPEM)},
						},
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule, secret).Build()
	step := NewCreatePublicKeysStep(c)
	status := step.Run(context.Background(), publicKeysOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: PublicKeysName}, cm); err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}
	if _, ok := cm.Data["s-public-ext-node1.pem"]; !ok {
		t.Errorf("expected external address key in ConfigMap, got: %v", cm.Data)
	}
}

func TestCreatePublicKeysStep_Run_ExternalAddress_Base64PEM(t *testing.T) {
	certPEM, _ := selfSignedCert(t)
	capsule := signingCapsule("node0", 0, "node0-sig")
	secret := signingSecret("node0-sig", certPEM)

	extPEM, _ := selfSignedCert(t)
	encoded := base64.StdEncoding.EncodeToString(extPEM)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Namespace: "test",
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{
							{SelfName: "ext-node2", PublicSigningCertificate: encoded},
						},
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule, secret).Build()
	step := NewCreatePublicKeysStep(c)
	status := step.Run(context.Background(), publicKeysOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: PublicKeysName}, cm); err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}
	got, ok := cm.Data["s-public-ext-node2.pem"]
	if !ok {
		t.Fatalf("expected base64-decoded external address key in ConfigMap")
	}
	if got != string(extPEM) {
		t.Error("base64-encoded external cert should be decoded before storing")
	}
}

func TestCreatePublicKeysStep_ShouldSkip(t *testing.T) {
	step := NewCreatePublicKeysStep(nil)
	op := publicKeysOp()

	if step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending}) {
		t.Error("should not skip when step has not run yet")
	}

	if !step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhaseFailed}) {
		t.Error("should skip when phase is Failed")
	}

	succeeded := &GenesisStatus{Phase: operatorv1alpha1.PhaseRunning}
	succeeded.AddCondition(metav1.Condition{
		Type:   StepConditionTypeConfigCreated,
		Status: metav1.ConditionTrue,
		Reason: StepConditionReasonPublicKeyConfig,
	})
	if !step.ShouldSkip(op, succeeded) {
		t.Error("should skip when step already succeeded")
	}
}
