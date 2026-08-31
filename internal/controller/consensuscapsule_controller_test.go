// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/automa-saga/errx"
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
	// updateStatus never execs, so the clientset/restConfig are left nil rather than
	// going through NewConsensusCapsuleReconciler (which needs a real rest.Config).
	return &ConsensusCapsuleReconciler{
		Client: c,
		Scheme: newControllerTestScheme(),
	}
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

	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
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

	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
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

	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
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

	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
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

// ── platform-status phase rollup ─────────────────────────────────────────────

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

// newReadyPodWithPlatformStatus returns a Ready Running pod on the given revision
// carrying the UC-written platform-status condition (reason = platformStatus). An
// empty platformStatus omits the condition — modelling a fresh pod whose UC has not
// reported yet.
func newReadyPodWithPlatformStatus(capsule *operatorv1alpha1.ConsensusCapsule, revision, platformStatus string) *corev1.Pod {
	pod := newReadyPodOnRevision(capsule, revision)
	if platformStatus != "" {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   operatorv1alpha1.PodConditionPlatformStatus,
			Status: corev1.ConditionTrue,
			Reason: platformStatus,
		})
	}
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

// buildActiveScenario wires a client with a Ready pod on podRevision carrying the
// given platform-status condition, an STS whose updateRevision is updateRevision,
// and a capsule at priorPhase, then returns the re-fetched capsule so the caller can
// invoke updateStatus. An empty platformStatus omits the pod condition.
func buildActiveScenario(t *testing.T, platformStatus string, priorPhase operatorv1alpha1.Phase, podRevision, updateRevision string) (*ConsensusCapsuleReconciler, *operatorv1alpha1.ConsensusCapsule, context.Context) {
	t.Helper()
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2 // != ObservedGeneration(0) so updateStatus never early-returns
	pod := newReadyPodWithPlatformStatus(capsule, podRevision, platformStatus)
	sts := newSTSWithUpdateRevision(capsule, updateRevision)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, sts).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	// The platform status is read from the pod condition; only the prior phase is
	// seeded on the capsule (for sticky-Active cases).
	capsule.Status.Phase = priorPhase
	return buildReconcilerForUpdateStatus(c), capsule, ctx
}

func TestUpdateStatus_ActiveWhenPlatformActiveOnCurrentRevision(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseActive {
		t.Errorf("expected PhaseActive, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_AutoIdle_StaysRunning verifies that an Auto-start capsule with the
// same idle reading (Ready pod, UNKNOWN platform status) is unaffected by the Stopped
// branch and stays Running while the CN comes up.
func TestUpdateStatus_AutoIdle_StaysRunning(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyAuto
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning for Auto+idle, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_Manual_Idle_PhaseStopped verifies that a capsule with StartPolicy=Manual
// and UNKNOWN platform status settles into PhaseStopped when DesiredRunState is not Running.
func TestUpdateStatus_Manual_Idle_PhaseStopped(t *testing.T) {
	t.Run("DesiredRunState is Stopped", func(t *testing.T) {
		r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhasePending, testRevision, testRevision)
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped
		if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.Status.Phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %s", capsule.Status.Phase)
		}
	})

	t.Run("DesiredRunState is empty", func(t *testing.T) {
		r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhasePending, testRevision, testRevision)
		capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
		capsule.Spec.DesiredRunState = ""
		if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.Status.Phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %s", capsule.Status.Phase)
		}
	})
}

