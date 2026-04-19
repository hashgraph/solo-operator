// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func checkSecretsOp() *operatorv1alpha1.NetworkOperation {
	return &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-op", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
}

// allKeysCapsule builds a capsule whose four required secrets all live in one Secret.
func allKeysCapsule(name string, nodeId int64) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			NodeId: nodeId,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "tls.key"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "tls.crt"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "s-private.pem"},
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "s-public.pem"},
				},
			},
		},
	}
}

// fullKeysSecret creates a Secret that satisfies allKeysCapsule.
func fullKeysSecret(capsuleName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: capsuleName + "-keys", Namespace: "test"},
		Data: map[string][]byte{
			"tls.key":       []byte("grpc-priv"),
			"tls.crt":       []byte("grpc-cert"),
			"s-private.pem": []byte("sign-priv"),
			"s-public.pem":  []byte("sign-pub"),
		},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCheckSecretsStep_Run_Success(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(allKeysCapsule("node0", 0), fullKeysSecret("node0")).Build()
	step := NewCheckSecretsStep(c)
	status := step.Run(context.Background(), checkSecretsOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success, got failed: %v", status.Conditions)
	}
	if !step.IsSuccessful(checkSecretsOp(), status) {
		t.Error("IsSuccessful should return true")
	}
}

func TestCheckSecretsStep_Run_NoCapsules(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	// No capsules yet → step returns pending status (will retry) rather than failing.
	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Error("expected pending (retry) when no capsules exist, got PhaseFailed")
	}
}

func TestCheckSecretsStep_Run_SecretNotFound(t *testing.T) {
	// capsule references secrets that don't exist in the cluster
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(allKeysCapsule("node0", 0)).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when referenced secret is absent")
	}
}

func TestCheckSecretsStep_Run_SecretMissingKey(t *testing.T) {
	// secret exists but is missing the required key
	capsule := allKeysCapsule("node0", 0)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "node0-keys", Namespace: "test"},
		Data:       map[string][]byte{"wrong-key": []byte("v")},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).WithObjects(capsule, secret).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when secret is missing required key")
	}
}

func TestCheckSecretsStep_Run_NoSecretsConfigured(t *testing.T) {
	// capsule with no Spec.Secrets at all — vacuous validation, should succeed
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"},
		Spec:       operatorv1alpha1.ConsensusCapsuleSpec{NodeId: 0},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).WithObjects(capsule).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Error("expected success when capsule has no secrets configured")
	}
}

func TestCheckSecretsStep_Run_OptionalSecretPresent(t *testing.T) {
	capsule := allKeysCapsule("node0", 0)
	capsule.Spec.Secrets.SecretResources.LedgerEncryptionKey = &operatorv1alpha1.KeyReference{
		SecretName: "node0-ledger", KeyName: "value",
	}
	ledger := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "node0-ledger", Namespace: "test"},
		Data:       map[string][]byte{"value": []byte("enc-key")},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(capsule, fullKeysSecret("node0"), ledger).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Errorf("expected success with optional secret present: %v", status.Conditions)
	}
}

func TestCheckSecretsStep_Run_OptionalSecretMissing(t *testing.T) {
	// optional secret is referenced but not present in the cluster
	capsule := allKeysCapsule("node0", 0)
	capsule.Spec.Secrets.SecretResources.LedgerEncryptionKey = &operatorv1alpha1.KeyReference{
		SecretName: "node0-ledger", KeyName: "value",
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(capsule, fullKeysSecret("node0")).Build()
	status := NewCheckSecretsStep(c).Run(context.Background(), checkSecretsOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when optional secret is referenced but absent")
	}
}

func TestCheckSecretsStep_ShouldSkip(t *testing.T) {
	step := NewCheckSecretsStep(nil)
	op := checkSecretsOp()

	if step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending}) {
		t.Error("should not skip when step has not run yet")
	}

	if !step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhaseFailed}) {
		t.Error("should skip when phase is Failed")
	}

	succeeded := &GenesisStatus{Phase: operatorv1alpha1.PhaseRunning}
	succeeded.AddCondition(metav1.Condition{
		Type:   StepConditionTypeVerifySecrets,
		Status: metav1.ConditionTrue,
		Reason: StepConditionReasonConsensusNodeSecrets,
	})
	if !step.ShouldSkip(op, succeeded) {
		t.Error("should skip when step already succeeded")
	}
}
