// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
)

func newControllerScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

func newHC(name, namespace string) *operatorv1alpha1.HelmCapsule {
	return &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart: operatorv1alpha1.HelmChartSpec{
				Repo:    "oci://ghcr.io/hashgraph",
				Chart:   "hedera-mirror",
				Version: "0.129.1",
			},
		},
	}
}

// ─── setCondition ─────────────────────────────────────────────────────────────

func TestSetCondition_addsNew(t *testing.T) {
	var conds []operatorv1alpha1.Condition
	setCondition(&conds, metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
		Reason: "Deployed",
	})
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Status != metav1.ConditionTrue {
		t.Errorf("expected True, got %s", conds[0].Status)
	}
}

func TestSetCondition_updatesExisting(t *testing.T) {
	var conds []operatorv1alpha1.Condition
	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed"})
	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "HelmFailed"})

	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(conds))
	}
	if conds[0].Status != metav1.ConditionFalse {
		t.Errorf("expected False, got %s", conds[0].Status)
	}
	if conds[0].Reason != "HelmFailed" {
		t.Errorf("expected reason HelmFailed, got %s", conds[0].Reason)
	}
}

func TestSetCondition_preservesTransitionTimeWhenStatusUnchanged(t *testing.T) {
	var conds []operatorv1alpha1.Condition
	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed"})
	original := conds[0].LastTransitionTime

	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed", Message: "updated message"})

	if !conds[0].LastTransitionTime.Equal(&original) {
		t.Error("LastTransitionTime should not change when Status stays the same")
	}
}

func TestSetCondition_updatesTransitionTimeOnStatusChange(t *testing.T) {
	var conds []operatorv1alpha1.Condition
	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed"})
	original := conds[0].LastTransitionTime

	setCondition(&conds, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "HelmFailed"})

	if conds[0].LastTransitionTime.Equal(&original) {
		t.Error("LastTransitionTime should change when Status changes")
	}
}

// ─── recordFailure ────────────────────────────────────────────────────────────

func TestRecordFailure_setsChartFetchReason(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	r := &HelmCapsuleReconciler{Client: c, Scheme: scheme}
	r.recordFailure(context.Background(), hc, fmt.Errorf("%w: repo down", helm.ErrChartFetch))

	if hc.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected Failed phase, got %s", hc.Status.Phase)
	}
	cond := findCondition(hc.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatal("expected Ready condition")
	}
	if cond.Reason != operatorv1alpha1.HelmCapsuleReasonChartFetchFailed {
		t.Errorf("expected ChartFetchFailed, got %s", cond.Reason)
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected False, got %s", cond.Status)
	}
}

func TestRecordFailure_setsValuesBuildReason(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	r := &HelmCapsuleReconciler{Client: c, Scheme: scheme}
	r.recordFailure(context.Background(), hc, fmt.Errorf("%w: missing configmap", helm.ErrValuesBuild))

	cond := findCondition(hc.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatal("expected Ready condition")
	}
	if cond.Reason != operatorv1alpha1.HelmCapsuleReasonValuesBuildFailed {
		t.Errorf("expected ValuesBuildFailed, got %s", cond.Reason)
	}
}

func TestRecordFailure_setsGenericHelmReason(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	r := &HelmCapsuleReconciler{Client: c, Scheme: scheme}
	r.recordFailure(context.Background(), hc, errors.New("hook timed out"))

	cond := findCondition(hc.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatal("expected Ready condition")
	}
	if cond.Reason != operatorv1alpha1.HelmCapsuleReasonHelmFailed {
		t.Errorf("expected HelmFailed, got %s", cond.Reason)
	}
}

// ─── Reconcile — success path ─────────────────────────────────────────────────

func TestReconcile_setsReadyTrueOnSuccess(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	r := NewHelmCapsuleReconciler(c, scheme, mock)

	// First reconcile adds finalizer and requeues.
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	// Second reconcile runs the full chain.
	_, err = r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected Running, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatal("expected Ready condition after successful reconcile")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected Ready=True, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.HelmCapsuleReasonDeployed {
		t.Errorf("expected reason Deployed, got %s", cond.Reason)
	}
}

func TestReconcile_setsReadyFalseOnHelmError(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, _, _ string, _ operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ helm.ReleaseOptions) (*helm.ReleaseInfo, error) {
		return nil, fmt.Errorf("%w: registry.ghcr.io: connection refused", helm.ErrChartFetch)
	}

	r := NewHelmCapsuleReconciler(c, scheme, mock)

	// First reconcile: add finalizer.
	_, _ = r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	// Second reconcile: helm call fails.
	_, _ = r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected Failed, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatal("expected Ready condition")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected Ready=False, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.HelmCapsuleReasonChartFetchFailed {
		t.Errorf("expected ChartFetchFailed, got %s", cond.Reason)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func findCondition(conds []operatorv1alpha1.Condition, condType string) *operatorv1alpha1.Condition {
	for i := range conds {
		if conds[i].Type == condType {
			return &conds[i]
		}
	}
	return nil
}
