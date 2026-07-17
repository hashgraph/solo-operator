// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

const (
	executeDigest    = operatorv1alpha1.Sha256Digest("sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	altExecuteDigest = operatorv1alpha1.Sha256Digest("sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
)

func newExecuteChecker(unreachable, mismatch map[string]bool) fakeUpgradeChecker {
	return fakeUpgradeChecker{unreachable: unreachable, mismatch: mismatch, defaultDigest: executeDigest}
}

// newExecuteForTest builds an Execute CR already in ReadyForProvisionerDaemon phase
// so single-reconcile tests skip initialization and go straight to the working state.
func newExecuteForTest() *operatorv1alpha1.NetworkUpgradeExecute {
	now := metav1.Now()
	return &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-execute",
			Namespace:  "default",
			Labels:     map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
			Finalizers: []string{FinalizerNetworkUpgradeExecute},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:         "test-orbit",
			OperationID:   "op-001",
			ConsensusNode: "node0",
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
		Status: operatorv1alpha1.NetworkUpgradeExecuteStatus{
			Phase:     operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon,
			StartTime: &now,
		},
	}
}

// newSucceededPrepare returns a Prepare CR in Succeeded phase for the given operationID.
func newSucceededPrepare(namespace, operationID string) *operatorv1alpha1.NetworkUpgradePrepare {
	return &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{Name: "prepare-" + operationID, Namespace: namespace},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			OperationID: operationID,
		},
		Status: operatorv1alpha1.NetworkUpgradePrepareStatus{
			Phase: operatorv1alpha1.PhaseSucceeded,
		},
	}
}

// newSucceededFreeze returns a Freeze CR in Succeeded phase for the given operationID.
func newSucceededFreeze(namespace, operationID string) *operatorv1alpha1.NetworkUpgradeFreeze {
	return &operatorv1alpha1.NetworkUpgradeFreeze{
		ObjectMeta: metav1.ObjectMeta{Name: "freeze-" + operationID, Namespace: namespace},
		Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
			OperationID: operationID,
		},
		Status: operatorv1alpha1.NetworkUpgradeFreezeStatus{
			Phase: operatorv1alpha1.PhaseSucceeded,
		},
	}
}

type executeReconcileResult struct {
	events  []string
	execute *operatorv1alpha1.NetworkUpgradeExecute
	result  ctrl.Result
	err     error
}

func reconcileExecuteWith(
	t *testing.T,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	checker registry.ImageChecker,
	extra ...client.Object,
) executeReconcileResult {
	t.Helper()
	scheme := newControllerTestScheme()

	objs := []client.Object{execute}
	objs = append(objs, extra...)

	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(execute)).
		Build()

	// Fake client stores spec from WithObjects but ignores status sub-resource fields.
	// Drive status for prerequisite CRs explicitly.
	for _, obj := range extra {
		switch v := obj.(type) {
		case *operatorv1alpha1.NetworkUpgradePrepare:
			_ = c.Status().Update(context.Background(), v)
		case *operatorv1alpha1.NetworkUpgradeFreeze:
			_ = c.Status().Update(context.Background(), v)
		}
	}

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	result, err := r.Reconcile(context.Background(), ctrl.Request{
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

	return executeReconcileResult{events: recorded, execute: &updated, result: result, err: err}
}

func withExecuteIndexes(b *fake.ClientBuilder) *fake.ClientBuilder {
	return b.
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradePrepare).Spec.OperationID}
		}).
		WithIndex(&operatorv1alpha1.NetworkUpgradeFreeze{}, fieldOperationID, func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreeze).Spec.OperationID}
		}).
		WithIndex(&operatorv1alpha1.NetworkUpgradeExecute{}, fieldOperationID, func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeExecute).Spec.OperationID}
		})
}

// prereqs returns the standard Prepare+Freeze CRs that satisfy prerequisites.
func prereqs() []client.Object {
	return []client.Object{
		newSucceededPrepare("default", "op-001"),
		newSucceededFreeze("default", "op-001"),
	}
}

// prereqsWithCapsule returns prereqs plus a node0 capsule.
func prereqsWithCapsule() []client.Object {
	return append(prereqs(), newOfflineCapsule("node0"))
}

// newOfflineCapsule returns a minimal ConsensusCapsule with Offline=true.
func newOfflineCapsule(name string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1, Log4j2ConfigRef: "network-log4j2",
			Offline: true,
			PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{},
			}}},
		},
	}
}