// TestUpdateStatus_Auto_DesiredStopped_PhaseStopped verifies that when StartPolicy=Auto
// and DesiredRunState=Stopped, an idle pod (platformStatus=UNKNOWN) reports PhaseStopped.
func TestUpdateStatus_Auto_DesiredStopped_PhaseStopped(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhasePending, testRevision, testRevision)
	capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyAuto
	capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseStopped {
		t.Errorf("expected PhaseStopped on Auto policy with DesiredRunState=Stopped, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_Manual_DesiredRunning_Idle_StaysRunning verifies that when
// StartPolicy=Manual and DesiredRunState=Running, an idle pod (platformStatus=UNKNOWN)
// reports PhaseRunning during startup rather than being misreported as PhaseStopped.
func TestUpdateStatus_Manual_DesiredRunning_Idle_StaysRunning(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhasePending, testRevision, testRevision)
	capsule.Spec.StartPolicy = operatorv1alpha1.StartPolicyManual
	capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning during Manual startup, got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_ProxiesPlatformStatusFromCondition verifies the operator (the
// sole writer of the capsule status) proxies the pod condition's raw name into
// status.platformStatus and derives the numeric code.
func TestUpdateStatus_ProxiesPlatformStatusFromCondition(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusReplayingEvents, operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.PlatformStatus != operatorv1alpha1.PlatformStatusReplayingEvents {
		t.Errorf("expected proxied platformStatus REPLAYING_EVENTS, got %q", capsule.Status.PlatformStatus)
	}
	if capsule.Status.PlatformStatusCode != 7 {
		t.Errorf("expected derived platformStatusCode 7, got %d", capsule.Status.PlatformStatusCode)
	}
}

func TestUpdateStatus_RevisionGuard_NoPromoteOnOldRevision(t *testing.T) {
	// Pod ACTIVE but on an old revision (image roll in progress) → stays Running.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusActive, operatorv1alpha1.PhaseRunning, "old-rev", testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (stale-reading guard), got %s", capsule.Status.Phase)
	}
	// The reading from an old-revision pod is not trusted, so the PLATFORM column
	// reflects UNKNOWN rather than the old pod's ACTIVE — the operator trusts only the
	// pod on the current updateRevision.
	if capsule.Status.PlatformStatus != operatorv1alpha1.PlatformStatusUnknown {
		t.Errorf("expected platformStatus UNKNOWN off the current revision, got %q", capsule.Status.PlatformStatus)
	}
	if capsule.Status.PlatformStatusCode != operatorv1alpha1.PlatformStatusCodeUnknown {
		t.Errorf("expected platformStatusCode %d off the current revision, got %d", operatorv1alpha1.PlatformStatusCodeUnknown, capsule.Status.PlatformStatusCode)
	}
}

func TestUpdateStatus_StickyActive_TransientDipKeepsActive(t *testing.T) {
	// Already Active; platform dips to BEHIND on the current revision → stays Active.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusBehind, operatorv1alpha1.PhaseActive, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseActive {
		t.Errorf("expected sticky PhaseActive on transient dip, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_StickyActive_OldRevision_DropsToRunning(t *testing.T) {
	// Already Active but the pod now runs on an old revision (a new STS template
	// exists and the pod hasn't rolled) → sticky must not hold Active off the current
	// revision.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusBehind, operatorv1alpha1.PhaseActive, "old-rev", testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (sticky not honored off current revision), got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_StickyActive_NoReading_DropsToRunning verifies sticky-Active does
// not hold Active on stale state: a Ready pod on the current revision that has not (yet)
// reported a platform status reflects UNKNOWN, so a previously-Active capsule falls back
// to Running rather than staying Active with no live confirmation. This covers a fresh
// pod after a roll / operator restart, where trusting sticky-Active would let upgrade
// gates succeed on stale state.
func TestUpdateStatus_StickyActive_NoReading_DropsToRunning(t *testing.T) {
	// Empty platformStatus omits the pod condition; prior phase is Active.
	r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhaseActive, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (sticky not held on UNKNOWN reading), got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_CatastrophicFailure_PhaseFailed(t *testing.T) {
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusCatastrophicFailure, operatorv1alpha1.PhaseActive, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed on CATASTROPHIC_FAILURE, got %s", capsule.Status.Phase)
	}
}

func TestUpdateStatus_CatastrophicOnOldRevision_NoFail(t *testing.T) {
	// CATASTROPHIC_FAILURE from a pod on an old revision (mid-roll) must not fast-fail
	// the capsule — only a current-revision reading drives the phase. Pod is Ready.
	r, capsule, ctx := buildActiveScenario(t, operatorv1alpha1.PlatformStatusCatastrophicFailure, operatorv1alpha1.PhaseRunning, "old-rev", testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (CATASTROPHIC off current revision ignored), got %s", capsule.Status.Phase)
	}
}

// TestUpdateStatus_NoConditionYet_ReflectsUnknown verifies a Ready pod on the current
// revision whose UC has not written the platform-status condition yet reflects
// UNKNOWN (no retained/stale value) and stays Running rather than promoting to Active.
// This replaces the old revision-freshness guard: no stale value survives a roll.
func TestUpdateStatus_NoConditionYet_ReflectsUnknown(t *testing.T) {
	// Empty platformStatus omits the pod condition.
	r, capsule, ctx := buildActiveScenario(t, "", operatorv1alpha1.PhaseRunning, testRevision, testRevision)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected PhaseRunning (no reading, not trusted), got %s", capsule.Status.Phase)
	}
	if capsule.Status.PlatformStatus != operatorv1alpha1.PlatformStatusUnknown {
		t.Errorf("expected platformStatus UNKNOWN when the pod has no condition, got %q", capsule.Status.PlatformStatus)
	}
	if capsule.Status.PlatformStatusCode != operatorv1alpha1.PlatformStatusCodeUnknown {
		t.Errorf("expected platformStatusCode %d, got %d", operatorv1alpha1.PlatformStatusCodeUnknown, capsule.Status.PlatformStatusCode)
	}
}

// TestUpdateStatus_TerminatingPod_ReflectsUnknown verifies a Terminating pod's
// platform-status condition is treated as stale (its UC has stopped): the operator
// reflects UNKNOWN rather than the frozen last value, even if it still reads ACTIVE.
func TestUpdateStatus_TerminatingPod_ReflectsUnknown(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2
	pod := newReadyPodWithPlatformStatus(capsule, testRevision, operatorv1alpha1.PlatformStatusActive)
	now := metav1.Now()
	pod.DeletionTimestamp = &now
	pod.Finalizers = []string{"keep/terminating"} // a DeletionTimestamp requires a finalizer in the fake client
	sts := newSTSWithUpdateRevision(capsule, testRevision)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, sts).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	capsule.Status.Phase = operatorv1alpha1.PhaseActive

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.PlatformStatus != operatorv1alpha1.PlatformStatusUnknown {
		t.Errorf("expected platformStatus UNKNOWN for a Terminating pod, got %q", capsule.Status.PlatformStatus)
	}
}

// TestUpdateStatus_PlatformActiveButNotReady_StaysPending verifies the platform-status
// condition is not a readiness substitute: a pod reporting ACTIVE but whose PodReady
// condition is not True stays Pending (readiness gates on PodReady only).
func TestUpdateStatus_PlatformActiveButNotReady_StaysPending(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2
	pod := newPodWithPhase(capsule, corev1.PodRunning) // no PodReady=True
	pod.Labels[appsv1.StatefulSetRevisionLabel] = testRevision
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: operatorv1alpha1.PodConditionPlatformStatus, Status: corev1.ConditionTrue, Reason: operatorv1alpha1.PlatformStatusActive},
	}
	sts := newSTSWithUpdateRevision(capsule, testRevision)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(capsule, pod, sts).
		WithStatusSubresource(capsule).
		Build()
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Name, Namespace: capsule.Namespace}, capsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}

	r := buildReconcilerForUpdateStatus(c)
	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capsule.Status.Phase != operatorv1alpha1.PhasePending {
		t.Errorf("expected PhasePending (ACTIVE is not a readiness substitute), got %s", capsule.Status.Phase)
	}
	if result.RequeueAfter != podReadinessRequeueAfter {
		t.Errorf("expected readiness requeue, got %v", result.RequeueAfter)
	}
}

