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

func boolPtr(b bool) *bool { return &b }

func registriesFor(images ...string) []operatorv1alpha1.UpgradeRegistrySpec {
	out := make([]operatorv1alpha1.UpgradeRegistrySpec, len(images))
	for i, img := range images {
		out[i] = operatorv1alpha1.UpgradeRegistrySpec{Image: img}
	}
	return out
}

func newPrepareForTest() *operatorv1alpha1.NetworkUpgradePrepare {
	return &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "test"},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			Orbit:       "test-orbit",
			OperationID: "op-001",
			Images: operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.62.0",
					Registries: registriesFor("gcr.io/a/cn:0.62.0", "ghcr.io/b/cn:0.62.0", "reg.c.io/cn:0.62.0"),
				},
				// present but disabled — must be skipped entirely
				RecordStreamUploader: &operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.62.0",
					Enabled:    boolPtr(false),
					Registries: registriesFor("gcr.io/a/rsu:0.62.0", "ghcr.io/b/rsu:0.62.0"),
				},
			},
		},
	}
}

type reconcileResult struct {
	events  []string
	prepare *operatorv1alpha1.NetworkUpgradePrepare
	err     error
}

func reconcilePrepare(t *testing.T, prepare *operatorv1alpha1.NetworkUpgradePrepare, checker registry.ImageChecker) reconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare).
		WithStatusSubresource(prepare).
		Build()

	rec := events.NewFakeRecorder(64)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
	})

	close(rec.Events)
	var recorded []string
	for e := range rec.Events {
		recorded = append(recorded, e)
	}

	var updated operatorv1alpha1.NetworkUpgradePrepare
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(prepare), &updated); getErr != nil {
		t.Fatalf("get updated prepare: %v", getErr)
	}

	return reconcileResult{events: recorded, prepare: &updated, err: err}
}

func countContaining(recorded []string, substr string) int {
	n := 0
	for _, e := range recorded {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

const prepareDefaultDigest = operatorv1alpha1.Sha256Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

func newPrepareChecker(unreachable, mismatch map[string]bool) fakeUpgradeChecker {
	return fakeUpgradeChecker{unreachable: unreachable, mismatch: mismatch, defaultDigest: prepareDefaultDigest}
}

func TestPrepareReconcile_AuditEventAllReachable(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	if got := countContaining(res.events, "RegistryAuditComplete consensusNode: 3/3 registries reachable"); got != 1 {
		t.Errorf("expected one consensusNode audit-complete event, got %d in %v", got, res.events)
	}
	if got := countContaining(res.events, "RegistryUnreachable"); got != 0 {
		t.Errorf("expected no unreachable warnings, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_SkipsDisabledComponent(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	if got := countContaining(res.events, "recordStreamUploader"); got != 0 {
		t.Errorf("disabled component should produce no events, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_NotFound(t *testing.T) {
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := NewNetworkUpgradePrepareReconciler(c, scheme)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "missing", Namespace: "test"},
	})
	if err != nil {
		t.Fatalf("expected no error for missing object, got %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected no requeue for missing object, got %+v", result)
	}
}

func TestPrepareReconcile_EmitsUnreachableWarning(t *testing.T) {
	checker := newPrepareChecker(map[string]bool{"ghcr.io/b/cn:0.62.0": true}, nil)
	res := reconcilePrepare(t, newPrepareForTest(), checker)

	if got := countContaining(res.events, "RegistryAuditComplete consensusNode: 2/3 registries reachable"); got != 1 {
		t.Errorf("expected 2/3 reachable audit event, got %d in %v", got, res.events)
	}
	if got := countContaining(res.events, "RegistryUnreachable consensusNode: 1/3 registries unreachable"); got != 1 {
		t.Errorf("expected 1/3 unreachable warning, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_EmitsInvalidOnHashMismatch(t *testing.T) {
	checker := newPrepareChecker(nil, map[string]bool{"ghcr.io/b/cn:0.62.0": true})
	res := reconcilePrepare(t, newPrepareForTest(), checker)

	if got := countContaining(res.events, "RegistryAuditComplete consensusNode: 3/3 registries reachable"); got != 1 {
		t.Errorf("expected 3/3 reachable audit event, got %d in %v", got, res.events)
	}
	if got := countContaining(res.events, "RegistryInvalid consensusNode: registry ghcr.io/b/cn:0.62.0 failed layer hash verification"); got != 1 {
		t.Errorf("expected one RegistryInvalid event, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_Success(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	t.Run("NoError", func(t *testing.T) {
		if res.err != nil {
			t.Fatalf("unexpected error: %v", res.err)
		}
	})
	t.Run("PhaseSucceeded", func(t *testing.T) {
		if res.prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			t.Errorf("expected phase Succeeded, got %s", res.prepare.Status.Phase)
		}
	})
	t.Run("SelectedSourcesPopulated", func(t *testing.T) {
		if res.prepare.Status.SelectedSources == nil {
			t.Fatal("expected selectedSources to be populated")
		}
		if res.prepare.Status.SelectedSources.ConsensusNode == nil {
			t.Error("expected consensusNode selectedSource to be set")
		}
	})
	t.Run("CompletionTimeSet", func(t *testing.T) {
		if res.prepare.Status.CompletionTime == nil {
			t.Error("expected completionTime to be set")
		}
	})
	t.Run("RegistrySelectedEvent", func(t *testing.T) {
		if got := countContaining(res.events, "RegistrySelected consensusNode"); got != 1 {
			t.Errorf("expected one RegistrySelected event for consensusNode, got %d in %v", got, res.events)
		}
	})
	t.Run("ConditionTrue", func(t *testing.T) {
		cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondRegistrySelected)
		if cond == nil {
			t.Fatal("RegistrySelected condition not set")
		}
		if cond.Status != metav1.ConditionTrue {
			t.Errorf("expected condition True, got %s", cond.Status)
		}
		if cond.Reason != operatorv1alpha1.ReasonSelectionComplete {
			t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonSelectionComplete, cond.Reason)
		}
	})
}

func TestPrepareReconcile_AllInvalidFails(t *testing.T) {
	checker := newPrepareChecker(map[string]bool{
		"gcr.io/a/cn:0.62.0":  true,
		"ghcr.io/b/cn:0.62.0": true,
		"reg.c.io/cn:0.62.0":  true,
	}, nil)
	res := reconcilePrepare(t, newPrepareForTest(), checker)

	if res.err == nil {
		t.Error("expected error when no valid registry")
	}
	if res.prepare.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected phase Running, got %s", res.prepare.Status.Phase)
	}
	if res.prepare.Status.SelectedSources != nil && res.prepare.Status.SelectedSources.ConsensusNode != nil {
		t.Error("should not set selectedSources when no valid registry")
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondRegistrySelected)
	if cond == nil {
		t.Fatal("RegistrySelected condition not set on failure")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected condition False, got %s", cond.Status)
	}
}

func TestPrepareReconcile_AlreadySucceededIsNoOp(t *testing.T) {
	prepare := newPrepareForTest()
	prepare.Status.Phase = operatorv1alpha1.PhaseSucceeded

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare).
		WithStatusSubresource(prepare).
		Build()
	// Pre-set status so it survives the fake client round-trip.
	if err := c.Status().Update(context.Background(), prepare); err != nil {
		t.Fatalf("pre-set status: %v", err)
	}

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newPrepareChecker(nil, nil)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
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
