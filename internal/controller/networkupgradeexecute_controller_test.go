// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/upgrade/registry"
)

func newSucceededFreeze(opID string, sel operatorv1alpha1.SelectedSources) *operatorv1alpha1.NetworkUpgradeFreeze {
	cr := newFreezeCR(opID, sel)
	cr.Status.Phase = operatorv1alpha1.PhaseSucceeded
	return cr
}

func newExecuteCR(opID string, images operatorv1alpha1.UpgradeImagesSpec, sel operatorv1alpha1.SelectedSources) *operatorv1alpha1.NetworkUpgradeExecute {
	return &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exec-" + opID,
			Namespace: "ns",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepExecute},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:           "ns",
			OperationID:     opID,
			Images:          images,
			SelectedSources: sel,
		},
	}
}

func newCC(name string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "ns", Generation: 1,
			Labels: map[string]string{"operator.solo.hedera.com/orbit": "ns"},
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "ns",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
		},
	}
}

func TestExecuteReconciler_WaitsForPrepareAndFreeze(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareCR("op1")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	}
	exec := newExecuteCR("op1", images, sel)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(exec).WithStatusSubresource(exec).Build()

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(16),
		Registry:        registry.NewFakeValidator(),
		PodReadyTimeout: time.Minute,
	}
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected requeue waiting for predecessors, got %+v", res)
	}
}

func TestExecuteReconciler_ScalesDownAndSignalsDaemon(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op2")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	}
	freeze := newSucceededFreeze("op2", sel)
	exec := newExecuteCR("op2", images, sel)
	cc := newCC("node0")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, freeze, exec, cc).
		WithStatusSubresource(prepare, freeze, exec, cc).
		Build()

	fakeRegistry := registry.NewFakeValidator()
	fakeRegistry.Set("ghcr.io/a/cn:0.75.0", registry.FakeOutcome{Digest: "sha256:aaa", LayerHashesVerified: true})
	fakeRegistry.Set("ghcr.io/a/evt:0.43.0", registry.FakeOutcome{Digest: "sha256:ccc", LayerHashesVerified: true})

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(32),
		Registry:        fakeRegistry,
		PodReadyTimeout: time.Minute,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := &operatorv1alpha1.NetworkUpgradeExecute{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Fatalf("phase = %q want ReadyForProvisionerDaemon", got.Status.Phase)
	}
	gotCC := &operatorv1alpha1.ConsensusCapsule{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: "node0", Namespace: "ns"}, gotCC)
	if !gotCC.Spec.Offline {
		t.Fatalf("expected ConsensusCapsule.Spec.Offline=true, got false")
	}
}

func TestExecuteReconciler_DaemonFailure_MarksFailed(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op3")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
	}
	freeze := newSucceededFreeze("op3", sel)
	exec := newExecuteCR("op3", images, sel)
	exec.Status.Phase = operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon
	exec.Status.Conditions = []metav1.Condition{{
		Type: operatorv1alpha1.CondDaemonResult, Status: metav1.ConditionFalse,
		Reason: "DaemonFailed", Message: "infra upgrade failed",
		LastTransitionTime: metav1.Now(),
	}}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, freeze, exec).
		WithStatusSubresource(prepare, freeze, exec).
		Build()

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(32),
		Registry:        registry.NewFakeValidator(),
		PodReadyTimeout: time.Minute,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := &operatorv1alpha1.NetworkUpgradeExecute{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Fatalf("phase = %q want Failed", got.Status.Phase)
	}
}