func TestUpdateStatus_PendingToActive_PatchesCheckpoint(t *testing.T) {
	// A cold start that observes a ready ACTIVE pod jumps Pending→Active; the
	// genesis-initialized checkpoint must still be marked (isReadyPhase covers Active).
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	capsule.Generation = 2
	pod := newReadyPodWithPlatformStatus(capsule, testRevision, operatorv1alpha1.PlatformStatusActive)
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

	r := buildReconcilerForUpdateStatus(c)
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
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

// ── pod-watch wake predicate + mapping ───────────────────────────────────────

func TestPlatformStatusPodPredicate(t *testing.T) {
	p := platformStatusPodPredicate()
	psCond := func(reason string) corev1.PodCondition {
		return corev1.PodCondition{Type: operatorv1alpha1.PodConditionPlatformStatus, Status: corev1.ConditionTrue, Reason: reason}
	}
	readyCond := func(status corev1.ConditionStatus) corev1.PodCondition {
		return corev1.PodCondition{Type: corev1.PodReady, Status: status}
	}
	pod := func(conds ...corev1.PodCondition) *corev1.Pod {
		return &corev1.Pod{Status: corev1.PodStatus{Conditions: conds}}
	}

	base := pod(readyCond(corev1.ConditionTrue), psCond(operatorv1alpha1.PlatformStatusActive))
	if p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: base.DeepCopy()}) {
		t.Error("a no-op update must be dropped")
	}

	// A label-only change (no condition/readiness change) must be dropped.
	labelled := base.DeepCopy()
	labelled.Labels = map[string]string{"x": "y"}
	if p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: labelled}) {
		t.Error("a non-condition change (labels) must be dropped")
	}

	added := pod(readyCond(corev1.ConditionTrue))
	if !p.Update(event.UpdateEvent{ObjectOld: added, ObjectNew: base}) {
		t.Error("adding the platform-status condition must fire")
	}

	reasonChanged := pod(readyCond(corev1.ConditionTrue), psCond(operatorv1alpha1.PlatformStatusBehind))
	if !p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: reasonChanged}) {
		t.Error("a platform-status reason change must fire")
	}

	readinessFlipped := pod(readyCond(corev1.ConditionFalse), psCond(operatorv1alpha1.PlatformStatusActive))
	if !p.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: readinessFlipped}) {
		t.Error("a pod-readiness flip must fire")
	}

	if !p.Create(event.CreateEvent{Object: base}) {
		t.Error("a pod create must fire")
	}
	if !p.Delete(event.DeleteEvent{Object: base}) {
		t.Error("a pod delete must fire")
	}
	if p.Generic(event.GenericEvent{Object: base}) {
		t.Error("a generic event must be dropped")
	}
}