// newExecuteInBringOnline builds an Execute CR in PendingNodeUpgrade with the
// capsule already patched and brought online, so gate tests exercise only the
// final phase==Active gate. NodeUpgradeStartTime is recent so the backstop does
// not trip.
func newExecuteInBringOnline() *operatorv1alpha1.NetworkUpgradeExecute {
	now := metav1.Now()
	e := newExecuteForTest()
	e.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	e.Status.NodeUpgradeStartTime = &now
	e.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}
	setCondition(&e.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondConsensusCapsulePatched,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonPatchApplied.String(),
		LastTransitionTime: now,
	})
	setCondition(&e.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondCapsulesOnline,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonCapsulesOnline.String(),
		LastTransitionTime: now,
	})
	return e
}

// newOnlineCapsule returns a minimal online (Offline=false) ConsensusCapsule with
// the upgraded consensus-node image.
func newOnlineCapsule(name string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "reg-a", ImageName: "cn", ImageTag: "0.75.0"},
			}}},
		},
	}
}

// newActiveCapsule returns an online capsule reporting phase==Active with an ACTIVE
// platform status (the operator-written rollup produced by #1136).
func newActiveCapsule(name string) *operatorv1alpha1.ConsensusCapsule {
	c := newOnlineCapsule(name)
	c.Status.Phase = operatorv1alpha1.PhaseActive
	c.Status.PlatformStatus = operatorv1alpha1.PlatformStatusActive
	return c
}

// newRunningCapsule returns an online capsule that is Ready but not yet ACTIVE.
func newRunningCapsule(name string) *operatorv1alpha1.ConsensusCapsule {
	c := newOnlineCapsule(name)
	c.Status.Phase = operatorv1alpha1.PhaseRunning
	c.Status.PlatformStatus = operatorv1alpha1.PlatformStatusStartingUp
	return c
}

// newExecuteSTS builds a StatefulSet whose status carries the given current/update
// revisions and ready/updated replica counts, for exercising the revision guard.
func newExecuteSTS(name, currentRevision, updateRevision string, ready, updated int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Status: appsv1.StatefulSetStatus{
			CurrentRevision: currentRevision,
			UpdateRevision:  updateRevision,
			ReadyReplicas:   ready,
			UpdatedReplicas: updated,
		},
	}
}

func TestExecuteReconcile_InitializesStatus(t *testing.T) {
	execute := &operatorv1alpha1.NetworkUpgradeExecute{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-execute",
			Namespace:  "default",
			Labels:     map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
			Finalizers: []string{FinalizerNetworkUpgradeExecute},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit: "test-orbit", OperationID: "op-001", ConsensusNode: "node0",
			Images:          operatorv1alpha1.UpgradeImagesSpec{ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")}},
			SelectedSources: operatorv1alpha1.SelectedSources{ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest}},
		},
	}
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if !res.result.Requeue {
		t.Error("expected Requeue=true on initialization")
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Errorf("expected ReadyForProvisionerDaemon, got %s", res.execute.Status.Phase)
	}
	if res.execute.Status.StartTime == nil {
		t.Error("expected StartTime set")
	}
	if got := countContaining(res.events, "ReadyForProvisionerDaemon"); got != 1 {
		t.Errorf("expected one ReadyForProvisionerDaemon event, got %d in %v", got, res.events)
	}
}

func TestExecuteReconcile_OperationIDLabelMissing(t *testing.T) {
	execute := newExecuteForTest()
	execute.Labels = nil

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil))

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.execute.Status.Phase)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventOperationIDLabelInvalid.String()); got != 1 {
		t.Errorf("expected one OperationIDLabelInvalid event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.execute.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondOperationIDLabelValid=False, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonOperationIDLabelInvalid.String(), cond.Reason)
	}
}

func TestExecuteReconcile_OperationIDLabelMismatch(t *testing.T) {
	execute := newExecuteForTest()
	execute.Labels[operatorv1alpha1.LabelOperationID] = "op-does-not-match"

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil))

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.execute.Status.Phase)
	}
	cond := findCondition(res.execute.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid.String() {
		t.Errorf("expected reason %s, got %v", operatorv1alpha1.ReasonOperationIDLabelInvalid.String(), cond)
	}
}

