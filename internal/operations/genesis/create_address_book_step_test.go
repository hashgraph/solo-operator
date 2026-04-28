// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"encoding/base64"
	"testing"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func TestCreateAddressBookStep_Run_Success(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)

	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		capsule, op,
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	step := NewCreateAddressBookStep(c)
	result := step.Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded, got %s", result.Phase)
	}
	if !step.IsSuccessful(op, result) {
		t.Error("expected IsSuccessful=true")
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: AddressBookBinConfigMapName, Namespace: "test"}, cm); err != nil {
		t.Fatalf("addressbook-bin ConfigMap not found: %v", err)
	}

	bin := cm.BinaryData[AddressBookBinKey]
	if len(bin) == 0 {
		t.Fatal("addressbook.bin is empty")
	}

	var book services.NodeAddressBook
	if err := proto.Unmarshal(bin, &book); err != nil {
		t.Fatalf("failed to unmarshal NodeAddressBook: %v", err)
	}
	if len(book.NodeAddress) != 1 {
		t.Fatalf("expected 1 node, got %d", len(book.NodeAddress))
	}

	node := book.NodeAddress[0]
	if node.NodeId != 0 {
		t.Errorf("expected nodeId=0, got %d", node.NodeId)
	}
	if node.NodeAccountId == nil || node.NodeAccountId.GetAccountNum() != 3 {
		t.Errorf("expected accountNum=3, got %v", node.NodeAccountId)
	}
	if node.RSA_PubKey == "" {
		t.Error("expected RSA_PubKey to be set")
	}
	if len(node.NodeCertHash) == 0 {
		t.Error("expected NodeCertHash to be set")
	}
	if len(node.ServiceEndpoint) != 1 {
		t.Errorf("expected 1 service endpoint, got %d", len(node.ServiceEndpoint))
	}
}

func TestCreateAddressBookStep_Run_MultiNode_Sorted(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	var objs []client.Object
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	objs = append(objs, op)

	for _, nodeId := range []int64{2, 0, 1} {
		name := "node" + string(rune('0'+nodeId))
		sigPEM, _ := selfSignedCert(t)
		grpcPEM, _ := selfSignedCert(t)
		objs = append(objs,
			newGenesisTestCapsule(name, nodeId, name+"-keys", name+"-tls"),
			newCertSecret(name+"-keys", "s-public.pem", sigPEM),
			newCertSecret(name+"-tls", "hedera.crt", grpcPEM),
		)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	result := NewCreateAddressBookStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("expected PhaseSucceeded, got %s", result.Phase)
	}

	cm := &corev1.ConfigMap{}
	_ = c.Get(ctx, client.ObjectKey{Name: AddressBookBinConfigMapName, Namespace: "test"}, cm)

	var book services.NodeAddressBook
	_ = proto.Unmarshal(cm.BinaryData[AddressBookBinKey], &book)

	if len(book.NodeAddress) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(book.NodeAddress))
	}
	for i, node := range book.NodeAddress {
		if node.NodeId != int64(i) {
			t.Errorf("expected node[%d].nodeId=%d, got %d", i, i, node.NodeId)
		}
	}
}

func TestCreateAddressBookStep_Run_NoCapsules(t *testing.T) {
	ctx := context.Background()
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).WithObjects(op).Build()

	result := NewCreateAddressBookStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if result.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", result.Phase)
	}
}

func TestCreateAddressBookStep_Run_MissingSecret(t *testing.T) {
	ctx := context.Background()
	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).WithObjects(capsule, op).Build()

	result := NewCreateAddressBookStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if result.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed when secret missing, got %s", result.Phase)
	}
}

func TestCreateAddressBookStep_ShouldSkip(t *testing.T) {
	step := NewCreateAddressBookStep(nil)
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
			Reason: StepConditionReasonAddressBook,
		})
		if !step.ShouldSkip(op, status) {
			t.Error("should skip when already succeeded")
		}
	})
}

func TestCreateAddressBookStep_Idempotent(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)
	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		capsule, op,
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	step := NewCreateAddressBookStep(c)
	step.Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})
	// Second run should update without error
	result := step.Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if result.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded on second run, got %s", result.Phase)
	}
}

func TestMirrorValues_addressBookInjected(t *testing.T) {
	ctx := context.Background()
	scheme := genesisTestScheme()

	sigPEM, _ := selfSignedCert(t)
	grpcPEM, _ := selfSignedCert(t)
	capsule := newGenesisTestCapsule("node0", 0, "node0-keys", "node0-tls")
	op := &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-op", Namespace: "test", Generation: 1},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		capsule, op,
		newCertSecret("node0-keys", "s-public.pem", sigPEM),
		newCertSecret("node0-tls", "hedera.crt", grpcPEM),
	).Build()

	// Run the step to produce the ConfigMap
	NewCreateAddressBookStep(c).Run(ctx, op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	// Verify the ConfigMap contains valid base64-decodable proto
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: AddressBookBinConfigMapName, Namespace: "test"}, cm); err != nil {
		t.Fatalf("addressbook-bin not found: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(cm.BinaryData[AddressBookBinKey])
	if encoded == "" {
		t.Error("expected non-empty base64 addressbook")
	}

	// Verify it round-trips through proto
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	var book services.NodeAddressBook
	if err := proto.Unmarshal(decoded, &book); err != nil {
		t.Fatalf("proto unmarshal failed: %v", err)
	}
	if len(book.NodeAddress) != 1 {
		t.Errorf("expected 1 node in decoded book, got %d", len(book.NodeAddress))
	}
}

func TestParseAccountIDParts(t *testing.T) {
	cases := []struct {
		accountId string
		nodeId    int64
		shard     int64
		realm     int64
		account   int64
	}{
		{"0.0.3", 0, 0, 0, 3},
		{"0.0.100", 0, 0, 0, 100},
		{"1.2.50", 0, 1, 2, 50},
		{"", 0, 0, 0, 3},    // fallback: 3 + nodeId
		{"", 2, 0, 0, 5},    // fallback: 3 + nodeId
		{"bad", 1, 0, 0, 4}, // fallback: 3 + nodeId
	}
	for _, c := range cases {
		s, r, a := common.ParseAccountIDParts(c.accountId, c.nodeId)
		if s != c.shard || r != c.realm || a != c.account {
			t.Errorf("ParseAccountIDParts(%q, %d) = (%d,%d,%d), want (%d,%d,%d)",
				c.accountId, c.nodeId, s, r, a, c.shard, c.realm, c.account)
		}
	}
}