func TestMapPodToCapsule(t *testing.T) {
	capsule := newCapsuleForControllerTest()
	consensusPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      capsule.Name + "-0",
		Namespace: capsule.Namespace,
		Labels:    consensus.PodLabels(capsule),
	}}
	reqs := mapPodToCapsule(context.Background(), consensusPod)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request for a consensus pod, got %d", len(reqs))
	}
	if reqs[0].Namespace != capsule.Namespace || reqs[0].Name != capsule.Name {
		t.Errorf("mapped request = %s/%s, want %s/%s", reqs[0].Namespace, reqs[0].Name, capsule.Namespace, capsule.Name)
	}

	// A non-consensus pod maps to nothing.
	other := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "random", Namespace: "test", Labels: map[string]string{"app": "x"}}}
	if reqs := mapPodToCapsule(context.Background(), other); reqs != nil {
		t.Errorf("expected nil for a non-consensus pod, got %v", reqs)
	}

	// A consensus pod missing the node-name label maps to nothing (can't resolve the capsule).
	partial := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "test", Labels: map[string]string{consensus.LabelPartOf: consensus.PartOfConsensusNode}}}
	if reqs := mapPodToCapsule(context.Background(), partial); reqs != nil {
		t.Errorf("expected nil for a consensus pod without the node-name label, got %v", reqs)
	}
}

// ── UC-mandatory terminal error ─────────────────────────────────────────────

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

func TestFailUCSidecarRequired_SetsPhaseFailed(t *testing.T) {
	ctx := context.Background()
	scheme := newControllerTestScheme()
	capsule := newCapsuleForControllerTest()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule).WithStatusSubresource(capsule).Build()
	r := buildReconcilerForUpdateStatus(c)
	termErr := validateUCMandatory(capsule)
	if termErr == nil {
		t.Fatal("expected error")
	}
	if _, err := r.failUCSidecarRequired(ctx, capsule, termErr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got operatorv1alpha1.ConsensusCapsule
	_ = c.Get(ctx, client.ObjectKeyFromObject(capsule), &got)
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed, got %s", got.Status.Phase)
	}
	if got.Status.ObservedGeneration != capsule.Generation {
		t.Errorf("expected ObservedGeneration=%d, got %d", capsule.Generation, got.Status.ObservedGeneration)
	}
}

// ── markGenesisInitialized ──────────────────────────────────────────────────

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
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
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
	if _, err := r.updateStatus(ctx, capsule, capsule.Status.RunState); err != nil {
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
	result, err := r.updateStatus(ctx, capsule, capsule.Status.RunState)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected empty result for no-op update, got %+v", result)
	}
}

type fakeS6 struct {
	up  bool
	err error
}

func (f *fakeS6) Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	return nil
}
func (f *fakeS6) Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	return nil
}
func (f *fakeS6) IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	return f.up, f.err
}

func TestFinalizeManualStopped(t *testing.T) {
	ctx := context.Background()
	capsule := newCapsuleForControllerTest()
	r := &ConsensusCapsuleReconciler{}
	s6 := &fakeS6{}
	r.S6 = s6

	// A cold idle Manual node (never live) still reaches Stopped promptly.
	t.Run("cold idle node", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		var requeue bool
		s6.up = false
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhasePending, &requeue)
		if phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %v", phase)
		}
	})

	// An IsUp error does not produce Stopped (keeps prior phase + requeues).
	t.Run("IsUp error", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		var requeue bool
		s6.up = false
		s6.err = fmt.Errorf("exec failed")
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseActive {
			t.Errorf("expected PhaseActive, got %v", phase)
		}
		if !requeue {
			t.Errorf("expected requeue=true")
		}
	})

	// UNKNOWN sustained >= debounce with s6 down -> Stopped
	t.Run("sustained UNKNOWN s6 down", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
		})
		var requeue bool
		s6.up = false
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %v", phase)
		}
	})

	// UNKNOWN sustained >= debounce with s6 up -> Running
	t.Run("sustained UNKNOWN s6 up", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
		})
		var requeue bool
		s6.up = true
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseRunning {
			t.Errorf("expected PhaseRunning, got %v", phase)
		}
	})

	// UNKNOWN < debounce stays Active/Running — no Stopped flap
	t.Run("transient UNKNOWN", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-5 * time.Second)},
		})
		var requeue bool
		s6.up = false // shouldn't be called, but if it is it shouldn't hit Stopped
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseActive {
			t.Errorf("expected PhaseActive, got %v", phase)
		}
		if !requeue {
			t.Errorf("expected requeue=true for transient")
		}
	})
}
