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
	freezeDigest    = operatorv1alpha1.Sha256Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	altFreezeDigest = operatorv1alpha1.Sha256Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
)

// fakeFreezeChecker is analogous to fakePrepareChecker but also supports
// returning a specific digest for valid results.
type fakeFreezeChecker struct {
	unreachable map[string]bool
	mismatch    map[string]bool
	digest      operatorv1alpha1.Sha256Digest
}

func (c fakeFreezeChecker) Check(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, _ operatorv1alpha1.UpgradeDeterministicSpec) registry.RegistryResult {
	d := c.digest
	if d == "" {
		d = freezeDigest
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

func newFreezeForTest() *operatorv1alpha1.NetworkUpgradeFreeze {
	return &operatorv1alpha1.NetworkUpgradeFreeze{
		ObjectMeta: metav1.ObjectMeta{Name: "test-freeze", Namespace: "default"},
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

type freezeReconcileResult struct {
	events []string
	freeze *operatorv1alpha1.NetworkUpgradeFreeze
	err    error
}

func reconcileFreeze(t *testing.T, freeze *operatorv1alpha1.NetworkUpgradeFreeze, checker registry.ImageChecker) freezeReconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(freeze).
		WithStatusSubresource(freeze).
		Build()

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeFreezeReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(freeze),
	})

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

func countFreezeEvents(recorded []string, substr string) int {
	n := 0
	for _, e := range recorded {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

func TestFreezeReconcile_SelectedRegistryStillValid(t *testing.T) {
	res := reconcileFreeze(t, newFreezeForTest(), fakeFreezeChecker{})

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countFreezeEvents(res.events, "RegistryFailover"); got != 0 {
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
}

func TestFreezeReconcile_FailoverToAlternate(t *testing.T) {
	checker := fakeFreezeChecker{
		unreachable: map[string]bool{"reg-a/cn:0.75.0": true},
		digest:      altFreezeDigest,
	}
	res := reconcileFreeze(t, newFreezeForTest(), checker)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countFreezeEvents(res.events, "RegistryFailover"); got != 1 {
		t.Errorf("expected one RegistryFailover event, got %d in %v", got, res.events)
	}
	if got := countFreezeEvents(res.events, "reg-a/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the failed registry")
	}
	if got := countFreezeEvents(res.events, "reg-b/cn:0.75.0"); got == 0 {
		t.Errorf("RegistryFailover event should name the replacement registry")
	}
	if res.freeze.Status.SelectedSources == nil || res.freeze.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources updated after failover")
	}
	if res.freeze.Status.SelectedSources.ConsensusNode.Image != "reg-b/cn:0.75.0" {
		t.Errorf("expected failover to reg-b, got %s", res.freeze.Status.SelectedSources.ConsensusNode.Image)
	}
}

func TestFreezeReconcile_NoValidRegistry(t *testing.T) {
	checker := fakeFreezeChecker{
		unreachable: map[string]bool{"reg-a/cn:0.75.0": true, "reg-b/cn:0.75.0": true},
	}
	res := reconcileFreeze(t, newFreezeForTest(), checker)

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
	if got := countFreezeEvents(res.events, "RegistryInvalid"); got == 0 {
		t.Errorf("expected RegistryInvalid event, got none in %v", res.events)
	}
}

func TestFreezeReconcile_AlreadySucceededIsNoOp(t *testing.T) {
	freeze := newFreezeForTest()
	freeze.Status.Phase = operatorv1alpha1.PhaseSucceeded

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(freeze).
		WithStatusSubresource(freeze).
		Build()
	if err := c.Status().Update(context.Background(), freeze); err != nil {
		t.Fatalf("pre-set status: %v", err)
	}

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeFreezeReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = fakeFreezeChecker{}

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
