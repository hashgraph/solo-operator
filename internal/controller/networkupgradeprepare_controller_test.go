// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// fakePrepareChecker reports every registry reachable except those in fail.
type fakePrepareChecker struct {
	fail map[string]bool
}

func (c fakePrepareChecker) Check(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, _ operatorv1alpha1.UpgradeDeterministicSpec) registry.RegistryResult {
	if c.fail[spec.Image] {
		return registry.RegistryResult{Image: spec.Image, Reachable: false, Err: fmt.Errorf("unreachable")}
	}
	return registry.RegistryResult{Image: spec.Image, Reachable: true}
}

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

func reconcilePrepare(t *testing.T, prepare *operatorv1alpha1.NetworkUpgradePrepare, checker registry.ImageChecker) []string {
	t.Helper()
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(prepare).Build()

	rec := record.NewFakeRecorder(32)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	close(rec.Events)
	var events []string
	for e := range rec.Events {
		events = append(events, e)
	}
	return events
}

func countContaining(events []string, substr string) int {
	n := 0
	for _, e := range events {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

func TestPrepareReconcile_AuditEventAllReachable(t *testing.T) {
	events := reconcilePrepare(t, newPrepareForTest(), registry.OCIChecker{})

	// consensusNode is enabled with 3 registries, all reachable.
	if got := countContaining(events, "RegistryAuditComplete consensusNode: 3/3 registries reachable"); got != 1 {
		t.Errorf("expected one consensusNode audit-complete event, got %d in %v", got, events)
	}
	// All reachable → no unreachable warnings.
	if got := countContaining(events, "RegistryUnreachable"); got != 0 {
		t.Errorf("expected no unreachable warnings, got %d in %v", got, events)
	}
}

func TestPrepareReconcile_SkipsDisabledComponent(t *testing.T) {
	events := reconcilePrepare(t, newPrepareForTest(), registry.OCIChecker{})

	// The disabled recordStreamUploader must not be audited at all.
	if got := countContaining(events, "recordStreamUploader"); got != 0 {
		t.Errorf("disabled component should produce no events, got %d in %v", got, events)
	}
}

func TestPrepareReconcile_EmitsUnreachableWarning(t *testing.T) {
	checker := fakePrepareChecker{fail: map[string]bool{"ghcr.io/b/cn:0.62.0": true}}
	events := reconcilePrepare(t, newPrepareForTest(), checker)

	if got := countContaining(events, "RegistryAuditComplete consensusNode: 2/3 registries reachable"); got != 1 {
		t.Errorf("expected 2/3 reachable audit event, got %d in %v", got, events)
	}
	if got := countContaining(events, "RegistryUnreachable consensusNode: 1/3 registries unreachable"); got != 1 {
		t.Errorf("expected 1/3 unreachable warning, got %d in %v", got, events)
	}
}
