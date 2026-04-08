// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected Failed phase, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, "Ready")
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

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, got)

	cond := findCondition(got.Status.Conditions, "Ready")
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

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, got)

	cond := findCondition(got.Status.Conditions, "Ready")
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

	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected Succeeded, got %s", got.Status.Phase)
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

// ─── Phase lifecycle ──────────────────────────────────────────────────────────

func TestUpdateStatus_setsPhaseRunningForPendingRelease(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	r := &HelmCapsuleReconciler{Client: c, Scheme: scheme}

	// Persist ReleaseStatus so the re-fetch inside updateStatus sees it.
	hc.Status.ReleaseStatus = "pending-install"
	if err := c.Status().Update(context.Background(), hc); err != nil {
		t.Fatalf("seeding status: %v", err)
	}
	if err := r.updateStatus(context.Background(), hc); err != nil {
		t.Fatalf("updateStatus error: %v", err)
	}

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning for pending-install, got %s", got.Status.Phase)
	}
}

func TestUpdateStatus_setsPhaseSucceededForDeployedRelease(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	r := &HelmCapsuleReconciler{Client: c, Scheme: scheme}

	// Persist ReleaseStatus so the re-fetch inside updateStatus sees it.
	hc.Status.ReleaseStatus = "deployed"
	if err := c.Status().Update(context.Background(), hc); err != nil {
		t.Fatalf("seeding status: %v", err)
	}
	if err := r.updateStatus(context.Background(), hc); err != nil {
		t.Fatalf("updateStatus error: %v", err)
	}

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded for deployed release, got %s", got.Status.Phase)
	}
}

func TestReconcile_phaseSucceededAfterIdleRequeue(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	r := NewHelmCapsuleReconciler(c, scheme, mock)

	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"}}

	// reconcile 1: finalizer
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile 1 error: %v", err)
	}
	// reconcile 2: install
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile 2 error: %v", err)
	}
	// reconcile 3: idle (hash matches, release deployed → RequeueAfter)
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("idle reconcile error: %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Error("expected RequeueAfter > 0 on idle path")
	}

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded after idle requeue, got %s", got.Status.Phase)
	}
}

// ─── PhasePending on first contact ────────────────────────────────────────────

func TestReconcile_setsPhasePendingOnFirstContact(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	r := NewHelmCapsuleReconciler(c, scheme, mock)

	// First reconcile: adds finalizer and sets PhasePending.
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)

	if got.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending after finalizer add, got %s", got.Status.Phase)
	}
}

// ─── Deletion / escape hatch ──────────────────────────────────────────────────

func TestDeletion_basicPath_callsUninstallAndRemovesFinalizer(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	now := metav1.Now()
	hc.DeletionTimestamp = &now
	hc.Finalizers = []string{HelmCapsuleFinalizer}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	uninstallCalled := false
	var capturedRelease, capturedNamespace string
	mock.UninstallFunc = func(_ context.Context, releaseName, namespace string, _ helm.UninstallOptions) error {
		uninstallCalled = true
		capturedRelease = releaseName
		capturedNamespace = namespace
		return nil
	}

	r := NewHelmCapsuleReconciler(c, scheme, mock)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("deletion reconcile error: %v", err)
	}
	if !uninstallCalled {
		t.Fatal("expected Helm uninstall to be called during deletion")
	}
	if capturedRelease != "mirror" || capturedNamespace != "test-ns" {
		t.Errorf("unexpected uninstall target: release=%s namespace=%s", capturedRelease, capturedNamespace)
	}

	// Finalizer must be removed.
	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)
	for _, f := range got.Finalizers {
		if f == HelmCapsuleFinalizer {
			t.Error("expected finalizer to be removed after successful uninstall")
		}
	}
}

func TestDeletion_honorsSpecTimeoutAndOptions(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	now := metav1.Now()
	hc.DeletionTimestamp = &now
	hc.Finalizers = []string{HelmCapsuleFinalizer}
	hc.Spec.KeepHistory = true
	hc.Spec.DisableHooks = true
	hc.Spec.Wait = true
	hc.Spec.Timeout = &metav1.Duration{Duration: 10 * time.Minute}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	var capturedOpts helm.UninstallOptions
	mock.UninstallFunc = func(_ context.Context, _, _ string, opts helm.UninstallOptions) error {
		capturedOpts = opts
		return nil
	}

	r := NewHelmCapsuleReconciler(c, scheme, mock)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("deletion reconcile error: %v", err)
	}

	if capturedOpts.Timeout != 10*time.Minute {
		t.Errorf("expected timeout=10m, got %v", capturedOpts.Timeout)
	}
	if !capturedOpts.KeepHistory {
		t.Error("expected KeepHistory=true")
	}
	if !capturedOpts.DisableHooks {
		t.Error("expected DisableHooks=true")
	}
	if !capturedOpts.Wait {
		t.Error("expected Wait=true")
	}
}

func TestDeletion_skipUninstallAnnotationBypassesHelm(t *testing.T) {
	scheme := newControllerScheme()
	hc := newHC("mirror", "test-ns")

	now := metav1.Now()
	hc.DeletionTimestamp = &now
	hc.Finalizers = []string{HelmCapsuleFinalizer}
	hc.Annotations = map[string]string{SkipUninstallAnnotation: "true"}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(hc).
		WithStatusSubresource(hc).
		Build()

	mock := helm.NewMockClient()
	uninstallCalled := false
	mock.UninstallFunc = func(_ context.Context, _, _ string, _ helm.UninstallOptions) error {
		uninstallCalled = true
		return nil
	}

	r := NewHelmCapsuleReconciler(c, scheme, mock)
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "mirror", Namespace: "test-ns"},
	})
	if err != nil {
		t.Fatalf("deletion reconcile error: %v", err)
	}
	if uninstallCalled {
		t.Error("expected Helm uninstall to be skipped when escape-hatch annotation is set")
	}

	// Finalizer must be removed so deletion can proceed.
	got := &operatorv1alpha1.HelmCapsule{}
	_ = c.Get(context.Background(), types.NamespacedName{Name: "mirror", Namespace: "test-ns"}, got)
	for _, f := range got.Finalizers {
		if f == HelmCapsuleFinalizer {
			t.Error("expected finalizer to be removed after escape-hatch uninstall skip")
		}
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