func TestExecuteReconcile_TerminalPhaseIsNoOp(t *testing.T) {
	phases := []operatorv1alpha1.ExecutePhase{
		operatorv1alpha1.ExecutePhaseSucceeded,
		operatorv1alpha1.ExecutePhaseFailed,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			execute := newExecuteForTest()
			execute.Status.Phase = phase

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
			r.Checker = newExecuteChecker(nil, nil)

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
				t.Errorf("terminal CR should emit no events, got %v", recorded)
			}
		})
	}
}

func TestExecuteReconcile_WaitsForPrereqs(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePending
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil))
	// No Prepare/Freeze in fake client → should requeue after prereq delay, no error.
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.result.RequeueAfter != executePrereqRequeueAfter {
		t.Errorf("expected RequeueAfter=%v, got %v", executePrereqRequeueAfter, res.result.RequeueAfter)
	}
}

func newExecuteInPendingNodeUpgrade() *operatorv1alpha1.NetworkUpgradeExecute {
	now := metav1.Now()
	e := newExecuteForTest()
	e.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	e.Status.NodeUpgradeStartTime = &now
	return e
}

func TestExecuteReconcile_SelectedRegistryStillValid(t *testing.T) {
	execute := newExecuteInPendingNodeUpgrade()
	capsule := newOfflineCapsule("node0")
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), capsule)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if got := countContaining(res.events, "RegistryFailover"); got != 0 {
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
	execute := newExecuteInPendingNodeUpgrade()
	capsule := newOfflineCapsule("node0")
	checker := fakeUpgradeChecker{
		unreachable:   map[string]bool{"reg-a/cn:0.75.0": true},
		digest:        altExecuteDigest,
		defaultDigest: executeDigest,
	}
	res := reconcileExecuteWith(t, execute, checker, capsule)

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
	if res.execute.Status.SelectedSources == nil || res.execute.Status.SelectedSources.ConsensusNode == nil {
		t.Fatal("expected selectedSources updated after failover")
	}
	if res.execute.Status.SelectedSources.ConsensusNode.Image != "reg-b/cn:0.75.0" {
		t.Errorf("expected failover to reg-b, got %s", res.execute.Status.SelectedSources.ConsensusNode.Image)
	}
}

func TestExecuteReconcile_NoValidRegistry_TerminalFailure(t *testing.T) {
	execute := newExecuteInPendingNodeUpgrade()
	capsule := newOfflineCapsule("node0")
	checker := newExecuteChecker(map[string]bool{"reg-a/cn:0.75.0": true, "reg-b/cn:0.75.0": true}, nil)
	res := reconcileExecuteWith(t, execute, checker, capsule)

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
	if got := countContaining(res.events, "RegistryInvalid"); got == 0 {
		t.Errorf("expected RegistryInvalid event, got none in %v", res.events)
	}
}

func TestExecuteReconcile_ScalesDownCapsule(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePending
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "default",
			Labels:    map[string]string{common.LabelOrbit: "test-orbit"},
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			PodProperties:   operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"}}}},
		},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001")).
		WithStatusSubresource(execute)).Build()
	_ = c.Status().Update(context.Background(), newSucceededPrepare("default", "op-001"))
	_ = c.Status().Update(context.Background(), newSucceededFreeze("default", "op-001"))

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated operatorv1alpha1.ConsensusCapsule
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(capsule), &updated); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if !updated.Spec.Offline {
		t.Error("expected capsule Offline=true after scale-down")
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "PodScaledDown"); got == 0 {
		t.Errorf("expected PodScaledDown event, got none in %v", evts)
	}
}

func TestExecuteReconcile_DaemonResultFalse_TerminalFailure(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Conditions = []operatorv1alpha1.Condition{
		{
			Type:               operatorv1alpha1.CondDaemonResult,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonDaemonFileHashMismatch.String(),
			LastTransitionTime: metav1.Now(),
		},
	}

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected Failed, got %s", res.execute.Status.Phase)
	}
	if res.execute.Status.CompletionTime == nil {
		t.Error("expected CompletionTime set")
	}
}

