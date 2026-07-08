// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func newFreezeAbortForTest() *operatorv1alpha1.NetworkUpgradeFreezeAbort {
	return &operatorv1alpha1.NetworkUpgradeFreezeAbort{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-abort",
			Namespace: "default",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeAbortSpec{
			Orbit:       "test-orbit",
			OperationID: "op-001",
		},
	}
}

func newExecuteForSameOp() *operatorv1alpha1.NetworkUpgradeExecute {
	return &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-execute",
			Namespace: "default",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:       "test-orbit",
			OperationID: "op-001",
		},
	}
}

type abortReconcileResult struct {
	events []string
	abort  *operatorv1alpha1.NetworkUpgradeFreezeAbort
	err    error
}

func executeIndexFunc(o client.Object) []string {
	return []string{o.(*operatorv1alpha1.NetworkUpgradeExecute).Spec.OperationID}
}

func abortIndexFunc(o client.Object) []string {
	return []string{o.(*operatorv1alpha1.NetworkUpgradeFreezeAbort).Spec.OperationID}
}

func reconcileFreezeAbort(t *testing.T, abort *operatorv1alpha1.NetworkUpgradeFreezeAbort, execute *operatorv1alpha1.NetworkUpgradeExecute) abortReconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()

	builder := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&operatorv1alpha1.NetworkUpgradeExecute{}, fieldOperationID, executeIndexFunc).
		WithIndex(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}, fieldOperationID, abortIndexFunc).
		WithObjects(abort).
		WithStatusSubresource(abort)
	if execute != nil {
		builder = builder.WithObjects(execute)
	}
	c := builder.Build()

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeFreezeAbortReconciler(c, scheme)
	r.Recorder = rec

	var err error
	for i := 0; i < 5; i++ {
		var res ctrl.Result
		res, err = r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(abort),
		})
		if !res.Requeue && res.RequeueAfter == 0 {
			break
		}
	}

	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}

	var updated operatorv1alpha1.NetworkUpgradeFreezeAbort
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(abort), &updated); getErr != nil {
		t.Fatalf("get updated abort: %v", getErr)
	}

	return abortReconcileResult{events: recorded, abort: &updated, err: err}
}

func TestFreezeAbortReconcile_NoExecuteCR(t *testing.T) {
	res := reconcileFreezeAbort(t, newFreezeAbortForTest(), nil)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.abort.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected phase Succeeded, got %s", res.abort.Status.Phase)
	}
	if res.abort.Status.StartTime == nil {
		t.Error("expected StartTime to be set")
	}
	if res.abort.Status.CompletionTime == nil {
		t.Error("expected CompletionTime to be set")
	}
	if got := countContaining(res.events, operatorv1alpha1.EventAbortProcessed); got != 1 {
		t.Errorf("expected one AbortProcessed event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.abort.Status.Conditions, operatorv1alpha1.CondAbortProcessed)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected CondAbortProcessed=True, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonAbortSuccess {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonAbortSuccess, cond.Reason)
	}
}

func TestFreezeAbortReconcile_ExecuteCRExists(t *testing.T) {
	res := reconcileFreezeAbort(t, newFreezeAbortForTest(), newExecuteForSameOp())

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.abort.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.abort.Status.Phase)
	}
	if res.abort.Status.CompletionTime == nil {
		t.Error("expected CompletionTime to be set")
	}
	if got := countContaining(res.events, operatorv1alpha1.EventAbortAfterFreeze); got != 1 {
		t.Errorf("expected one AbortAfterFreeze warning event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.abort.Status.Conditions, operatorv1alpha1.CondAbortProcessed)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondAbortProcessed=False, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonAbortAfterFreeze {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonAbortAfterFreeze, cond.Reason)
	}
}

func TestFreezeAbortReconcile_OperationIDLabelMissing(t *testing.T) {
	abort := newFreezeAbortForTest()
	abort.Labels = nil

	res := reconcileFreezeAbort(t, abort, nil)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.abort.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.abort.Status.Phase)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventOperationIDLabelInvalid); got != 1 {
		t.Errorf("expected one OperationIDLabelInvalid event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.abort.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondOperationIDLabelValid=False, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonOperationIDLabelInvalid, cond.Reason)
	}
	// Invalid identity must not advance: no abort-processed side effects.
	if got := countContaining(res.events, operatorv1alpha1.EventAbortProcessed); got != 0 {
		t.Errorf("invalid CR must not process abort, got %d AbortProcessed events", got)
	}
}

func TestFreezeAbortReconcile_OperationIDLabelMismatch(t *testing.T) {
	abort := newFreezeAbortForTest()
	abort.Labels[operatorv1alpha1.LabelOperationID] = "op-does-not-match"

	res := reconcileFreezeAbort(t, abort, nil)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.abort.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.abort.Status.Phase)
	}
	cond := findCondition(res.abort.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid {
		t.Errorf("expected reason %s, got %v", operatorv1alpha1.ReasonOperationIDLabelInvalid, cond)
	}
}

func TestFreezeAbortReconcile_DuplicateOperationID_SetsPhaseFailed(t *testing.T) {
	existing := newFreezeAbortForTest()
	existing.Name = "existing"
	existing.UID = "uid-existing"

	abort := newFreezeAbortForTest()
	abort.Name = "duplicate"
	abort.UID = "uid-duplicate"

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&operatorv1alpha1.NetworkUpgradeExecute{}, fieldOperationID, executeIndexFunc).
		WithIndex(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}, fieldOperationID, abortIndexFunc).
		WithObjects(existing, abort).
		WithStatusSubresource(existing, abort).
		Build()

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeFreezeAbortReconciler(c, scheme)
	r.Recorder = rec

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(abort),
	})
	if err == nil {
		t.Error("expected error for duplicate operationId")
	}

	var got operatorv1alpha1.NetworkUpgradeFreezeAbort
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(abort), &got); getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, operatorv1alpha1.CondAbortProcessed)
	if cond == nil {
		t.Fatal("CondAbortProcessed condition not set")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondAbortProcessed=False, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.ReasonDuplicateOpID {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonDuplicateOpID, cond.Reason)
	}
	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}
	if got := countContaining(recorded, "ManifestInvalid"); got != 1 {
		t.Errorf("expected one ManifestInvalid event, got %d in %v", got, recorded)
	}
}

func TestFreezeAbortReconcile_TerminalPhaseIsNoOp(t *testing.T) {
	for _, phase := range []operatorv1alpha1.Phase{
		operatorv1alpha1.PhaseSucceeded,
		operatorv1alpha1.PhaseFailed,
	} {
		t.Run(string(phase), func(t *testing.T) {
			abort := newFreezeAbortForTest()

			scheme := newControllerTestScheme()
			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&operatorv1alpha1.NetworkUpgradeExecute{}, fieldOperationID, executeIndexFunc).
				WithIndex(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}, fieldOperationID, abortIndexFunc).
				WithObjects(abort).
				WithStatusSubresource(abort).
				Build()
			abort.Status.Phase = phase
			if err := c.Status().Update(context.Background(), abort); err != nil {
				t.Fatalf("pre-set status: %v", err)
			}

			rec := events.NewFakeRecorder(16)
			r := NewNetworkUpgradeFreezeAbortReconciler(c, scheme)
			r.Recorder = rec

			if _, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(abort),
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			close(rec.Events)
			var recorded []string
			for e := range rec.Events {
				recorded = append(recorded, e)
			}
			if len(recorded) != 0 {
				t.Errorf("terminal CR should emit no events, got %v", recorded)
			}
		})
	}
}
