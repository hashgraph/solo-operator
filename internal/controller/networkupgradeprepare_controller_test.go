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
	"github.com/hashgraph/solo-operator/internal/upgrade/registry"
)

func newPrepareCR(opID string) *operatorv1alpha1.NetworkUpgradePrepare {
	enabled := true
	disabled := false
	return &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prep-" + opID,
			Namespace: "ns",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
		},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			Orbit:       "ns",
			OperationID: opID,
			Images: operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Enabled:    &enabled,
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "ghcr.io/a/cn:0.75.0"}, {Image: "ghcr.io/b/cn:0.75.0"}},
				},
				RecordStreamUploader: &operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.43.0",
					Enabled:    &disabled,
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "ghcr.io/a/rec:0.43.0"}},
				},
				EventStreamUploader: &operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.43.0",
					Enabled:    &enabled,
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "ghcr.io/a/evt:0.43.0"}},
				},
			},
		},
	}
}

func runReconcile(t *testing.T, r *NetworkUpgradePrepareReconciler, name, namespace string) {
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
	t.Fatalf("reconcile did not converge within 5 iterations")
}

func TestPrepareReconciler_Succeeds(t *testing.T) {
	scheme := newControllerTestScheme()
	cr := newPrepareCR("op1")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).WithStatusSubresource(cr).Build()

	fakeRegistry := registry.NewFakeValidator()
	fakeRegistry.Set("ghcr.io/a/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:aaa", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/b/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:bbb", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/a/evt:0.43.0", registry.FakeOutcome{Digest: "sha256:ccc", LayerHashesVerified: true})

	r := &NetworkUpgradePrepareReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(32),
		Registry: fakeRegistry,
	}
	runReconcile(t, r, cr.Name, cr.Namespace)

	got := &operatorv1alpha1.NetworkUpgradePrepare{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	if got.Status.SelectedSources == nil || got.Status.SelectedSources.ConsensusNode == nil {
		t.Fatalf("selectedSources.consensusNode missing")
	}
	if got.Status.SelectedSources.ConsensusNode.Digest == "" {
		t.Fatalf("consensus digest empty")
	}
	if got.Status.SelectedSources.EventStreamUploader == nil {
		t.Fatalf("selectedSources.eventStreamUploader missing")
	}
	if got.Status.SelectedSources.RecordStreamUploader != nil {
		t.Fatalf("disabled component should not appear in selectedSources")
	}
}

func TestPrepareReconciler_FailsWhenNoRegistryValid(t *testing.T) {
	scheme := newControllerTestScheme()
	cr := newPrepareCR("op2")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).WithStatusSubresource(cr).Build()

	r := &NetworkUpgradePrepareReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(32),
		Registry: registry.NewFakeValidator(),
	}
	runReconcile(t, r, cr.Name, cr.Namespace)

	got := &operatorv1alpha1.NetworkUpgradePrepare{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q want Failed", got.Status.Phase)
	}
}

func TestPrepareReconciler_PartialRegistryFailureStillSelects(t *testing.T) {
	scheme := newControllerTestScheme()
	cr := newPrepareCR("op3")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).WithStatusSubresource(cr).Build()

	fakeRegistry := registry.NewFakeValidator()
	fakeRegistry.Set("ghcr.io/b/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:bbb", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/a/evt:0.43.0", registry.FakeOutcome{Digest: "sha256:ccc", LayerHashesVerified: true})

	r := &NetworkUpgradePrepareReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(32),
		Registry: fakeRegistry,
	}
	runReconcile(t, r, cr.Name, cr.Namespace)

	got := &operatorv1alpha1.NetworkUpgradePrepare{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	if got.Status.SelectedSources.ConsensusNode.Image != "ghcr.io/b/cn:0.75.0" {
		t.Fatalf("expected b registry selected, got %q", got.Status.SelectedSources.ConsensusNode.Image)
	}
}

func TestPrepareReconciler_TerminalPhaseShortCircuits(t *testing.T) {
	scheme := newControllerTestScheme()
	cr := newPrepareCR("op4")
	cr.Status.Phase = operatorv1alpha1.PhaseSucceeded
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).WithStatusSubresource(cr).Build()

	r := &NetworkUpgradePrepareReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(8),
		Registry: registry.NewFakeValidator(),
	}
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: cr.Name, Namespace: cr.Namespace}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Requeue || res.RequeueAfter != 0 {
		t.Fatalf("expected no requeue for terminal CR, got %+v", res)
	}
}