func TestExecuteReconcile_DuplicateOperationID_SetsPhaseFailed(t *testing.T) {
	existing := newExecuteForTest()
	existing.Name = "existing"
	existing.UID = "uid-existing"
	existing.Status = operatorv1alpha1.NetworkUpgradeExecuteStatus{}

	execute := newExecuteForTest()
	execute.Name = "duplicate"
	execute.UID = "uid-duplicate"
	execute.Status = operatorv1alpha1.NetworkUpgradeExecuteStatus{}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existing, execute).
		WithStatusSubresource(existing, execute)).
		Build()

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
	})
	if err == nil {
		t.Error("expected error for duplicate operationId")
	}

	var got operatorv1alpha1.NetworkUpgradeExecute
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &got); getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected ExecutePhaseFailed, got %s", got.Status.Phase)
	}
	cond := findCondition(got.Status.Conditions, operatorv1alpha1.CondManifestValidated)
	if cond == nil {
		t.Fatal("ManifestValidated condition not set")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected ManifestValidated=False, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.ReasonDuplicateOpID.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonDuplicateOpID.String(), cond.Reason)
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

func TestExecuteReconcile_DaemonResultTrue_AdvancesToPendingInfraUpgrade(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Conditions = []operatorv1alpha1.Condition{
		{
			Type:               operatorv1alpha1.CondDaemonResult,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonDaemonSucceeded.String(),
			LastTransitionTime: metav1.Now(),
		},
	}

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	// Daemon success transitions to PendingInfraUpgrade on this tick; the next
	// tick (via Requeue) will run reconcilePendingInfraUpgrade and advance to
	// PendingNodeUpgrade.
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhasePendingInfraUpgrade {
		t.Errorf("expected PendingInfraUpgrade, got %s", res.execute.Status.Phase)
	}
	if !res.result.Requeue {
		t.Error("expected Requeue=true after advancing phase")
	}
}

func TestExecuteReconcile_PendingInfraUpgrade_WaitsForDaemon(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingInfraUpgrade
	// CondConfigCRsApplied not set — daemon hasn't written it yet.

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhasePendingInfraUpgrade {
		t.Errorf("expected PendingInfraUpgrade (still waiting), got %s", res.execute.Status.Phase)
	}
}

func TestExecuteReconcile_PendingInfraUpgrade_AdvancesToPendingNodeUpgrade(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingInfraUpgrade
	// Simulate daemon having written CondConfigCRsApplied=True.
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:   operatorv1alpha1.CondConfigCRsApplied,
		Status: metav1.ConditionTrue,
		Reason: operatorv1alpha1.ReasonDaemonSucceeded.String(),
	})

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhasePendingNodeUpgrade {
		t.Errorf("expected PendingNodeUpgrade, got %s", res.execute.Status.Phase)
	}
}

func TestExecuteReconcile_PendingNodeUpgrade_PatchesCapsule(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "default",
			Labels:    map[string]string{common.LabelOrbit: "test-orbit"},
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			Offline:         true,
			PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"},
			}}},
		},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule).
		WithStatusSubresource(execute)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	close(rec.Events)

	var updatedCapsule operatorv1alpha1.ConsensusCapsule
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(capsule), &updatedCapsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if !updatedCapsule.Spec.Offline {
		t.Error("capsule should remain Offline=true until all are patched and brought online together")
	}
	sv := updatedCapsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
	if sv == nil {
		t.Fatal("expected SoftwareVersion set")
	}
	if sv.ImageName != "cn" || sv.ImageTag != "0.75.0" {
		t.Errorf("expected cn:0.75.0, got %s/%s:%s", sv.Repository, sv.ImageName, sv.ImageTag)
	}

	var updatedExecute operatorv1alpha1.NetworkUpgradeExecute
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updatedExecute); err != nil {
		t.Fatalf("get execute: %v", err)
	}
	cond := findCondition(updatedExecute.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected ConsensusCapsulePatched=True, got %v", cond)
	}
}

func TestExecuteReconcile_PendingNodeUpgrade_BringsCapsuleOnline(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondConsensusCapsulePatched,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonPatchApplied.String(),
		LastTransitionTime: metav1.Now(),
	})

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			Offline:         true,
			PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "reg-a", ImageName: "cn", ImageTag: "0.75.0"},
			}}},
		},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule).
		WithStatusSubresource(execute)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updatedCapsule operatorv1alpha1.ConsensusCapsule
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(capsule), &updatedCapsule); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if updatedCapsule.Spec.Offline {
		t.Error("expected Offline=false after bring-online step")
	}

	var updatedExecute operatorv1alpha1.NetworkUpgradeExecute
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updatedExecute); err != nil {
		t.Fatalf("get execute: %v", err)
	}
	cond := findCondition(updatedExecute.Status.Conditions, operatorv1alpha1.CondCapsulesOnline)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected CapsulesOnline=True, got %v", cond)
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "CapsulesOnline"); got == 0 {
		t.Errorf("expected CapsulesOnline event, got none in %v", evts)
	}
}

