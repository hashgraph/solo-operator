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
	"github.com/hashgraph/solo-operator/internal/registry"
)

const (
	freezeDigest    = operatorv1alpha1.Sha256Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	altFreezeDigest = operatorv1alpha1.Sha256Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
)

func newFreezeChecker(unreachable, mismatch map[string]bool) fakeUpgradeChecker {
	return fakeUpgradeChecker{unreachable: unreachable, mismatch: mismatch, defaultDigest: freezeDigest}
}

func newFreezeForTest() *operatorv1alpha1.NetworkUpgradeFreeze {
	return &operatorv1alpha1.NetworkUpgradeFreeze{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-freeze",
			Namespace: "default",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
			Orbit:         "test-orbit",
			OperationID:   "op-001",
			ScheduledTime: metav1.Now(),
			Images: operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: registriesFor("reg-a/cn:0.75.0", "reg-b/cn:0.75.0"),
				},
			},
			SelectedSources: operatorv1alpha1.SelectedSources{
				ConsensusNode: &operatorv1alpha1.SelectedSource{
					Image:               "reg-a/cn:0.75.0",
					Digest:              freezeDigest,
					LayerHashesVerified: true,
				},
			},
		},
	}
}

func newSucceededPrepareForFreeze() *operatorv1alpha1.NetworkUpgradePrepare {
	return &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prepare",
			Namespace: "default",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
		},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			Orbit:       "test-orbit",
			OperationID: "op-001",
		},
		Status: operatorv1alpha1.NetworkUpgradePrepareStatus{
			Phase: operatorv1alpha1.PhaseSucceeded,
		},
	}
}

type freezeReconcileResult struct {
	events []string
	freeze *operatorv1alpha1.NetworkUpgradeFreeze
	err    error
}

func reconcileFreeze(t *testing.T, freeze *operatorv1alpha1.NetworkUpgradeFreeze, prepare *operatorv1alpha1.NetworkUpgradePrepare, checker registry.ImageChecker) freezeReconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()

	builder := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.NetworkUpgradeFreeze{}, fieldOperationID, func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreeze).Spec.OperationID}
		}).
		WithObjects(freeze).
		WithStatusSubresource(freeze)
	if prepare != nil {
		builder = builder.WithObjects(prepare).WithStatusSubresource(prepare)
	}
	c := builder.Build()

	// Pre-seed prepare status since fake client doesn't persist status from WithObjects.
	if prepare != nil {
		if err := c.Status().Update(context.Background(), prepare); err != nil {
			t.Fatalf("pre-seed prepare status: %v", err)
		}
	}

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeFreezeReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	var err error
	for i := 0; i < 5; i++ {
		var res ctrl.Result
		res, err = r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(freeze),
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

	var updated operatorv1alpha1.NetworkUpgradeFreeze
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(freeze), &updated); getErr != nil {
		t.Fatalf("get updated freeze: %v", getErr)
	}

	return freezeReconcileResult{events: recorded, freeze: &updated, err: err}
}

func TestFreezeReconcile_OperationIDLabelMissing(t *testing.T) {
	freeze := newFreezeForTest()
	freeze.Labels = nil

	// prepare=nil: label validation runs before the prepare prereq wait,
	// so an invalid CR fails terminally regardless of prerequisites.
	res := reconcileFreeze(t, freeze, nil, nil)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.freeze.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.freeze.Status.Phase)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventOperationIDLabelInvalid); got != 1 {
		t.Errorf("expected one OperationIDLabelInvalid event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondOperationIDLabelValid=False, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonOperationIDLabelInvalid, cond.Reason)
	}
}

func TestFreezeReconcile_OperationIDLabelMismatch(t *testing.T) {
	freeze := newFreezeForTest()
	freeze.Labels[operatorv1alpha1.LabelOperationID] = "op-does-not-match"

	res := reconcileFreeze(t, freeze, nil, nil)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.freeze.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.freeze.Status.Phase)
	}
	cond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid {
		t.Errorf("expected reason %s, got %v", operatorv1alpha1.ReasonOperationIDLabelInvalid, cond)
	}
}

func assertFreezeWaitsForPrepare(t *testing.T, res freezeReconcileResult) {
	t.Helper()
	if res.err != nil {
		t.Fatalf("expected no error while waiting for prepare, got %v", res.err)
	}
	if res.freeze.Status.Phase == operatorv1alpha1.PhaseSucceeded || res.freeze.Status.Phase == operatorv1alpha1.PhaseFailed {
		t.Errorf("expected non-terminal phase while waiting, got %s", res.freeze.Status.Phase)
	}
}

func TestFreezeReconcile_PrepareNotFound(t *testing.T) {
	assertFreezeWaitsForPrepare(t, reconcileFreeze(t, newFreezeForTest(), nil, newFreezeChecker(nil, nil)))
}

func TestFreezeReconcile_PrepareNotSucceeded(t *testing.T) {
	prepare := newSucceededPrepareForFreeze()
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	assertFreezeWaitsForPrepare(t, reconcileFreeze(t, newFreezeForTest(), prepare, newFreezeChecker(nil, nil)))
}

