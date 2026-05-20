// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/operations/genesis"
)

func ngTestScheme(t *testing.T) *runtime.Scheme {
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

func ngSelfSignedCertPEM(t *testing.T) []byte {
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
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func ngCapsule(name string, nodeId int64) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    nodeId,
			AccountId: fmt.Sprintf("0.0.%d", nodeId+3),
			Weight:    1,
			Secrets: &operatorv1alpha1.ConsensusSecrets{
				SecretResources: &operatorv1alpha1.ConsensusSecretResources{
					SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "s-public.pem"},
					SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: name + "-keys", KeyName: "s-private.pem"},
					GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: name + "-tls", KeyName: "hedera.crt"},
					GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: name + "-tls", KeyName: "hedera.key"},
				},
			},
		},
	}
}

func ngSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Data:       data,
	}
}

func ngCapsuleSecrets(name string, sigPEM, grpcPEM []byte) []client.Object {
	return []client.Object{
		ngSecret(name+"-keys", map[string][]byte{
			"s-public.pem":  sigPEM,
			"s-private.pem": []byte("sign-priv"),
		}),
		ngSecret(name+"-tls", map[string][]byte{
			"hedera.crt": grpcPEM,
			"hedera.key": []byte("tls-priv"),
		}),
	}
}

// reconcileToCompletion drives Reconcile until it returns no requeue and no
// error. Use this only for success-path tests; failure paths return errors
// and need to call Reconcile directly.
func reconcileToCompletion(t *testing.T, r *NetworkGenesisReconciler, name types.NamespacedName) {
	t.Helper()
	const maxLoops = 10
	for i := 0; i < maxLoops; i++ {
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: name})
		if err != nil {
			t.Fatalf("Reconcile error on iteration %d: %v", i, err)
		}
		if !result.Requeue && result.RequeueAfter == 0 {
			return
		}
	}
	t.Fatalf("Reconcile did not converge within %d iterations", maxLoops)
}

func TestNetworkGenesisReconciler_HappyPath(t *testing.T) {
	scheme := ngTestScheme(t)
	sigPEM := ngSelfSignedCertPEM(t)
	grpcPEM := ngSelfSignedCertPEM(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := ngCapsule("node0", 0)
	ng := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-1", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkGenesisSpec{Orbit: "test-orbit", OperationID: "genesis-2026-05-20"},
	}

	objs := []client.Object{orbit, capsule, ng}
	objs = append(objs, ngCapsuleSecrets("node0", sigPEM, grpcPEM)...)

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&operatorv1alpha1.NetworkGenesis{}).
		WithObjects(objs...).Build()

	r := NewNetworkGenesisReconciler(c, scheme)
	name := types.NamespacedName{Namespace: "test", Name: "genesis-1"}
	reconcileToCompletion(t, r, name)

	got := &operatorv1alpha1.NetworkGenesis{}
	if err := c.Get(context.Background(), name, got); err != nil {
		t.Fatalf("get NetworkGenesis: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected Phase=Succeeded, got %s; conditions=%v", got.Status.Phase, got.Status.Conditions)
	}
	if got.Status.CompletionTime == nil {
		t.Error("expected CompletionTime to be set")
	}
	if got.Status.ObservedGeneration != got.Generation {
		t.Errorf("expected ObservedGeneration=%d, got %d", got.Generation, got.Status.ObservedGeneration)
	}

	for _, cmName := range []string{
		genesis.GenesisConfigMapName,
		genesis.AddressBookBinConfigMapName,
		genesis.ClusterConfigMapName,
	} {
		cm := &corev1.ConfigMap{}
		if err := c.Get(context.Background(), types.NamespacedName{Namespace: "test", Name: cmName}, cm); err != nil {
			t.Errorf("ConfigMap %s not created: %v", cmName, err)
		}
	}

	if len(got.Status.GeneratedConfigMaps) != 3 {
		t.Errorf("expected 3 generated ConfigMaps in status, got %v", got.Status.GeneratedConfigMaps)
	}
}

