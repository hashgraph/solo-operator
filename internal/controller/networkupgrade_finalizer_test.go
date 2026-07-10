// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestEnsureFinalizer_SkipsDeletingObject(t *testing.T) {
	now := metav1.Now()
	obj := &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"other/keep-me"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).WithObjects(obj).Build()

	res, err := ensureFinalizer(context.Background(), c, obj, FinalizerNetworkUpgradePrepare)
	if err != nil {
		t.Fatalf("expected no error for deleting object, got %v", err)
	}
	if res == nil {
		t.Fatal("expected short-circuit result for deleting object, got nil")
	}
	if controllerutil.ContainsFinalizer(obj, FinalizerNetworkUpgradePrepare) {
		t.Error("finalizer must not be re-added to a deleting object")
	}
}

func TestPrepareReconcile_AddsFinalizer(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))
	if !controllerutil.ContainsFinalizer(res.prepare, FinalizerNetworkUpgradePrepare) {
		t.Errorf("expected finalizer %q on prepare, got %v", FinalizerNetworkUpgradePrepare, res.prepare.Finalizers)
	}
}

func TestFreezeReconcile_AddsFinalizer(t *testing.T) {
	res := reconcileFreeze(t, newFreezeForTest(), newSucceededPrepareForFreeze(), newFreezeChecker(nil, nil))
	if !controllerutil.ContainsFinalizer(res.freeze, FinalizerNetworkUpgradeFreeze) {
		t.Errorf("expected finalizer %q on freeze, got %v", FinalizerNetworkUpgradeFreeze, res.freeze.Finalizers)
	}
}

func TestExecuteReconcile_AddsFinalizer(t *testing.T) {
	scheme := newControllerTestScheme()
	execute := newExecuteForTest()
	execute.Finalizers = nil // verify the reconciler stamps it from scratch
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute).
		WithStatusSubresource(execute)).
		Build()

	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got operatorv1alpha1.NetworkUpgradeExecute
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !controllerutil.ContainsFinalizer(&got, FinalizerNetworkUpgradeExecute) {
		t.Errorf("expected finalizer %q on execute, got %v", FinalizerNetworkUpgradeExecute, got.Finalizers)
	}
}

func TestFreezeAbortReconcile_AddsFinalizer(t *testing.T) {
	res := reconcileFreezeAbort(t, newFreezeAbortForTest(), nil)
	if !controllerutil.ContainsFinalizer(res.abort, FinalizerNetworkUpgradeFreezeAbort) {
		t.Errorf("expected finalizer %q on abort, got %v", FinalizerNetworkUpgradeFreezeAbort, res.abort.Finalizers)
	}
}
