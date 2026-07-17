// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/joomcode/errorx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

func newControllerTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	return scheme
}

func newCapsuleForControllerTest() *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node0",
			Namespace:  "test",
			Generation: 1,
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
		},
	}
}

func newPodWithPhase(capsule *operatorv1alpha1.ConsensusCapsule, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      capsule.Name + "-0",
			Namespace: capsule.Namespace,
			Labels:    consensus.PodLabels(capsule),
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func newReadyPodWithPhase(capsule *operatorv1alpha1.ConsensusCapsule, phase corev1.PodPhase) *corev1.Pod {
	pod := newPodWithPhase(capsule, phase)
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	return pod
}

func buildReconcilerForUpdateStatus(c client.Client) *ConsensusCapsuleReconciler {
	scheme := newControllerTestScheme()
	return NewConsensusCapsuleReconciler(c, scheme)
}

func TestUpdateStatus_NoPods_PhasePending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule).
		WithStatusSubresource(capsule).
		Build()
	// re-get to pick up the ResourceVersion assigned by the fake store
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_PodRunning_Ready_PhaseRunning(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_PodRunning_NotReady_StaysPending verifies that a Running pod
// whose Ready condition is not yet True keeps the capsule in PhasePending and
// schedules a requeue so the controller re-checks readiness.
func TestUpdateStatus_PodRunning_NotReady_StaysPending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newPodWithPhase(capsule, corev1.PodRunning) // no Ready condition
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != podReadinessRequeueAfter {
		t.Errorf("expected requeue after %v, got %v", podReadinessRequeueAfter, result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending when pod not ready, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_PodFailed_PhaseFailed(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newPodWithPhase(capsule, corev1.PodFailed)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", capsule.Status.Phase)
	}
}

// ── platform-status phase rollup ──────────────────────────────────────────────

const testRevision = "rev-abc123"

// newReadyPodOnRevision returns a Ready Running pod stamped with the given
// StatefulSet controller-revision-hash label.
func newReadyPodOnRevision(capsule *operatorv1alpha1.ConsensusCapsule, revision string) *corev1.Pod {
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	pod.Labels[appsv1.StatefulSetRevisionLabel] = revision
	return pod
}

// newSTSWithUpdateRevision returns a StatefulSet whose status.updateRevision is
// set, so the revision guard can compare the Ready pod's revision against it.
func newSTSWithUpdateRevision(capsule *operatorv1alpha1.ConsensusCapsule, revision string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consensus.StatefulSetName(capsule),
			Namespace: capsule.Namespace,
		},
		Status: appsv1.StatefulSetStatus{UpdateRevision: revision},
	}
}

// buildActiveScenario wires a client with a Ready pod on updateRevision and a
// capsule carrying the given platform status + prior phase, then returns the
// re-fetched capsule so the caller can invoke updateStatus.
func buildActiveScenario(t *testing.T, platformStatus, platformStatusRevision string, priorPhase operatorv1alpha1.Phase, podRevision, updateRevision string) (*ConsensusCapsuleReconciler, *operatorv1alpha1.ConsensusCapsule, context.Context) {
	t.Helper()
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2 // != ObservedGeneration(0) so updateStatus never early-returns
	pod := newReadyPodOnRevision(capsule, podRevision)
	sts := newSTSWithUpdateRevision(capsule, updateRevision)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, sts).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	// PlatformStatus/Phase/PlatformStatusRevision are read from the passed object.
	capsule.Status.PlatformStatus = platformStatus
	capsule.Status.PlatformStatusRevision = platformStatusRevision
	capsule.Status.Phase = priorPhase
	return buildReconcilerForUpdateStatus(c), capsule, ctx
}