func TestExecuteReconcile_PendingNodeUpgrade_WaitsForActive(t *testing.T) {
	execute := newExecuteInBringOnline()

	// Pod is Ready but the node has not yet reached ACTIVE (phase=Running): the
	// upgrade must not be reported Succeeded on mere pod readiness.
	capsule := newRunningCapsule("node0")
	sts := newExecuteSTS("node0", "rev-new", "rev-new", 1, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, capsule, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), capsule); err != nil {
		t.Fatalf("pre-set capsule status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = events.NewFakeRecorder(8)
	r.Checker = newExecuteChecker(nil, nil)

	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != executeReadyRequeueAfter {
		t.Errorf("expected RequeueAfter=%v, got %v", executeReadyRequeueAfter, result.RequeueAfter)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded {
		t.Error("should not succeed while the node is Ready but not yet ACTIVE")
	}
}

// TestExecuteReconcile_PendingNodeUpgrade_NotFooledByPreUpgradeActive verifies the
// revision guard: a capsule reporting phase==Active while the StatefulSet is still
// mid-roll (currentRevision != updateRevision) is a stale pre-upgrade reading and
// must not report Succeeded.
func TestExecuteReconcile_PendingNodeUpgrade_NotFooledByPreUpgradeActive(t *testing.T) {
	execute := newExecuteInBringOnline()

	capsule := newActiveCapsule("node0")
	// Mid-roll: the update revision differs from the current revision, so any ACTIVE
	// reading belongs to the pre-upgrade pod.
	sts := newExecuteSTS("node0", "rev-old", "rev-new", 1, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, capsule, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), capsule); err != nil {
		t.Fatalf("pre-set capsule status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = events.NewFakeRecorder(8)
	r.Checker = newExecuteChecker(nil, nil)

	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != executeReadyRequeueAfter {
		t.Errorf("expected RequeueAfter=%v, got %v", executeReadyRequeueAfter, result.RequeueAfter)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded {
		t.Error("must not succeed on a pre-upgrade ACTIVE while the StatefulSet is mid-roll")
	}
}

// TestExecuteReconcile_PendingNodeUpgrade_CatastrophicFastFail verifies that a
// CATASTROPHIC_FAILURE on the post-upgrade node fails the upgrade fast, before the
// backstop and without waiting for a readiness that will never come.
func TestExecuteReconcile_PendingNodeUpgrade_CatastrophicFastFail(t *testing.T) {
	execute := newExecuteInBringOnline()

	capsule := newActiveCapsule("node0")
	capsule.Status.Phase = operatorv1alpha1.PhaseFailed
	capsule.Status.PlatformStatus = operatorv1alpha1.PlatformStatusCatastrophicFailure
	capsule.Status.PlatformStatusRevision = "rev-new"
	// The new-revision pod was created but is crash-looping: not Ready, roll not
	// complete (updatedReplicas=1, readyReplicas=0).
	sts := newExecuteSTS("node0", "rev-old", "rev-new", 0, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, capsule, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), capsule); err != nil {
		t.Fatalf("pre-set capsule status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected Failed on CATASTROPHIC_FAILURE, got %s", updated.Status.Phase)
	}
	if updated.Status.CompletionTime == nil {
		t.Error("expected CompletionTime set on catastrophic failure")
	}
	if cond := findCondition(updated.Status.Conditions, operatorv1alpha1.CondNodesUpgraded); cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected NodesUpgraded=False, got %v", cond)
	}
	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "UpgradeFailed"); got == 0 {
		t.Errorf("expected UpgradeFailed event, got none in %v", evts)
	}
}

func TestExecuteReconcile_PendingNodeUpgrade_SucceedsWhenActiveOnNewRevision(t *testing.T) {
	execute := newExecuteInBringOnline()

	// The node reached ACTIVE (operator-written phase) on the fully-rolled
	// post-upgrade revision: currentRevision has caught up to updateRevision with a
	// Ready replica.
	capsule := newActiveCapsule("node0")
	sts := newExecuteSTS("node0", "rev-new", "rev-new", 1, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, capsule, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), capsule); err != nil {
		t.Fatalf("pre-set capsule status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase != operatorv1alpha1.ExecutePhaseSucceeded {
		t.Errorf("expected Succeeded, got %s", updated.Status.Phase)
	}
	if cond := findCondition(updated.Status.Conditions, operatorv1alpha1.CondNodesUpgraded); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected NodesUpgraded=True, got %v", cond)
	}
	if updated.Status.CompletionTime == nil {
		t.Error("expected CompletionTime set")
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "UpgradeSucceeded"); got == 0 {
		t.Errorf("expected UpgradeSucceeded event, got none in %v", evts)
	}
}

