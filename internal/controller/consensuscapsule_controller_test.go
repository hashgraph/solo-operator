// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

func newControllerTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	return scheme
}

func newCapsuleForControllerTest() *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node0",
			Namespace:  "test",
			Generation: 1,
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
		},
	}
}

func newPodWithPhase(capsule *operatorv1alpha1.ConsensusCapsule, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      capsule.Name + "-0",
			Namespace: capsule.Namespace,
			Labels:    consensus.PodLabels(capsule),
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func newReadyPodWithPhase(capsule *operatorv1alpha1.ConsensusCapsule, phase corev1.PodPhase) *corev1.Pod {
	pod := newPodWithPhase(capsule, phase)
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	return pod
}

func buildReconcilerForUpdateStatus(c client.Client) *ConsensusCapsuleReconciler {
	scheme := newControllerTestScheme()
	return NewConsensusCapsuleReconciler(c, scheme)
}

func TestUpdateStatus_NoPods_PhasePending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule).
		WithStatusSubresource(capsule).
		Build()
	// re-get to pick up the ResourceVersion assigned by the fake store
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_PodRunning_Ready_PhaseRunning(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_PodRunning_NotReady_StaysPending verifies that a Running pod
// whose Ready condition is not yet True keeps the capsule in PhasePending and
// schedules a requeue so the controller re-checks readiness.
func TestUpdateStatus_PodRunning_NotReady_StaysPending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newPodWithPhase(capsule, corev1.PodRunning) // no Ready condition
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != podReadinessRequeueAfter {
		t.Errorf("expected requeue after %v, got %v", podReadinessRequeueAfter, result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending when pod not ready, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_PodFailed_PhaseFailed(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newPodWithPhase(capsule, corev1.PodFailed)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", capsule.Status.Phase)
	}
}

// ── markGenesisInitialized ────────────────────────────────────────────────────

func newCheckpointCM(capsule *operatorv1alpha1.ConsensusCapsule, value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consensus.CheckpointConfigMapName(capsule),
			Namespace: capsule.Namespace,
		},
		Data: map[string]string{
			consensus.CheckpointKeyGenesisInitialized: value,
		},
	}
}

func TestMarkGenesisInitialized_PatchesCheckpoint(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule, cm).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueTrue {
		t.Errorf("expected genesis-initialized=true after patch, got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

func TestMarkGenesisInitialized_Idempotent(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	cm := newCheckpointCM(capsule, consensus.CheckpointValueTrue)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule, cm).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("unexpected error on already-true checkpoint: %v", err)
	}
}

func TestMarkGenesisInitialized_CheckpointMissing(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	// No checkpoint ConfigMap in the store — should be a no-op, not an error.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("expected nil when checkpoint is missing, got: %v", err)
	}
}

func TestUpdateStatus_PodRunning_PatchesCheckpoint(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, cm).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueTrue {
		t.Errorf("expected checkpoint patched to true when pod running, got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

func TestUpdateStatus_NoChangeWhenPhaseUnchanged(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Status.Phase = operatorv1alpha1.PhasePending
	capsule.Status.ObservedGeneration = 1
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	// No pods → phase stays Pending; ObservedGeneration matches → early return, no Status().Update()
	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected empty result for no-op update, got %+v", result)
	}
}
