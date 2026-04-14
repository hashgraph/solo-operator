// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
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

func buildReconcilerForUpdateStatus(c client.Client, hc HealthChecker) *ConsensusCapsuleReconciler {
	scheme := newControllerTestScheme()
	r := NewConsensusCapsuleReconciler(c, scheme)
	r.healthChecker = hc
	return r
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

	r := buildReconcilerForUpdateStatus(c, &stubHealthChecker{})

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

func TestUpdateStatus_PodRunning_HealthPass_PhaseRunning(t *testing.T) {
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

	r := buildReconcilerForUpdateStatus(c, &stubHealthChecker{}) // both checks pass

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

func TestUpdateStatus_PodRunning_HealthFail_StaysPending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newPodWithPhase(capsule, corev1.PodRunning)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c, &stubHealthChecker{dnsErr: errors.New("no such host")})

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != healthCheckRequeueAfter {
		t.Errorf("expected requeue after %v, got %v", healthCheckRequeueAfter, result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending when health checks fail, got %s", capsule.Status.Phase)
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

	r := buildReconcilerForUpdateStatus(c, &stubHealthChecker{})

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

	r := buildReconcilerForUpdateStatus(c, &stubHealthChecker{})

	// No pods → phase stays Pending; ObservedGeneration matches → early return, no Status().Update()
	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected empty result for no-op update, got %+v", result)
	}
}