func TestFreezeReconcile_SelectedRegistryStillValid(t *testing.T) {
	res := reconcileFreeze(t, newFreezeForTest(), newSucceededPrepareForFreeze(), newFreezeChecker(nil, nil))

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.freeze.Status.StartTime == nil {
		t.Error("expected StartTime to be set")
	}
	if got := countContaining(res.events, "RegistryFailover"); got != 0 {
		t.Errorf("expected no failover events, got %d", got)
	}
	if res.freeze.Status.SelectedSources == nil || res.freeze.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources to be populated in status")
	}
	if res.freeze.Status.SelectedSources.ConsensusNode.Image != "reg-a/cn:0.75.0" {
		t.Errorf("expected original image to be preserved, got %s", res.freeze.Status.SelectedSources.ConsensusNode.Image)
	}
	cond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondRegistryHealthCheck)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected RegistryHealthCheck=True, got %v", cond)
	}
	rfCond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)
	if rfCond == nil || rfCond.Status != metav1.ConditionTrue {
		t.Errorf("expected ReadyForFreeze=True, got %v", rfCond)
	}
}

func TestFreezeReconcile_FailoverToAlternate(t *testing.T) {
	checker := fakeUpgradeChecker{
		unreachable:   map[string]bool{"reg-a/cn:0.75.0": true},
		digest:        altFreezeDigest,
		defaultDigest: freezeDigest,
	}
	res := reconcileFreeze(t, newFreezeForTest(), newSucceededPrepareForFreeze(), checker)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countContaining(res.events, "RegistryFailover"); got != 1 {
		t.Errorf("expected one RegistryFailover event, got %d in %v", got, res.events)
	}
	if got := countContaining(res.events, "reg-a/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the failed registry")
	}
	if got := countContaining(res.events, "reg-b/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the replacement registry")
	}
	if res.freeze.Status.SelectedSources == nil || res.freeze.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources updated after failover")
	}
	if res.freeze.Status.SelectedSources.ConsensusNode.Image != "reg-b/cn:0.75.0" {
		t.Errorf("expected failover to reg-b, got %s", res.freeze.Status.SelectedSources.ConsensusNode.Image)
	}
	rfCond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)
	if rfCond == nil || rfCond.Status != metav1.ConditionTrue {
		t.Errorf("expected ReadyForFreeze=True after successful failover, got %v", rfCond)
	}
}

func TestFreezeReconcile_NoValidRegistry(t *testing.T) {
	checker := newFreezeChecker(map[string]bool{"reg-a/cn:0.75.0": true, "reg-b/cn:0.75.0": true}, nil)
	res := reconcileFreeze(t, newFreezeForTest(), newSucceededPrepareForFreeze(), checker)

	if res.err == nil {
		t.Error("expected error when no valid registry")
	}
	if res.freeze.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected phase Running, got %s", res.freeze.Status.Phase)
	}
	cond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondRegistryHealthCheck)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected RegistryHealthCheck=False, got %v", cond)
	}
	rfCond := findCondition(res.freeze.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)
	if rfCond == nil || rfCond.Status != metav1.ConditionFalse {
		t.Errorf("expected ReadyForFreeze=False, got %v", rfCond)
	}
	if got := countContaining(res.events, "RegistryInvalid"); got == 0 {
		t.Errorf("expected RegistryInvalid event, got none in %v", res.events)
	}
}

func TestFreezeReconcile_DuplicateOperationID_SetsPhaseFailed(t *testing.T) {
	existing := newFreezeForTest()
	existing.Name = "existing"
	existing.UID = "uid-existing"

	freeze := newFreezeForTest()
	freeze.Name = "duplicate"
	freeze.UID = "uid-duplicate"

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.NetworkUpgradeFreeze{}, fieldOperationID, func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreeze).Spec.OperationID}
		}).
		WithObjects(existing, freeze).
		WithStatusSubresource(existing, freeze).
		Build()

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeFreezeReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newFreezeChecker(nil, nil)

	// First reconcile stamps the finalizer and requeues before any phase work.
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(freeze),
	}); err != nil {
		t.Fatalf("finalizer reconcile: %v", err)
	}

	// Advance "duplicate" to Running phase so the uniqueness check runs.
	var dup operatorv1alpha1.NetworkUpgradeFreeze
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(freeze), &dup); err != nil {
		t.Fatalf("get duplicate: %v", err)
	}
	dup.Status.Phase = operatorv1alpha1.PhaseRunning
	if err := c.Status().Update(context.Background(), &dup); err != nil {
		t.Fatalf("pre-set phase: %v", err)
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(freeze),
	})
	if err == nil {
		t.Error("expected error for duplicate operationId")
	}

	var got operatorv1alpha1.NetworkUpgradeFreeze
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(freeze), &got); getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, operatorv1alpha1.CondManifestValidated)
	if cond == nil {
		t.Fatal("ManifestValidated condition not set")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected ManifestValidated=False, got %s", cond.Status)
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

func TestFreezeReconcile_AlreadySucceededIsNoOp(t *testing.T) {
	freeze := newFreezeForTest()
	freeze.Status.Phase = operatorv1alpha1.PhaseSucceeded

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithObjects(freeze).
		WithStatusSubresource(freeze).
		Build()
	if err := c.Status().Update(context.Background(), freeze); err != nil {
		t.Fatalf("pre-set status: %v", err)
	}

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeFreezeReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newFreezeChecker(nil, nil)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(freeze),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}
	if len(recorded) != 0 {
		t.Errorf("already-succeeded CR should emit no events, got %v", recorded)
	}
}
