// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/upgrade/registry"
)

func newSucceededPrepare(opID string) *operatorv1alpha1.NetworkUpgradePrepare {
	cr := newPrepareCR(opID)
	cr.Status.Phase = operatorv1alpha1.PhaseSucceeded
	return cr
}

func newFreezeCR(opID string, sel operatorv1alpha1.SelectedSources) *operatorv1alpha1.NetworkUpgradeFreeze {
	prepare := newPrepareCR(opID)
	return &operatorv1alpha1.NetworkUpgradeFreeze{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frz-" + opID,
			Namespace: "ns",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepFreeze},
		},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
			Orbit:           "ns",
			OperationID:     opID,
			ScheduledTime:   metav1.NewTime(time.Now().Add(time.Hour)),
			Images:          prepare.Spec.Images,
			SelectedSources: sel,
		},
	}
}

func TestFreezeReconciler_WaitsForPrepare(t *testing.T) {
	scheme := newControllerTestScheme()
	freeze := newFreezeCR("op1", operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(freeze).WithStatusSubresource(freeze).Build()
	r := &NetworkUpgradeFreezeReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(8),
		Registry: registry.NewFakeValidator(),
	}
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: freeze.Name, Namespace: freeze.Namespace}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected requeue waiting for prepare, got %+v", res)
	}
	got := &operatorv1alpha1.NetworkUpgradeFreeze{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: freeze.Name, Namespace: freeze.Namespace}, got)
	if got.Status.Phase == operatorv1alpha1.PhaseSucceeded || got.Status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("unexpected terminal phase %q while prepare not ready", got.Status.Phase)
	}
}

func TestFreezeReconciler_Succeeds_NoFailover(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op2")
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	}
	freeze := newFreezeCR("op2", sel)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(prepare, freeze).WithStatusSubresource(prepare, freeze).Build()

	fakeRegistry := registry.NewFakeValidator()
	fakeRegistry.Set("ghcr.io/a/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:aaa", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/a/evt:0.43.0", registry.FakeOutcome{Digest: "sha256:ccc", LayerHashesVerified: true})

	r := &NetworkUpgradeFreezeReconciler{
		Client: c, Scheme: scheme,
		Recorder: record.NewFakeRecorder(32),
		Registry: fakeRegistry,
	}
	runFreezeReconcile(t, r, freeze.Name, freeze.Namespace)

	got := &operatorv1alpha1.NetworkUpgradeFreeze{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: freeze.Name, Namespace: freeze.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	if got.Status.SelectedSources != nil {
		t.Fatalf("status.selectedSources should be empty when no failover occurred")
	}
}

func TestFreezeReconciler_FailsOver(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op3")
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	}
	freeze := newFreezeCR("op3", sel)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(prepare, freeze).WithStatusSubresource(prepare, freeze).Build()

	fakeRegistry := registry.NewFakeValidator()
	fakeRegistry.Set("ghcr.io/b/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:bbb", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/a/evt:0.43.0", registry.FakeOutcome{Digest: "sha256:ccc", LayerHashesVerified: true})

	recorder := record.NewFakeRecorder(32)
	r := &NetworkUpgradeFreezeReconciler{
		Client: c, Scheme: scheme,
		Recorder: recorder,
		Registry: fakeRegistry,
	}
	runFreezeReconcile(t, r, freeze.Name, freeze.Namespace)

	got := &operatorv1alpha1.NetworkUpgradeFreeze{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: freeze.Name, Namespace: freeze.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	if got.Status.SelectedSources == nil || got.Status.SelectedSources.ConsensusNode == nil {
		t.Fatalf("expected status.selectedSources.consensusNode after failover")
	}
	if got.Status.SelectedSources.ConsensusNode.Image != "ghcr.io/b/cn:0.75.0" {
		t.Fatalf("expected failover to b, got %q", got.Status.SelectedSources.ConsensusNode.Image)
	}
}

func runFreezeReconcile(t *testing.T, r *NetworkUpgradeFreezeReconciler, name, namespace string) {
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
	t.Fatalf("freeze reconcile did not converge")
}