func TestExecuteReconciler_ThreeRulePatch_PendingNodeUpgrade(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op4")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode:       &operatorv1alpha1.SelectedSource{Image: "ghcr.io/b/cn:0.75.0", Digest: "sha256:bbb"},
		EventStreamUploader: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/evt:0.43.0", Digest: "sha256:ccc"},
	}
	freeze := newSucceededFreeze("op4", sel)
	exec := newExecuteCR("op4", images, sel)
	exec.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	exec.Status.Conditions = []metav1.Condition{{
		Type: operatorv1alpha1.CondDaemonResult, Status: metav1.ConditionTrue,
		Reason: "Succeeded", LastTransitionTime: metav1.Now(),
	}}
	now := metav1.Now()
	exec.Status.StartTime = &now

	cc := newCC("node0")
	// Pre-existing sidecars: records (will be disabled), events (will be upgraded)
	enabled := true
	cc.Spec.PodProperties.Containers = operatorv1alpha1.ConsensusContainers{
		ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
			Name: "consensus-node",
			SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
				Repository: "ghcr.io", ImageName: "old/cn", ImageTag: "old",
			},
		},
		RecordsUploader: &operatorv1alpha1.CheetahUploader{Name: "rec", Enabled: enabled},
		EventsUploader: &operatorv1alpha1.CheetahUploader{
			Name:    "evt",
			Enabled: enabled,
			SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
				Repository: "ghcr.io", ImageName: "old/evt", ImageTag: "old",
			},
		},
	}
	cc.Spec.Offline = true

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, freeze, exec, cc).
		WithStatusSubresource(prepare, freeze, exec, cc).
		Build()

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(32),
		Registry:        registry.NewFakeValidator(),
		PodReadyTimeout: time.Minute,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	gotCC := &operatorv1alpha1.ConsensusCapsule{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "node0", Namespace: "ns"}, gotCC); err != nil {
		t.Fatalf("get cc: %v", err)
	}
	if gotCC.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.ImageName != "b/cn" {
		t.Fatalf("consensusNode image not patched: %+v", gotCC.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion)
	}
	if gotCC.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.ManifestHash != "sha256:bbb" {
		t.Fatalf("consensusNode digest not set: %+v", gotCC.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion)
	}
	if gotCC.Spec.PodProperties.Containers.RecordsUploader != nil {
		t.Fatalf("recordsUploader should be removed (disabled by manifest)")
	}
	if gotCC.Spec.PodProperties.Containers.EventsUploader == nil ||
		gotCC.Spec.PodProperties.Containers.EventsUploader.SoftwareVersion.ImageName != "a/evt" {
		t.Fatalf("eventsUploader not patched: %+v", gotCC.Spec.PodProperties.Containers.EventsUploader)
	}
	if gotCC.Spec.Offline {
		t.Fatalf("expected ConsensusCapsule.Spec.Offline=false after patch, got true")
	}

	gotExec := &operatorv1alpha1.NetworkUpgradeExecute{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, gotExec)
	cond := findUpgradeCondition(gotExec.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConsensusCapsulePatched=True, got %+v", cond)
	}
}

func TestExecuteReconciler_PodReady_MarksSucceeded(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op5")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/b/cn:0.75.0", Digest: "sha256:bbb"},
	}
	freeze := newSucceededFreeze("op5", sel)
	exec := newExecuteCR("op5", images, sel)
	exec.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	now := metav1.Now()
	exec.Status.StartTime = &now
	exec.Status.Conditions = []metav1.Condition{
		{Type: operatorv1alpha1.CondDaemonResult, Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: now},
		{Type: operatorv1alpha1.CondConsensusCapsulePatched, Status: metav1.ConditionTrue, Reason: "PatchApplied", LastTransitionTime: now},
	}

	cc := newCC("node0")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0-0",
			Namespace: "ns",
			Labels: map[string]string{
				"app.kubernetes.io/instance":     "node0",
				"operator.solo.hedera.com/orbit": "ns",
			},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
		}},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, freeze, exec, cc, pod).
		WithStatusSubresource(prepare, freeze, exec, cc).
		Build()

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(32),
		Registry:        registry.NewFakeValidator(),
		PodReadyTimeout: time.Minute,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := &operatorv1alpha1.NetworkUpgradeExecute{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.ExecutePhaseSucceeded {
		t.Fatalf("phase = %q want Succeeded", got.Status.Phase)
	}
	if len(got.Status.UpgradedNodes) != 1 || got.Status.UpgradedNodes[0] != "0" {
		t.Fatalf("upgradedNodes = %v want [0]", got.Status.UpgradedNodes)
	}
}

func TestExecuteReconciler_PodReadyTimeout_MarksFailed(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newSucceededPrepare("op6")
	images := prepare.Spec.Images
	sel := operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/a/cn:0.75.0", Digest: "sha256:aaa"},
	}
	freeze := newSucceededFreeze("op6", sel)
	exec := newExecuteCR("op6", images, sel)
	exec.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	old := metav1.NewTime(time.Now().Add(-time.Hour))
	exec.Status.StartTime = &old
	exec.Status.Conditions = []metav1.Condition{
		{Type: operatorv1alpha1.CondDaemonResult, Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: old},
		{Type: operatorv1alpha1.CondConsensusCapsulePatched, Status: metav1.ConditionTrue, Reason: "PatchApplied", LastTransitionTime: old},
	}

	cc := newCC("node0")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, freeze, exec, cc).
		WithStatusSubresource(prepare, freeze, exec, cc).
		Build()

	r := &NetworkUpgradeExecuteReconciler{
		Client: c, Scheme: scheme,
		Recorder:        record.NewFakeRecorder(32),
		Registry:        registry.NewFakeValidator(),
		PodReadyTimeout: time.Minute,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := &operatorv1alpha1.NetworkUpgradeExecute{}
	_ = c.Get(context.Background(), client.ObjectKey{Name: exec.Name, Namespace: exec.Namespace}, got)
	if got.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Fatalf("phase = %q want Failed", got.Status.Phase)
	}
}
