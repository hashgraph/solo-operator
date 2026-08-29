// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

type mockS6Runner struct {
	isUpVal   bool
	isUpErr   error
	startErr  error
	stopErr   error
	startHits int
	stopHits  int
	isUpHits  int
}

func (m *mockS6Runner) Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	m.startHits++
	return m.startErr
}

func (m *mockS6Runner) Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	m.stopHits++
	return m.stopErr
}

func (m *mockS6Runner) IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	m.isUpHits++
	return m.isUpVal, m.isUpErr
}

func TestActuationSubReconciler_ShouldReconcile(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	makePod := func(name, ns string, ready bool) *corev1.Pod {
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: status,
					},
				},
			},
		}
	}

	t.Run("s6 is nil", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		c := newTestClient(scheme, capsule)
		r := NewActuationSubReconciler(c, scheme, nil, nil)
		if r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=false when s6 is nil")
		}
	})

	t.Run("StartPolicy is Auto and DesiredRunState is empty", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyAuto
		c := newTestClient(scheme, capsule)
		r := NewActuationSubReconciler(c, scheme, &mockS6Runner{}, nil)
		if r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=false when StartPolicy is Auto and DesiredRunState is empty")
		}
	})

	t.Run("StartPolicy is Auto and DesiredRunState is Stopped, pod ready", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyAuto
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped
		pod := makePod(PodName(capsule), capsule.Namespace, true)
		c := newTestClient(scheme, capsule, pod)
		r := NewActuationSubReconciler(c, scheme, &mockS6Runner{}, nil)
		if !r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=true when StartPolicy is Auto, DesiredRunState is Stopped, and pod is ready")
		}
	})

	t.Run("pod does not exist", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		c := newTestClient(scheme, capsule)
		r := NewActuationSubReconciler(c, scheme, &mockS6Runner{}, nil)
		if r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=false when pod does not exist")
		}
	})

	t.Run("pod not ready", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		pod := makePod(PodName(capsule), capsule.Namespace, false)
		c := newTestClient(scheme, capsule, pod)
		r := NewActuationSubReconciler(c, scheme, &mockS6Runner{}, nil)
		if r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=false when pod is not ready")
		}
	})

	t.Run("pod is ready and StartPolicy is Manual", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		pod := makePod(PodName(capsule), capsule.Namespace, true)
		c := newTestClient(scheme, capsule, pod)
		r := NewActuationSubReconciler(c, scheme, &mockS6Runner{}, nil)
		if !r.ShouldReconcile(ctx, capsule) {
			t.Error("expected ShouldReconcile=true when pod is ready and StartPolicy=Manual")
		}
	})
}

func TestActuationSubReconciler_Reconcile(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	recorder := events.NewFakeRecorder(10)

	t.Run("desired Running and currently down -> calls Start", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning
		c := newTestClient(scheme, capsule)
		runner := &mockS6Runner{isUpVal: false}
		r := NewActuationSubReconciler(c, scheme, runner, recorder)

		res, err := r.Reconcile(ctx, capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Requeue {
			t.Error("unexpected requeue")
		}
		if runner.startHits != 1 {
			t.Errorf("expected 1 Start call, got %d", runner.startHits)
		}
		if runner.stopHits != 0 {
			t.Errorf("expected 0 Stop calls, got %d", runner.stopHits)
		}
	})

	t.Run("desired Running and already up -> no Start call", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning
		c := newTestClient(scheme, capsule)
		runner := &mockS6Runner{isUpVal: true}
		r := NewActuationSubReconciler(c, scheme, runner, recorder)

		res, err := r.Reconcile(ctx, capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Requeue {
			t.Error("unexpected requeue")
		}
		if runner.startHits != 0 {
			t.Errorf("expected 0 Start calls, got %d", runner.startHits)
		}
	})

	t.Run("desired Stopped and currently up (Auto policy) -> calls Stop", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyAuto
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped
		c := newTestClient(scheme, capsule)
		runner := &mockS6Runner{isUpVal: true}
		r := NewActuationSubReconciler(c, scheme, runner, recorder)

		res, err := r.Reconcile(ctx, capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Requeue {
			t.Error("unexpected requeue")
		}
		if runner.stopHits != 1 {
			t.Errorf("expected 1 Stop call, got %d", runner.stopHits)
		}
		if runner.startHits != 0 {
			t.Errorf("expected 0 Start calls, got %d", runner.startHits)
		}
	})

	t.Run("desired Stopped and already down -> no Stop call", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped
		c := newTestClient(scheme, capsule)
		runner := &mockS6Runner{isUpVal: false}
		r := NewActuationSubReconciler(c, scheme, runner, recorder)

		res, err := r.Reconcile(ctx, capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Requeue {
			t.Error("unexpected requeue")
		}
		if runner.stopHits != 0 {
			t.Errorf("expected 0 Stop calls, got %d", runner.stopHits)
		}
	})

	t.Run("IsUp error propagated", func(t *testing.T) {
		capsule := newTestCapsule()
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning
		c := newTestClient(scheme, capsule)
		expectedErr := errors.New("exec error")
		runner := &mockS6Runner{isUpErr: expectedErr}
		r := NewActuationSubReconciler(c, scheme, runner, recorder)

		_, err := r.Reconcile(ctx, capsule)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("nil s6 is no-op", func(t *testing.T) {
		capsule := newTestCapsule()
		c := newTestClient(scheme, capsule)
		r := NewActuationSubReconciler(c, scheme, nil, recorder)

		res, err := r.Reconcile(ctx, capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Requeue {
			t.Error("unexpected requeue")
		}
	})
}