func TestNetworkGenesisReconciler_ExplicitRoster(t *testing.T) {
	scheme := ngTestScheme(t)
	sigPEM := ngSelfSignedCertPEM(t)
	grpcPEM := ngSelfSignedCertPEM(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	ng := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-1", Namespace: "test"},
		Spec: operatorv1alpha1.NetworkGenesisSpec{
			Orbit:       "test-orbit",
			OperationID: "explicit-roster",
			Roster: []operatorv1alpha1.NetworkGenesisRosterEntry{
				{
					NodeID:              7,
					AccountID:           "0.0.10",
					Description:         "node7",
					GossipHost:          "node7.example.com",
					GossipPort:          50111,
					ServiceHost:         "node7.example.com",
					ServicePort:         50211,
					GossipCaCertificate: string(sigPEM),
					GrpcCertificate:     string(grpcPEM),
					Weight:              2,
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&operatorv1alpha1.NetworkGenesis{}).
		WithObjects(orbit, ng).Build() // no ConsensusCapsules, no Secrets

	r := NewNetworkGenesisReconciler(c, scheme)
	name := types.NamespacedName{Namespace: "test", Name: "genesis-1"}
	reconcileToCompletion(t, r, name)

	got := &operatorv1alpha1.NetworkGenesis{}
	if err := c.Get(context.Background(), name, got); err != nil {
		t.Fatalf("get NetworkGenesis: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected Phase=Succeeded, got %s; conditions=%v", got.Status.Phase, got.Status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "test", Name: genesis.GenesisConfigMapName}, cm); err != nil {
		t.Fatalf("genesis-config ConfigMap not created: %v", err)
	}
	body := cm.Data[genesis.GenesisNetworkFileName]
	if body == "" {
		t.Fatal("genesis-network.json is empty")
	}
	if !strings.Contains(body, `"nodeId": "7"`) {
		t.Errorf("expected genesis-network.json to contain nodeId 7; got:\n%s", body)
	}
	if !strings.Contains(body, `"description": "node7"`) {
		t.Errorf("expected description=node7 in genesis-network.json")
	}
	if !strings.Contains(body, `"weight": "2"`) {
		t.Errorf("expected weight=2 (string-encoded uint64) in genesis-network.json")
	}
}

func TestNetworkGenesisReconciler_Idempotent(t *testing.T) {
	scheme := ngTestScheme(t)
	sigPEM := ngSelfSignedCertPEM(t)
	grpcPEM := ngSelfSignedCertPEM(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := ngCapsule("node0", 0)
	ng := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-1", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkGenesisSpec{Orbit: "test-orbit", OperationID: "genesis-2026-05-20"},
	}

	objs := []client.Object{orbit, capsule, ng}
	objs = append(objs, ngCapsuleSecrets("node0", sigPEM, grpcPEM)...)

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&operatorv1alpha1.NetworkGenesis{}).
		WithObjects(objs...).Build()

	r := NewNetworkGenesisReconciler(c, scheme)
	name := types.NamespacedName{Namespace: "test", Name: "genesis-1"}
	reconcileToCompletion(t, r, name)

	first := snapshotConfigMaps(t, c)

	// Clearing the terminal Phase forces a second pass through the pipeline;
	// idempotency means ConfigMap content stays byte-identical.
	got := &operatorv1alpha1.NetworkGenesis{}
	if err := c.Get(context.Background(), name, got); err != nil {
		t.Fatalf("get NetworkGenesis: %v", err)
	}
	got.Status.Phase = ""
	got.Status.Conditions = nil
	if err := c.Status().Update(context.Background(), got); err != nil {
		t.Fatalf("clear status: %v", err)
	}
	reconcileToCompletion(t, r, name)

	second := snapshotConfigMaps(t, c)
	for k, v := range first {
		if string(second[k]) != string(v) {
			t.Errorf("ConfigMap %s changed on second reconcile (not idempotent)", k)
		}
	}
}

// Missing Secret is a transient failure that recovers after the Secret is
// created — Phase=Failed is observational, not terminal.
func TestNetworkGenesisReconciler_MissingSecret_RecoversAfterCreation(t *testing.T) {
	scheme := ngTestScheme(t)

	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := ngCapsule("node0", 0)
	ng := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-1", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkGenesisSpec{Orbit: "test-orbit", OperationID: "genesis-2026-05-20"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&operatorv1alpha1.NetworkGenesis{}).
		WithObjects(orbit, capsule, ng).Build()

	r := NewNetworkGenesisReconciler(c, scheme)
	name := types.NamespacedName{Namespace: "test", Name: "genesis-1"}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: name}); err != nil {
		t.Fatalf("first reconcile (finalizer install): unexpected error: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: name}); err == nil {
		t.Fatal("expected reconcile error when secrets are missing")
	}

	got := &operatorv1alpha1.NetworkGenesis{}
	if err := c.Get(context.Background(), name, got); err != nil {
		t.Fatalf("get NetworkGenesis: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected Phase=Failed, got %s; conditions=%v", got.Status.Phase, got.Status.Conditions)
	}
	if !hasConditionTrue(got.Status.Conditions, networkGenesisConditionFailed) {
		t.Error("expected Failed condition to be set")
	}

	// Now create the missing secrets and reconcile again — should recover.
	for _, obj := range ngCapsuleSecrets("node0", ngSelfSignedCertPEM(t), ngSelfSignedCertPEM(t)) {
		if err := c.Create(context.Background(), obj); err != nil {
			t.Fatalf("create secret: %v", err)
		}
	}
	reconcileToCompletion(t, r, name)

	if err := c.Get(context.Background(), name, got); err != nil {
		t.Fatalf("get NetworkGenesis after recovery: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected Phase=Succeeded after secret creation, got %s; conditions=%v", got.Status.Phase, got.Status.Conditions)
	}
}

func hasConditionTrue(conditions []operatorv1alpha1.Condition, typ string) bool {
	for _, c := range conditions {
		if c.Type == typ && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// snapshotConfigMaps returns (name → deterministic flattened bytes) for
// every ConfigMap in the "test" namespace.
func snapshotConfigMaps(t *testing.T, c client.Client) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	list := &corev1.ConfigMapList{}
	if err := c.List(context.Background(), list, client.InNamespace("test")); err != nil {
		t.Fatalf("list configmaps: %v", err)
	}
	for _, cm := range list.Items {
		var buf []byte
		dataKeys := make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			dataKeys = append(dataKeys, k)
		}
		sort.Strings(dataKeys)
		for _, k := range dataKeys {
			buf = append(buf, []byte(k+"="+cm.Data[k]+";")...)
		}
		binKeys := make([]string, 0, len(cm.BinaryData))
		for k := range cm.BinaryData {
			binKeys = append(binKeys, k)
		}
		sort.Strings(binKeys)
		for _, k := range binKeys {
			buf = append(buf, []byte(k+"=")...)
			buf = append(buf, cm.BinaryData[k]...)
			buf = append(buf, ';')
		}
		out[cm.Name] = buf
	}
	return out
}
