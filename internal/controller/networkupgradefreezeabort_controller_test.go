// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func newAbortCR(opID string) *operatorv1alpha1.NetworkUpgradeFreezeAbort {
	return &operatorv1alpha1.NetworkUpgradeFreezeAbort{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "abrt-" + opID,
			Namespace: "ns",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepAbort},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeAbortSpec{
			Orbit:       "ns",
			OperationID: opID,
		},
	}
}

func runAbortReconcile(t *testing.T, r *NetworkUpgradeFreezeAbortReconciler, name, namespace string) {
	t.Helper()
	for i := 0; i < 5; i++ {
		res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: name, Namespace: namespace}})
		if err != nil {
			t.Fatalf("reconcile #%d: %v", i, err)
		}
		if !res.Requeue && res.RequeueAfter == 0 {
			return
		}
	}
	t.Fatalf("abort reconcile did not converge")
}

func TestFreezeAbortReconciler_NoExecute_MarksProcessed(t *testing.T) {
	scheme := newControllerTestScheme()
	abort := newAbortCR("op1")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(abort).WithStatusSubresource(abort).Build()

	r := &NetworkUpgradeFreezeAbortReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(8),
	}
	runAbortReconcile(t, r, abort.Name, abort.Namespace)

	got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: abort.Name, Namespace: abort.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	cond := findUpgradeCondition(got.Status.Conditions, operatorv1alpha1.CondAbortProcessed)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != operatorv1alpha1.ReasonAbortSuccess {
		t.Fatalf("expected AbortProcessed=True/Success, got %+v", cond)
	}
}

func TestFreezeAbortReconciler_ExecuteExists_MarksTooLate(t *testing.T) {
	scheme := newControllerTestScheme()
	abort := newAbortCR("op2")
	exec := &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exec-op2",
			Namespace: "ns",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op2"},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{OperationID: "op2", Orbit: "ns"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(abort, exec).WithStatusSubresource(abort, exec).Build()

	r := &NetworkUpgradeFreezeAbortReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(8),
	}
	runAbortReconcile(t, r, abort.Name, abort.Namespace)

	got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: abort.Name, Namespace: abort.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q want Failed", got.Status.Phase)
	}
	cond := findUpgradeCondition(got.Status.Conditions, operatorv1alpha1.CondAbortProcessed)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != operatorv1alpha1.ReasonAbortAfterFreeze {
		t.Fatalf("expected AbortProcessed=False/AbortAfterFreeze, got %+v", cond)
	}

	// Prior CRs should not be deleted by abort processing — verify Execute still exists.
	gotExec := &operatorv1alpha1.NetworkUpgradeExecute{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, gotExec); err != nil {
		t.Fatalf("expected Execute CR to still exist after abort: %v", err)
	}
}

func findUpgradeCondition(conds []metav1.Condition, condType string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == condType {
			return &conds[i]
		}
	}
	return nil
}