func TestExecuteReconcile_NodeActiveDeadlineExceeded(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondConsensusCapsulePatched,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonPatchApplied.String(),
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondCapsulesOnline,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonCapsulesOnline.String(),
		LastTransitionTime: metav1.Now(),
	})
	pastTime := metav1.NewTime(time.Now().Add(-30 * time.Minute))
	execute.Status.NodeUpgradeStartTime = &pastTime

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			PodProperties:   operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "reg-a", ImageName: "cn", ImageTag: "0.75.0"}}}},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)
	r.NodeActiveTimeout = 5 * time.Minute // shorter than the 30m elapsed

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Error("expected RequeueAfter > 0 (non-terminal, keep waiting)")
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase != operatorv1alpha1.ExecutePhasePendingNodeUpgrade {
		t.Errorf("expected PendingNodeUpgrade (non-terminal timeout), got %s", updated.Status.Phase)
	}
	if updated.Status.CompletionTime != nil {
		t.Error("expected no CompletionTime (upgrade still in progress)")
	}
	if cond := findCondition(updated.Status.Conditions, operatorv1alpha1.CondNodeActiveDeadlineExceeded); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("expected NodeActiveDeadlineExceeded=True, got %v", cond)
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "NodeActiveDeadlineExceeded"); got == 0 {
		t.Errorf("expected NodeActiveDeadlineExceeded event, got none in %v", evts)
	}
}

func TestExecuteReconcile_NodeActiveDeadlineExceeded_IdempotentOnRequeue(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondConsensusCapsulePatched,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonPatchApplied.String(),
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondCapsulesOnline,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonCapsulesOnline.String(),
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondNodeActiveDeadlineExceeded,
		Status:  metav1.ConditionTrue,
		Reason:  operatorv1alpha1.ReasonNodeActiveDeadlineExceeded.String(),
		Message: "already warned",
	})
	pastTime := metav1.NewTime(time.Now().Add(-60 * time.Minute))
	execute.Status.NodeUpgradeStartTime = &pastTime

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1,
			Log4j2ConfigRef: "network-log4j2",
			PodProperties:   operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "reg-a", ImageName: "cn", ImageTag: "0.75.0"}}}},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)
	r.NodeActiveTimeout = 5 * time.Minute

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Error("expected RequeueAfter > 0")
	}

	// No new events should be emitted — the condition was already set.
	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "NodeActiveDeadlineExceeded"); got != 0 {
		t.Errorf("expected no duplicate NodeActiveDeadlineExceeded event on requeue, got %d in %v", got, evts)
	}
}

// TestExecuteReconcile_StaleCatastrophic_NoFastFail verifies that a
// CATASTROPHIC_FAILURE reading from the pre-upgrade pod (stale
// platformStatusRevision) does not fast-fail the upgrade. The backstop timeout
// must handle it instead.
func TestExecuteReconcile_StaleCatastrophic_NoFastFail(t *testing.T) {
	execute := newExecuteInBringOnline()

	capsule := newActiveCapsule("node0")
	capsule.Status.Phase = operatorv1alpha1.PhaseFailed
	capsule.Status.PlatformStatus = operatorv1alpha1.PlatformStatusCatastrophicFailure
	capsule.Status.PlatformStatusRevision = "rev-old"
	sts := newExecuteSTS("node0", "rev-old", "rev-new", 0, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts).
		WithStatusSubresource(execute, capsule, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), capsule); err != nil {
		t.Fatalf("pre-set capsule status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	if updated.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		t.Error("stale CATASTROPHIC_FAILURE must not fast-fail the upgrade")
	}
}

// applyImagePatch unit tests — test the three-rule logic directly without a full reconcile.

