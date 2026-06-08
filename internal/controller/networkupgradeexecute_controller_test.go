// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"strings"
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
	executeDigest    = operatorv1alpha1.Sha256Digest("sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	altExecuteDigest = operatorv1alpha1.Sha256Digest("sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
)

type fakeExecuteChecker struct {
	unreachable map[string]bool
	mismatch    map[string]bool
	digest      operatorv1alpha1.Sha256Digest
}

func (c fakeExecuteChecker) Check(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, _ operatorv1alpha1.UpgradeDeterministicSpec) registry.RegistryResult {
	d := c.digest
	if d == "" {
		d = executeDigest
	}
	switch {
	case c.unreachable[spec.Image]:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable}
	case c.mismatch[spec.Image]:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateHashMismatch}
	default:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateValid, Digest: d, LayerHashesVerified: true}
	}
}

func newExecuteForTest() *operatorv1alpha1.NetworkUpgradeExecute {
	return &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{Name: "test-execute", Namespace: "default"},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:       "test-orbit",
			OperationID: "op-001",
			Images: operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: registriesFor("reg-a/cn:0.75.0", "reg-b/cn:0.75.0"),
				},
			},
			SelectedSources: operatorv1alpha1.SelectedSources{
				ConsensusNode: &operatorv1alpha1.SelectedSource{
					Image:               "reg-a/cn:0.75.0",
					Digest:              executeDigest,
					LayerHashesVerified: true,
				},
			},
		},
	}
}

type executeReconcileResult struct {
	events  []string
	execute *operatorv1alpha1.NetworkUpgradeExecute
	err     error
}

func reconcileExecute(t *testing.T, execute *operatorv1alpha1.NetworkUpgradeExecute, checker registry.ImageChecker) executeReconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute).
		WithStatusSubresource(execute).
		Build()

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
	})

	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated); getErr != nil {
		t.Fatalf("get updated execute: %v", getErr)
	}

	return executeReconcileResult{events: recorded, execute: &updated, err: err}
}

func countExecuteEvents(recorded []string, substr string) int {
	n := 0
	for _, e := range recorded {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

func TestExecuteReconcile_SelectedRegistryStillValid(t *testing.T) {
	res := reconcileExecute(t, newExecuteForTest(), fakeExecuteChecker{})

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countExecuteEvents(res.events, "RegistryFailover"); got != 0 {
		t.Errorf("expected no failover events, got %d", got)
	}
	if res.execute.Status.SelectedSources == nil || res.execute.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources to be populated in status")
	}
	if res.execute.Status.SelectedSources.ConsensusNode.Image != "reg-a/cn:0.75.0" {
		t.Errorf("expected original image preserved, got %s", res.execute.Status.SelectedSources.ConsensusNode.Image)
	}
	cond := findCondition(res.execute.Status.Conditions, operatorv1alpha1.CondRegistrySelected)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected RegistrySelected=True, got %v", cond)
	}
}

func TestExecuteReconcile_FailoverToAlternate(t *testing.T) {
	checker := fakeExecuteChecker{
		unreachable: map[string]bool{"reg-a/cn:0.75.0": true},
		digest:      altExecuteDigest,
	}
	res := reconcileExecute(t, newExecuteForTest(), checker)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countExecuteEvents(res.events, "RegistryFailover"); got != 1 {
		t.Errorf("expected one RegistryFailover event, got %d in %v", got, res.events)
	}
	if got := countExecuteEvents(res.events, "reg-a/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the failed registry")
	}
	if got := countExecuteEvents(res.events, "reg-b/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the replacement registry")
	}
	if res.execute.Status.SelectedSources == nil || res.execute.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources updated after failover")
	}
	if res.execute.Status.SelectedSources.ConsensusNode.Image != "reg-b/cn:0.75.0" {
		t.Errorf("expected failover to reg-b, got %s", res.execute.Status.SelectedSources.ConsensusNode.Image)
	}
}

func TestExecuteReconcile_NoValidRegistry_TerminalFailure(t *testing.T) {
	checker := fakeExecuteChecker{
		unreachable: map[string]bool{"reg-a/cn:0.75.0": true, "reg-b/cn:0.75.0": true},
	}
	res := reconcileExecute(t, newExecuteForTest(), checker)

	if res.err == nil {
		t.Error("expected error when no valid registry")
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected phase Failed (terminal), got %s", res.execute.Status.Phase)
	}
	if res.execute.Status.CompletionTime == nil {
		t.Error("expected CompletionTime set on terminal failure")
	}
	cond := findCondition(res.execute.Status.Conditions, operatorv1alpha1.CondRegistrySelected)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected RegistrySelected=False, got %v", cond)
	}
	if got := countExecuteEvents(res.events, "RegistryInvalid"); got == 0 {
		t.Errorf("expected RegistryInvalid event, got none in %v", res.events)
	}
}

func TestExecuteReconcile_AlreadySucceededIsNoOp(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhaseSucceeded

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute).
		WithStatusSubresource(execute).
		Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set status: %v", err)
	}

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = fakeExecuteChecker{}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
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

func TestExecuteReconcile_AlreadyFailedIsNoOp(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute).
		WithStatusSubresource(execute).
		Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set status: %v", err)
	}

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = fakeExecuteChecker{}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}
	if len(recorded) != 0 {
		t.Errorf("already-failed CR should emit no events, got %v", recorded)
	}
}