func TestUpdateStatus_ActiveWhenPlatformActiveOnCurrentRevision(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, testRevision, operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseActive {
		t.Errorf("expected PhaseActive, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_RevisionGuard_NoPromoteOnOldRevision(t *testing.T) {
	// Pod ACTIVE but on an old revision (image roll in progress) → stays Running.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, "old-rev", operatorv1alpha1.PhaseRunning, "old-rev", testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (stale-reading guard), got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_StickyActive_TransientDipKeepsActive(t *testing.T) {
	// Already Active; platform dips to BEHIND on the current revision → stays Active.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusBehind, testRevision, operatorv1alpha1.PhaseActive, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseActive {
		t.Errorf("expected sticky PhaseActive on transient dip, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_CatastrophicFailure_PhaseFailed(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusCatastrophicFailure, testRevision, operatorv1alpha1.PhaseActive, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed on CATASTROPHIC_FAILURE, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_StalePlatformActive_NoPromote(t *testing.T) {
	// Pod on current revision, but platformStatusRevision is from the old pod →
	// stale ACTIVE must not promote; phase stays Running.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, "old-rev", operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (stale platform-status reading), got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_StaleCatastrophic_NoFail(t *testing.T) {
	// Pod on current revision, but CATASTROPHIC_FAILURE reading is from the old
	// pod → stale reading must not fast-fail; phase stays Running (pod is Ready).
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusCatastrophicFailure, "old-rev", operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (stale CATASTROPHIC reading), got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_EmptyPlatformRevision_FailClosed(t *testing.T) {
	// Empty platformStatusRevision (unstamped UC) → fail-closed: the reading is
	// not trusted, so the node stays Running rather than promoting to Active.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, "", operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (fail-closed: empty revision not trusted), got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_PendingToActive_PatchesCheckpoint(t *testing.T) {
	// A cold start that observes a ready ACTIVE pod jumps Pending→Active; the
	// genesis-initialized checkpoint must still be marked (isReadyPhase covers Active).
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2
	pod := newReadyPodOnRevision(capsule, testRevision)
	sts := newSTSWithUpdateRevision(capsule, testRevision)
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, sts, cm).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	capsule.Status.PlatformStatus = operatorv1alpha1.PlatformStatusActive
	capsule.Status.PlatformStatusRevision = testRevision

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseActive {
		t.Fatalf("expected PhaseActive, got %s", capsule.Status.Phase)
	}
	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueTrue {
		t.Errorf("expected checkpoint patched to true on Pending→Active, got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

// ── widened wake predicate ────────────────────────────────────────────────────

func TestGenOrPlatformStatusChangedPredicate(t *testing.T) {
	p := genOrPlatformStatusChangedPredicate()
	base := &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

	if p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: base.DeepCopy()}) {
		t.Error("a no-op update must be dropped")
	}

	genUp := base.DeepCopy()
	genUp.Generation = 2
	if !p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: genUp}) {
		t.Error("a generation change must fire")
	}

	psUp := base.DeepCopy()
	psUp.Status.PlatformStatus = operatorv1alpha1.PlatformStatusActive
	if !p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: psUp}) {
		t.Error("a status.platformStatus change must fire (UC wake)")
	}

	// The operator's own phase write must NOT wake the controller (no self-loop).
	phaseUp := base.DeepCopy()
	phaseUp.Status.Phase = operatorv1alpha1.PhaseActive
	if p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: phaseUp}) {
		t.Error("a phase-only change must not fire (operator self-write)")
	}
}

// ── UC-mandatory terminal error ───────────────────────────────────────────────

func TestValidateUCMandatory(t *testing.T) {
	capsule := newCapsuleForControllerTest() // no UC configured

	err := validateUCMandatory(capsule)
	if err == nil {
		t.Fatal("expected terminal error when the UC sidecar is disabled")
	}
	if !errorx.IsOfType(err, errUCSidecarRequired) {
		t.Errorf("expected errUCSidecarRequired type, got %v", err)
	}
	if reason, ok := errx.ReasonOf(err); !ok || reason != ReasonUCSidecarRequired {
		t.Errorf("expected reason %s, got %q (ok=%v)", ReasonUCSidecarRequired, reason, ok)
	}
	if hints, ok := errx.Hints(err); !ok || len(hints) == 0 {
		t.Error("expected remediation hints on the UC-mandatory error")
	}
	// The terminal type carries no Temporary trait.
	if errorx.IsOfType(err, errorx.InternalError) {
		t.Error("unexpected type coercion")
	}

	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}
	if err := validateUCMandatory(capsule); err != nil {
		t.Errorf("expected nil when UC is enabled, got %v", err)
	}
}

// ── markGenesisInitialized ────────────────────────────────────────────────────

func newCheckpointCM(capsule *operatorv1alpha1.ConsensusCapsule, value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consensus.CheckpointConfigMapName(capsule),
			Namespace: capsule.Namespace,
		},
		Data: map[string]string{
			consensus.CheckpointKeyGenesisInitialized: value,
		},
	}
}

func TestMarkGenesisInitialized_PatchesCheckpoint(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule, cm).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueTrue {
		t.Errorf("expected genesis-initialized=true after patch, got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

func TestMarkGenesisInitialized_Idempotent(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	cm := newCheckpointCM(capsule, consensus.CheckpointValueTrue)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule, cm).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("unexpected error on already-true checkpoint: %v", err)
	}
}

func TestMarkGenesisInitialized_CheckpointMissing(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	// No checkpoint ConfigMap in the store — should be a no-op, not an error.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule).Build()

	r := buildReconcilerForUpdateStatus(c)
	if err := r.markGenesisInitialized(ctx, capsule); err != nil {
		t.Fatalf("expected nil when checkpoint is missing, got: %v", err)
	}
}

func TestUpdateStatus_PodRunning_PatchesCheckpoint(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, cm).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueTrue {
		t.Errorf("expected checkpoint patched to true when pod running, got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

func TestUpdateStatus_PodRunning_GenChange_DoesNotOverwriteCheckpoint(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	// Status already Running from a previous reconcile.
	capsule.Status.Phase = operatorv1alpha1.PhaseRunning
	capsule.Status.ObservedGeneration = 0 // gen changed → no early-return
	pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
	// Checkpoint was manually reset to false (user wants re-genesis).
	cm := newCheckpointCM(capsule, consensus.CheckpointValueFalse)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, cm).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, got); err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if got.Data[consensus.CheckpointKeyGenesisInitialized] != consensus.CheckpointValueFalse {
		t.Errorf("checkpoint must not be overwritten on gen-change reconcile: got %q", got.Data[consensus.CheckpointKeyGenesisInitialized])
	}
}

func TestUpdateStatus_NoChangeWhenPhaseUnchanged(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Status.Phase = operatorv1alpha1.PhasePending
	capsule.Status.ObservedGeneration = 1
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)

	// No pods → phase stays Pending; ObservedGeneration matches → early return, no Status().Update()
	result, err := r.updateStatus(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected empty result for no-op update, got %+v", result)
	}
}