func newExecuteWithImages(images operatorv1alpha1.UpgradeImagesSpec, selected operatorv1alpha1.SelectedSources) *operatorv1alpha1.NetworkUpgradeExecute {
	e := newExecuteForTest()
	e.Spec.Images = images
	e.Status.SelectedSources = &selected
	return e
}

func TestApplyImagePatch_ConsensusNodeUpgrade(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
				Version:    "0.75.0",
				Registries: registriesFor("reg-a/cn:0.75.0"),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old", ImageName: "cn", ImageTag: "0.74.0"},
					},
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	sv := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
	if sv == nil {
		t.Fatal("expected SoftwareVersion set")
	}
	if sv.Repository != "reg-a" || sv.ImageName != "cn" || sv.ImageTag != "0.75.0" {
		t.Errorf("unexpected sv: %+v", sv)
	}
}

func TestApplyImagePatch_CheetahSidecar_Upgrade(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			RecordStreamUploader: &operatorv1alpha1.UpgradeComponentSpec{
				Version:    "1.0.0",
				Enabled:    boolPtr(true),
				Registries: registriesFor("reg-a/rsu:1.0.0"),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode:        &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
			RecordStreamUploader: &operatorv1alpha1.SelectedSource{Image: "reg-a/rsu:1.0.0", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"}},
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	ru := capsule.Spec.PodProperties.Containers.RecordsUploader
	if ru == nil {
		t.Fatal("expected RecordsUploader allocated")
	}
	if ru.SoftwareVersion == nil {
		t.Fatal("expected SoftwareVersion set on RecordsUploader")
	}
	if ru.SoftwareVersion.ImageName != "rsu" || ru.SoftwareVersion.ImageTag != "1.0.0" {
		t.Errorf("unexpected SoftwareVersion: %+v", ru.SoftwareVersion)
	}
}

func TestApplyImagePatch_CheetahSidecar_Remove(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			RecordStreamUploader: &operatorv1alpha1.UpgradeComponentSpec{
				Version: "1.0.0",
				Enabled: boolPtr(false),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
		},
	)
	existing := &operatorv1alpha1.CheetahUploader{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "rsu", ImageTag: "0.9.0"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode:   operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"}},
					RecordsUploader: existing,
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	if capsule.Spec.PodProperties.Containers.RecordsUploader != nil {
		t.Error("expected RecordsUploader=nil after remove")
	}
}

func TestApplyImagePatch_StreamSidecar_Upgrade(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			BackupUploader: &operatorv1alpha1.UpgradeComponentSpec{
				Version:    "2.0.0",
				Enabled:    boolPtr(true),
				Registries: registriesFor("reg-a/bu:2.0.0"),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode:  &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
			BackupUploader: &operatorv1alpha1.SelectedSource{Image: "reg-a/bu:2.0.0", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"}},
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	bu := capsule.Spec.PodProperties.Containers.BackupUploader
	if bu == nil {
		t.Fatal("expected BackupUploader allocated")
	}
	if bu.SoftwareVersion == nil {
		t.Fatal("expected SoftwareVersion set on BackupUploader")
	}
	if bu.SoftwareVersion.ImageName != "bu" || bu.SoftwareVersion.ImageTag != "2.0.0" {
		t.Errorf("unexpected SoftwareVersion: %+v", bu.SoftwareVersion)
	}
}

func TestApplyImagePatch_StreamSidecar_Remove(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			BackupUploader: &operatorv1alpha1.UpgradeComponentSpec{
				Version: "2.0.0",
				Enabled: boolPtr(false),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
		},
	)
	existing := &operatorv1alpha1.StreamUploader{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "bu", ImageTag: "1.9.0"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode:  operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"}},
					BackupUploader: existing,
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	if capsule.Spec.PodProperties.Containers.BackupUploader != nil {
		t.Error("expected BackupUploader=nil after remove")
	}
}

func TestApplyImagePatch_OmittedSidecar_NoChange(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			// RecordStreamUploader deliberately omitted
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
		},
	)
	existing := &operatorv1alpha1.CheetahUploader{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "rsu", ImageTag: "0.9.0"}}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode:   operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"}},
					RecordsUploader: existing,
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	r.applyImagePatch(execute, capsule)

	if capsule.Spec.PodProperties.Containers.RecordsUploader != existing {
		t.Error("expected RecordsUploader unchanged when component omitted from spec")
	}
}
