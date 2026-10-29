// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
			Labels:     map[string]string{operatorv1alpha1.LabelOperationID: "op-001", operatorv1alpha1.LabelStep: operatorv1alpha1.StepExecute},
			Finalizers: []string{FinalizerNetworkUpgradeExecute},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit:            "test-orbit",
			OperationID:      "op-001",
			ConsensusCapsule: "node0",
			Images: operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.ImageComponentSpec{
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
	client  client.Client
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

	// A ConsensusCapsule always references an existing Orbit (spec.orbit is
	// required, and the Orbit finalizer cascade-deletes its capsules), so seed the
	// default test Orbit unless the test provides its own.
	hasOrbit := false
	for _, o := range extra {
		if _, ok := o.(*operatorv1alpha1.Orbit); ok {
			hasOrbit = true
			break
		}
	}
	if !hasOrbit {
		objs = append(objs, newTestOrbit())
	}

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

	return executeReconcileResult{events: recorded, execute: &updated, result: result, err: err, client: c}
}

// newTestOrbit returns the minimal cluster-scoped Orbit that every test capsule
// references via spec.orbit ("test-orbit"). getOrbit requires the referenced
// Orbit to exist, mirroring production where a capsule cannot outlive its Orbit.
func newTestOrbit() *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
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
// revisions and ready/updated replica counts, for exercising the revision guard. Its
// template runs the consensus-node image selected by newExecuteInBringOnline.
func newExecuteSTS(name, currentRevision, updateRevision string, ready, updated int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "consensus-node", Image: "reg-a/cn:0.75.0@" + string(executeDigest)}},
		}}},
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
			Labels:     map[string]string{operatorv1alpha1.LabelOperationID: "op-001", operatorv1alpha1.LabelStep: operatorv1alpha1.StepExecute},
			Finalizers: []string{FinalizerNetworkUpgradeExecute},
		},
		Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
			Orbit: "test-orbit", OperationID: "op-001", ConsensusCapsule: "node0",
			Images:          operatorv1alpha1.UpgradeImagesSpec{ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")}},
			SelectedSources: operatorv1alpha1.SelectedSources{ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest}},
		},
	}
	// Orbit with ProvisionerDaemonEnabled=false → cluster-only. The operator sets
	// ReadyForProvisionerDaemon (pod stays up so the in-pod UC can deploy config
	// CRs and advance the phase).
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil),
		append(prereqsWithCapsule(), newClusterOnlyOrbit())...)

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

// newMainnetOrbit returns an Orbit with the host-level provisioner daemon
// enabled, so ResolveUCMode reports mainnet.
func newMainnetOrbit() *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: &operatorv1alpha1.OrbitConsensus{ProvisionerDaemonEnabled: true},
		},
	}
}

// newClusterOnlyOrbit returns an Orbit with the provisioner daemon disabled, so
// ResolveUCMode reports cluster-only. The Orbit CR exists in both modes; only
// ProvisionerDaemonEnabled distinguishes them.
func newClusterOnlyOrbit() *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: &operatorv1alpha1.OrbitConsensus{ProvisionerDaemonEnabled: false},
		},
	}
}

// TestExecuteReconcile_MainnetInfraUpgradeScalesDownBeforeReady asserts that in
// mainnet mode WITH an infra upgrade required (declared provisioner version differs
// from the Orbit annotation), the operator scales the capsule down (offline=true) as
// it enters ReadyForProvisionerDaemon, so the host provisioner daemon operates on a
// stopped node during the infra teardown.
func TestExecuteReconcile_MainnetInfraUpgradeScalesDownBeforeReady(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = "" // start at initialization, before ReadyForProvisionerDaemon
	// Declared provisioner version differs from the (absent) Orbit annotation →
	// requiresInfraAccess=true → skipScaleDown=false → scale down at init.
	execute.Spec.Provisioner = &operatorv1alpha1.DeclaredProvisionerVersions{DaemonVersion: "0.33.0"}
	capsule := newOnlineCapsule("node0")

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil),
		newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"),
		capsule, newMainnetOrbit())

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Errorf("expected ReadyForProvisionerDaemon, got %s", res.execute.Status.Phase)
	}
	var updated operatorv1alpha1.ConsensusCapsule
	if err := res.client.Get(context.Background(), client.ObjectKey{Name: "node0", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if !updated.Spec.Offline {
		t.Error("expected capsule Offline=true at ReadyForProvisionerDaemon in mainnet mode with an infra upgrade")
	}
}

// TestExecuteReconcile_MainnetConfigOnlyKeepsPodUpAtReady asserts that a config-only
// mainnet upgrade (no provisioner version declared, default SkipScaleDown=true) does
// NOT scale the pod down at init — it behaves like cluster-only, because the daemon's
// config step is pod-state-agnostic. This is the option-B invariant: skipScaleDown is
// honored in both modes when no infra work is required.
func TestExecuteReconcile_MainnetConfigOnlyKeepsPodUpAtReady(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = "" // start at initialization
	// No provisioner declared → requiresInfraAccess=false → skipScaleDown=true (default).
	capsule := newOnlineCapsule("node0")

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil),
		newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"),
		capsule, newMainnetOrbit())

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Errorf("expected ReadyForProvisionerDaemon, got %s", res.execute.Status.Phase)
	}
	var updated operatorv1alpha1.ConsensusCapsule
	if err := res.client.Get(context.Background(), client.ObjectKey{Name: "node0", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if updated.Spec.Offline {
		t.Error("expected capsule to stay online at ReadyForProvisionerDaemon for a config-only mainnet upgrade (skipScaleDown honored)")
	}
}

// TestExecuteReconcile_ClusterOnlyKeepsPodUpAtReady asserts that in cluster-only
// mode the operator sets ReadyForProvisionerDaemon WITHOUT scaling the pod down.
// The pod (and its in-pod UC sidecar) must stay alive at this phase, because only
// the UC can read the upgrade package from the pod's PVC to deploy config CRs and
// then advance the phase. Scale-down is deferred to reconcilePendingNodeUpgrade.
func TestExecuteReconcile_ClusterOnlyKeepsPodUpAtReady(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = "" // start at initialization
	capsule := newOnlineCapsule("node0")

	// Orbit exists with ProvisionerDaemonEnabled=false → cluster-only.
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil),
		newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"),
		capsule, newClusterOnlyOrbit())

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Errorf("expected ReadyForProvisionerDaemon, got %s", res.execute.Status.Phase)
	}
	var updated operatorv1alpha1.ConsensusCapsule
	if err := res.client.Get(context.Background(), client.ObjectKey{Name: "node0", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if updated.Spec.Offline {
		t.Error("expected capsule to stay online at ReadyForProvisionerDaemon in cluster-only mode (pod must survive for the UC to deploy config CRs)")
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

func TestExecuteReconcile_StepLabelInvalid(t *testing.T) {
	execute := newExecuteForTest()
	// Wrong step value for an Execute CR — must be rejected terminally (#1133).
	execute.Labels[operatorv1alpha1.LabelStep] = operatorv1alpha1.StepPrepare

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil))

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.execute.Status.Phase)
	}
	cond := findCondition(res.execute.Status.Conditions, operatorv1alpha1.CondStepLabelValid)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != operatorv1alpha1.ReasonStepLabelInvalid.String() {
		t.Errorf("expected CondStepLabelValid=False reason %s, got %v", operatorv1alpha1.ReasonStepLabelInvalid.String(), cond)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventStepLabelInvalid.String()); got != 1 {
		t.Errorf("expected one StepLabelInvalid event, got %d", got)
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
	if res.result.RequeueAfter != operatorv1alpha1.DefaultExecutePrereqRequeueInterval {
		t.Errorf("expected RequeueAfter=%v, got %v", operatorv1alpha1.DefaultExecutePrereqRequeueInterval, res.result.RequeueAfter)
	}
}

// TestExecuteReconcile_OrbitOverridesRequeue verifies that
// Orbit.spec.consensus.upgrade.* overrides the hardcoded requeue defaults,
// exercising the getCapsule → getOrbit → ResolveUpgradeConfig wiring.
func TestExecuteReconcile_OrbitOverridesRequeue(t *testing.T) {
	const override = 3 * time.Second
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: &operatorv1alpha1.OrbitConsensus{
				Upgrade: &operatorv1alpha1.ConsensusNodeUpgrade{
					PrereqRequeueInterval: &metav1.Duration{Duration: override},
				},
			},
		},
	}
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePending
	// node0 capsule references test-orbit, so config resolves through the Orbit.
	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), orbit, newOfflineCapsule("node0"))
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	// No Prepare in fake client → requeues after the Orbit-provided prereq interval.
	if res.result.RequeueAfter != override {
		t.Errorf("expected RequeueAfter=%v (Orbit override), got %v", override, res.result.RequeueAfter)
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
	execute.Spec.Provisioner = &operatorv1alpha1.DeclaredProvisionerVersions{DaemonVersion: "0.33.0"}
	// Scale-down now happens at PendingNodeUpgrade (after UC advances the phase),
	// not at Pending. DaemonResult=True is required to reach PendingNodeUpgrade.
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
			PodProperties:   operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"}}}},
		},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"), newTestOrbit()).
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

func TestExecuteReconcile_SkipScaleDown_PatchesInPlace(t *testing.T) {
	execute := newExecuteForTest()
	// No Provisioner versions declared → rolling restart path is used
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
			PodProperties:   operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"}}}},
		},
	}

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"), newTestOrbit()).
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
	if updated.Spec.Offline {
		t.Error("expected capsule Offline=false with rolling restart (no scale-down)")
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "PodScaledDown"); got != 0 {
		t.Errorf("expected no PodScaledDown event with rolling restart, got %d in %v", got, evts)
	}
}

// TestExecuteReconcile_PatchesIndependentlyOfPeers is the regression test for the
// cross-node barrier deadlock: node0 must scale down and patch its own capsule
// without waiting for sibling nodes. Each node's UC (which advances the phase)
// dies when its pod is scaled to zero, so a sibling that has not yet reached
// PendingNodeUpgrade can never help clear a shared barrier — node0 would sit
// Offline forever. With the barrier removed, node0 proceeds regardless.
func TestExecuteReconcile_PatchesIndependentlyOfPeers(t *testing.T) {
	execute := newExecuteForTest()
	execute.Spec.Provisioner = &operatorv1alpha1.DeclaredProvisionerVersions{DaemonVersion: "0.33.0"}
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
	execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0", Digest: executeDigest},
	}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default", Labels: map[string]string{common.LabelOrbit: "test-orbit"}},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit: "test-orbit", NodeId: 0, AccountId: "0.0.3", Weight: 1, Log4j2ConfigRef: "network-log4j2",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"}}}},
		},
	}

	// Sibling node1: same operationId, still online, CR not yet patched. Under the
	// old barrier this blocked node0; now it must be irrelevant.
	sibling := newExecuteForTest()
	sibling.ObjectMeta = metav1.ObjectMeta{
		Name: "upgrade-op-001-node1", Namespace: "default", UID: "sibling-uid",
		Labels: map[string]string{operatorv1alpha1.LabelOperationID: "op-001", operatorv1alpha1.LabelStep: operatorv1alpha1.StepExecute},
	}
	sibling.Spec.ConsensusCapsule = "node1"
	siblingCapsule := newOnlineCapsule("node1")
	siblingCapsule.Spec.NodeId = 1

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sibling, siblingCapsule, newSucceededPrepare("default", "op-001"), newSucceededFreeze("default", "op-001"), newTestOrbit()).
		WithStatusSubresource(execute)).Build()

	rec := events.NewFakeRecorder(32)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated operatorv1alpha1.ConsensusCapsule
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(capsule), &updated); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if !updated.Spec.Offline {
		t.Error("expected node0 capsule Offline=true despite online sibling — cross-node barrier must be gone")
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
		t.Error("expected error for duplicate operationId+consensusNode")
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

func TestExecuteReconcile_SameOperationID_DifferentNode_NotDuplicate(t *testing.T) {
	existing := newExecuteForTest()
	existing.Name = "execute-node0"
	existing.UID = "uid-node0"
	existing.Spec.ConsensusCapsule = "node0"
	existing.Status = operatorv1alpha1.NetworkUpgradeExecuteStatus{}

	execute := newExecuteForTest()
	execute.Name = "execute-node1"
	execute.UID = "uid-node1"
	execute.Spec.ConsensusCapsule = "node1"
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

	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(execute),
	})

	var got operatorv1alpha1.NetworkUpgradeExecute
	if getErr := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &got); getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		cond := findCondition(got.Status.Conditions, operatorv1alpha1.CondManifestValidated)
		if cond != nil && cond.Reason == operatorv1alpha1.ReasonDuplicateOpID.String() {
			t.Error("execute CR for a different consensusNode should not be rejected as a duplicate")
		}
	}
}

func TestExecuteReconcile_DaemonResultTrue_WaitsForDaemonToAdvancePhase(t *testing.T) {
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
	// Per HIP-1496 the provisioner (daemon / provisioner-proxy) — not the operator —
	// owns the PendingInfraUpgrade / PendingNodeUpgrade phase transitions. On a
	// positive DaemonResult the operator only waits for the phase to advance; it
	// must not write it and race the provisioner.
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Errorf("expected phase to stay ReadyForProvisionerDaemon, got %s", res.execute.Status.Phase)
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

func TestExecuteReconcile_PendingInfraUpgrade_DoesNotAdvancePhase(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingInfraUpgrade
	// Even with CondConfigCRsApplied=True, the operator must NOT advance to
	// PendingNodeUpgrade — the provisioner (daemon / provisioner-proxy) owns that
	// transition per HIP-1496. The operator only waits.
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:   operatorv1alpha1.CondConfigCRsApplied,
		Status: metav1.ConditionTrue,
		Reason: operatorv1alpha1.ReasonDaemonSucceeded.String(),
	})

	res := reconcileExecuteWith(t, execute, newExecuteChecker(nil, nil), prereqsWithCapsule()...)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.execute.Status.Phase != operatorv1alpha1.ExecutePhasePendingInfraUpgrade {
		t.Errorf("expected phase to stay PendingInfraUpgrade (daemon owns the advance), got %s", res.execute.Status.Phase)
	}
}

func TestExecuteReconcile_PendingNodeUpgrade_PatchesCapsule(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
		WithObjects(execute, capsule, newTestOrbit()).
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
	execute.Spec.Provisioner = &operatorv1alpha1.DeclaredProvisionerVersions{DaemonVersion: "0.33.0"}
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
		WithObjects(execute, capsule, newTestOrbit()).
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

// TestExecuteReconcile_PendingNodeUpgrade_BringsCapsuleOnlineWhenSkipScaleDown is
// the regression test for the mainnet strand-at-zero bug: mainnet mode scales the
// capsule offline unconditionally at init, but this upgrade declares no provisioner
// version bump, so requiresInfraAccess is false and skipScaleDown resolves to the
// Orbit default true. The bring-online step must key off the capsule's Offline flag,
// not skipScaleDown — otherwise the pod is stranded at 0 replicas forever.
func TestExecuteReconcile_PendingNodeUpgrade_BringsCapsuleOnlineWhenSkipScaleDown(t *testing.T) {
	execute := newExecuteForTest()
	// No Provisioner versions declared → requiresInfraAccess=false → with the Orbit
	// default SkipScaleDown=true, skipScaleDown=true. The capsule is nonetheless
	// Offline=true (mainnet init scaled it down before the daemon handoff).
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
		WithObjects(execute, capsule, newMainnetOrbit()).
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
		t.Error("expected Offline=false: an offline capsule must be brought online even when skipScaleDown=true")
	}

	var updatedExecute operatorv1alpha1.NetworkUpgradeExecute
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updatedExecute); err != nil {
		t.Fatalf("get execute: %v", err)
	}
	cond := findCondition(updatedExecute.Status.Conditions, operatorv1alpha1.CondCapsulesOnline)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected CapsulesOnline=True, got %v", cond)
	}
	if cond.Message != "ConsensusCapsule brought online" {
		t.Errorf("expected 'brought online' message when recovering an offline capsule, got %q", cond.Message)
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
		WithObjects(execute, capsule, sts, newTestOrbit()).
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
	if result.RequeueAfter != operatorv1alpha1.DefaultExecuteNodeReadyRequeueInterval {
		t.Errorf("expected RequeueAfter=%v, got %v", operatorv1alpha1.DefaultExecuteNodeReadyRequeueInterval, result.RequeueAfter)
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
		WithObjects(execute, capsule, sts, newTestOrbit()).
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
	if result.RequeueAfter != operatorv1alpha1.DefaultExecuteNodeReadyRequeueInterval {
		t.Errorf("expected RequeueAfter=%v, got %v", operatorv1alpha1.DefaultExecuteNodeReadyRequeueInterval, result.RequeueAfter)
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
	// The new-revision pod was created but is crash-looping: not Ready, roll not
	// complete (updatedReplicas=1, readyReplicas=0). The operator only reflects the
	// live pod's condition, so CATASTROPHIC here belongs to the post-upgrade node.
	sts := newExecuteSTS("node0", "rev-old", "rev-new", 0, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts, newTestOrbit()).
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
	// Ready replica (phase==Active already implies a current-revision ACTIVE reading).
	capsule := newActiveCapsule("node0")
	sts := newExecuteSTS("node0", "rev-new", "rev-new", 1, 1)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts, newTestOrbit()).
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
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
		WithObjects(execute, capsule, sts, newTestOrbit()).
		WithStatusSubresource(execute, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	alertInterval := 2 * time.Minute
	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)
	r.NodeActiveTimeout = 5 * time.Minute // shorter than the 30m elapsed
	r.NodeActiveAlertInterval = alertInterval

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter != alertInterval {
		t.Errorf("expected RequeueAfter=%v, got %v", alertInterval, res.RequeueAfter)
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

func TestExecuteReconcile_NodeActiveDeadlineExceeded_PeriodicAlert(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
	// LastTransitionTime far in the past so the alert interval has elapsed and the
	// time guard lets the alert through.
	oldAlertTime := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	execute.Status.Conditions = append(execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondNodeActiveDeadlineExceeded,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonNodeActiveDeadlineExceeded.String(),
		Message:            "already warned",
		LastTransitionTime: oldAlertTime,
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
		WithObjects(execute, capsule, sts, newTestOrbit()).
		WithStatusSubresource(execute, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	alertInterval := 2 * time.Minute
	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)
	r.NodeActiveTimeout = 5 * time.Minute
	r.NodeActiveAlertInterval = alertInterval

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter != alertInterval {
		t.Errorf("expected RequeueAfter=%v, got %v", alertInterval, res.RequeueAfter)
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "NodeActiveDeadlineExceeded"); got != 1 {
		t.Errorf("expected 1 periodic NodeActiveDeadlineExceeded event, got %d in %v", got, evts)
	}
}

func TestExecuteReconcile_NodeActiveDeadlineExceeded_ShortCircuitWithinInterval(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	// NodeUpgradeStartTime is pre-set: the daemon advances the phase and the
	// operator records this timestamp on first observation, then requeues. Setting
	// it here exercises the capsule-patching work in a single reconcile tick.
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
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
	// Condition was set very recently (now) — within the alert interval.
	execute.Status.Conditions = append(execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondNodeActiveDeadlineExceeded,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonNodeActiveDeadlineExceeded.String(),
		Message:            "already warned",
		LastTransitionTime: metav1.Now(),
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
		WithObjects(execute, capsule, sts, newTestOrbit()).
		WithStatusSubresource(execute, sts)).Build()
	if err := c.Status().Update(context.Background(), execute); err != nil {
		t.Fatalf("pre-set execute status: %v", err)
	}
	if err := c.Status().Update(context.Background(), sts); err != nil {
		t.Fatalf("pre-set sts status: %v", err)
	}

	alertInterval := 2 * time.Minute
	rec := events.NewFakeRecorder(8)
	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newExecuteChecker(nil, nil)
	r.NodeActiveTimeout = 5 * time.Minute
	r.NodeActiveAlertInterval = alertInterval

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)})
	close(rec.Events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > alertInterval {
		t.Errorf("expected RequeueAfter in (0, %v], got %v", alertInterval, res.RequeueAfter)
	}

	var evts []string
	for e := range rec.Events {
		evts = append(evts, e)
	}
	if got := countContaining(evts, "NodeActiveDeadlineExceeded"); got != 0 {
		t.Errorf("expected no event (short-circuited within interval), got %d in %v", got, evts)
	}
}

// TestExecuteReconcile_CatastrophicBeforeNewPod_NoFastFail verifies the surviving
// newRevisionPodCreated guard: a CATASTROPHIC_FAILURE while the StatefulSet has not
// yet produced a post-upgrade pod (updatedReplicas=0) must not fast-fail the upgrade
// — the reading cannot be attributed to the post-upgrade node. The backstop handles it.
func TestExecuteReconcile_CatastrophicBeforeNewPod_NoFastFail(t *testing.T) {
	execute := newExecuteInBringOnline()

	capsule := newActiveCapsule("node0")
	capsule.Status.Phase = operatorv1alpha1.PhaseFailed
	capsule.Status.PlatformStatus = operatorv1alpha1.PlatformStatusCatastrophicFailure
	// No new-revision pod created yet (updatedReplicas=0): newRevisionPodCreated is false.
	sts := newExecuteSTS("node0", "rev-old", "rev-new", 0, 0)

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts, newTestOrbit()).
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
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{
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
	// The verified digest must be carried onto the SoftwareVersion so the deployed
	// reference is pinned (repo/name:tag@sha256:...), not the mutable tag alone.
	if sv.Digest != executeDigest {
		t.Errorf("expected digest %q carried to SoftwareVersion, got %q", executeDigest, sv.Digest)
	}
	if got, want := common.BuildImage(sv), "reg-a/cn:0.75.0@"+string(executeDigest); got != want {
		t.Errorf("expected pinned image ref %q, got %q", want, got)
	}
}

func sourcedFor(repo string) *operatorv1alpha1.SoftwareVersionSource {
	return &operatorv1alpha1.SoftwareVersionSource{
		ImageRepositories: []operatorv1alpha1.ImageRepository{
			{Repository: repo, ImageName: "cn", ImageTag: "0.74.0", ImagePullSecrets: []corev1.LocalObjectReference{{Name: "sec-" + repo}}},
			{Repository: "other", ImageName: "cn", ImageTag: "0.74.0", ImagePullSecrets: []corev1.LocalObjectReference{{Name: "sec-other"}}},
		},
		ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{{OS: "linux", Architecture: "amd64", LayerHashes: []string{"aa"}}},
		ImagePullPolicy:       "Always",
	}
}

// deterministicComponent is an execute-spec component for the new version: candidate
// registries plus per-platform layer hashes, the inputs rebuildSource needs.
func deterministicComponent(version string, images ...string) operatorv1alpha1.ImageComponentSpec {
	return operatorv1alpha1.ImageComponentSpec{
		Version:    version,
		Registries: registriesFor(images...),
		Deterministic: operatorv1alpha1.ImageDeterministicSpec{
			Supported:   true,
			LayerHashes: map[string][]operatorv1alpha1.Sha256Digest{"linux/amd64": {"aa"}},
		},
	}
}

func compPtr(c operatorv1alpha1.ImageComponentSpec) *operatorv1alpha1.ImageComponentSpec { return &c }

// A capsule installed with softwareVersionSource must have the source rebuilt for the new
// version (not cleared) and softwareVersion left unset; otherwise the capsule controller
// keeps resolving the old source and the node never upgrades, and clearing would drop
// multi-registry failover.
func TestApplyImagePatch_SourcedConsensusNodeRebuildsSource(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: deterministicComponent("0.74.2", "reg-a/cn:0.74.2"),
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.74.2", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	cn := &capsule.Spec.PodProperties.Containers.ConsensusNode
	cn.SoftwareVersionSource = sourcedFor("reg-a")

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}

	if cn.SoftwareVersion != nil {
		t.Error("expected softwareVersion cleared (source-only container)")
	}
	src := cn.SoftwareVersionSource
	if src == nil {
		t.Fatal("expected softwareVersionSource rebuilt, not cleared")
	}
	if len(src.ImageRepositories) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(src.ImageRepositories))
	}
	repo := src.ImageRepositories[0]
	if repo.Repository != "reg-a" || repo.ImageName != "cn" || repo.ImageTag != "0.74.2" {
		t.Errorf("candidate = %+v, want reg-a/cn:0.74.2", repo)
	}
	if len(repo.ImagePullSecrets) != 1 || repo.ImagePullSecrets[0].Name != "sec-reg-a" {
		t.Errorf("pull secrets = %+v, want the matching repo's", repo.ImagePullSecrets)
	}
	if src.ImagePullPolicy != "Always" {
		t.Errorf("pull policy = %q, want Always (from the install source)", src.ImagePullPolicy)
	}
	if src.Selected == nil || src.Selected.Image != "reg-a/cn:0.74.2" || src.Selected.Digest != executeDigest {
		t.Errorf("selected pin = %+v, want reg-a/cn:0.74.2 @ %s", src.Selected, executeDigest)
	}
}

// When the upgrade introduces a registry that was not a candidate of the old source, the
// rebuilt candidate keeps the union of every old secret (host-keyed, so extras are
// harmless) rather than risk an unauthenticated pull.
func TestApplyImagePatch_SourcedUnknownRegistryUnionSecrets(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: deterministicComponent("0.74.2", "new-reg/cn:0.74.2"),
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "new-reg/cn:0.74.2", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	cn := &capsule.Spec.PodProperties.Containers.ConsensusNode
	cn.SoftwareVersionSource = sourcedFor("reg-a")

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}
	if cn.SoftwareVersion != nil {
		t.Error("expected softwareVersion cleared")
	}
	src := cn.SoftwareVersionSource
	if src == nil || len(src.ImageRepositories) != 1 {
		t.Fatalf("expected rebuilt source with 1 candidate, got %+v", src)
	}
	if len(src.ImageRepositories[0].ImagePullSecrets) != 2 {
		t.Errorf("expected union of old secrets (2), got %+v", src.ImageRepositories[0].ImagePullSecrets)
	}
}

func TestApplyImagePatch_SourcedUploaderRebuildsSource(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			RecordStreamUploader: compPtr(deterministicComponent("1", "reg-a/up:1")),
		},
		operatorv1alpha1.SelectedSources{
			RecordStreamUploader: &operatorv1alpha1.SelectedSource{Image: "reg-a/up:1", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	capsule.Spec.PodProperties.Containers.RecordsUploader = &operatorv1alpha1.CheetahUploader{
		SoftwareVersionSource: sourcedFor("reg-a"),
	}

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}
	u := capsule.Spec.PodProperties.Containers.RecordsUploader
	if u.SoftwareVersion != nil || u.SoftwareVersionSource == nil {
		t.Fatalf("uploader source not rebuilt: %+v", u)
	}
	if u.SoftwareVersionSource.ImageRepositories[0].ImageTag != "1" {
		t.Errorf("uploader candidate tag = %q, want 1", u.SoftwareVersionSource.ImageRepositories[0].ImageTag)
	}
}

func TestApplyImagePatch_UnpinnedSourceErrors(t *testing.T) {
	// A selected source with no digest must abort the execute — the operator must never
	// deploy an unpinned (mutable-tag) image. This is the operator-layer digest check.
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{
				Version:    "0.75.0",
				Registries: registriesFor("reg-a/cn:0.75.0"),
			},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.75.0"}, // no digest
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{ImageName: "cn", ImageTag: "0.74.0"},
					},
				},
			},
		},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err == nil {
		t.Fatal("expected applyImagePatch to error when the selected source has no digest")
	}
}

func TestApplyImagePatch_CheetahSidecar_Upgrade(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			RecordStreamUploader: &operatorv1alpha1.ImageComponentSpec{
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
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			RecordStreamUploader: &operatorv1alpha1.ImageComponentSpec{
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
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			BackupUploader: &operatorv1alpha1.ImageComponentSpec{
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
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
			BackupUploader: &operatorv1alpha1.ImageComponentSpec{
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
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0", Registries: registriesFor("reg-a/cn:0.75.0")},
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

// reconcilePendingGate runs one reconcile of an execute waiting in PendingNodeUpgrade
// against the given capsule and StatefulSet, and returns the updated execute.
func reconcilePendingGate(t *testing.T, capsule *operatorv1alpha1.ConsensusCapsule, sts *appsv1.StatefulSet) operatorv1alpha1.NetworkUpgradeExecute {
	t.Helper()
	execute := newExecuteInBringOnline()

	scheme := newControllerTestScheme()
	c := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(execute, capsule, sts, newTestOrbit()).
		WithStatusSubresource(execute, capsule, sts)).Build()
	for _, o := range []client.Object{execute, capsule, sts} {
		if err := c.Status().Update(context.Background(), o); err != nil {
			t.Fatalf("pre-set status: %v", err)
		}
	}

	r := NewNetworkUpgradeExecuteReconciler(c, scheme)
	r.Recorder = events.NewFakeRecorder(8)
	r.Checker = newExecuteChecker(nil, nil)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var updated operatorv1alpha1.NetworkUpgradeExecute
	_ = c.Get(context.Background(), client.ObjectKeyFromObject(execute), &updated)
	return updated
}

// A patch that changed nothing leaves the pre-upgrade revision "rolled". The gate must
// not report Succeeded while the template still runs the old consensus-node digest.
func TestExecuteReconcile_PendingNodeUpgrade_NotFooledByUnchangedTemplate(t *testing.T) {
	sts := newExecuteSTS("node0", "rev-old", "rev-old", 1, 1)
	sts.Spec.Template.Spec.Containers[0].Image = "reg-a/cn:0.74.0@" + string(altExecuteDigest)

	updated := reconcilePendingGate(t, newActiveCapsule("node0"), sts)
	if updated.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded {
		t.Error("must not succeed while the template runs the pre-upgrade image")
	}
}

// phase==Active alone is not proof the node is ACTIVE: a frozen node reported
// phase Active with platformStatus FREEZE_COMPLETE.
func TestExecuteReconcile_PendingNodeUpgrade_RequiresPlatformActive(t *testing.T) {
	capsule := newActiveCapsule("node0")
	capsule.Status.PlatformStatus = operatorv1alpha1.PlatformStatusFreezeComplete

	updated := reconcilePendingGate(t, capsule, newExecuteSTS("node0", "rev-new", "rev-new", 1, 1))
	if updated.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded {
		t.Error("must not succeed while platformStatus is FREEZE_COMPLETE")
	}
}

func TestTemplateRunsSelectedConsensusNode(t *testing.T) {
	image := func(d operatorv1alpha1.Sha256Digest) string { return "reg-a/cn:0.75.0@" + string(d) }
	tests := []struct {
		name          string
		containerName string // capsule spec override; "" uses the default
		containers    []corev1.Container
		selected      *operatorv1alpha1.SelectedSource
		want          bool
	}{
		{"selected digest", "", []corev1.Container{{Name: "consensus-node", Image: image(executeDigest)}}, &operatorv1alpha1.SelectedSource{Digest: executeDigest}, true},
		{"old digest", "", []corev1.Container{{Name: "consensus-node", Image: image(altExecuteDigest)}}, &operatorv1alpha1.SelectedSource{Digest: executeDigest}, false},
		{"digest on another container", "", []corev1.Container{{Name: "consensus-node", Image: image(altExecuteDigest)}, {Name: "uc", Image: image(executeDigest)}}, &operatorv1alpha1.SelectedSource{Digest: executeDigest}, false},
		{"container missing", "", nil, &operatorv1alpha1.SelectedSource{Digest: executeDigest}, false},
		{"custom container name", "cn", []corev1.Container{{Name: "cn", Image: image(executeDigest)}}, &operatorv1alpha1.SelectedSource{Digest: executeDigest}, true},
		{"consensus node not upgraded", "", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: tt.containers}}}}
			execute := &operatorv1alpha1.NetworkUpgradeExecute{}
			execute.Status.SelectedSources = &operatorv1alpha1.SelectedSources{ConsensusNode: tt.selected}
			capsule := &operatorv1alpha1.ConsensusCapsule{}
			capsule.Spec.PodProperties.Containers.ConsensusNode.Name = tt.containerName

			if got := templateRunsSelectedConsensusNode(sts, execute, capsule); got != tt.want {
				t.Errorf("templateRunsSelectedConsensusNode = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyImagePatch_SourcedUCAndBackupRebuildSource(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			UC:             compPtr(deterministicComponent("2", "reg-a/uc:2")),
			BackupUploader: compPtr(deterministicComponent("3", "reg-a/bk:3")),
		},
		operatorv1alpha1.SelectedSources{
			UC:             &operatorv1alpha1.SelectedSource{Image: "reg-a/uc:2", Digest: executeDigest},
			BackupUploader: &operatorv1alpha1.SelectedSource{Image: "reg-a/bk:3", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	ct := &capsule.Spec.PodProperties.Containers
	ct.UC = &operatorv1alpha1.UcSidecar{SoftwareVersionSource: sourcedFor("reg-a")}
	ct.BackupUploader = &operatorv1alpha1.StreamUploader{SoftwareVersionSource: sourcedFor("reg-a")}

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}
	if ct.UC.SoftwareVersion != nil || ct.UC.SoftwareVersionSource == nil || ct.UC.SoftwareVersionSource.ImageRepositories[0].ImageTag != "2" {
		t.Errorf("uc not rebuilt: %+v", ct.UC)
	}
	if ct.BackupUploader.SoftwareVersion != nil || ct.BackupUploader.SoftwareVersionSource == nil || ct.BackupUploader.SoftwareVersionSource.ImageRepositories[0].ImageTag != "3" {
		t.Errorf("backup uploader not rebuilt: %+v", ct.BackupUploader)
	}
}

// End-to-end: after execute rebuilds a sourced capsule's source for the new version, a
// capsule reconcile resolves the rebuilt source and pins the NEW version — it does not
// revert to the old image (the #1395 regression). A stale resolved-image cache entry for
// the old version is present to prove the rebuild busts the cache and re-resolves.
func TestSourcedUpgrade_ReconcileAfterPatchDoesNotRevert(t *testing.T) {
	// Capsule installed with a source for the old version, already resolved + cached to it.
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test", Generation: 1},
	}
	cn := &capsule.Spec.PodProperties.Containers.ConsensusNode
	cn.SoftwareVersionSource = &operatorv1alpha1.SoftwareVersionSource{
		ImageRepositories: []operatorv1alpha1.ImageRepository{
			{Repository: "ghcr.io/hashgraph", ImageName: "consensus-node", ImageTag: "0.74.0"},
		},
		ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{
			{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:aaa"}},
		},
	}
	oldHash, err := sourceHash(cn.SoftwareVersionSource)
	if err != nil {
		t.Fatalf("sourceHash: %v", err)
	}
	capsule.Status.ResolvedImages = []operatorv1alpha1.ResolvedImageStatus{{
		Container:  "consensusNode",
		SourceHash: oldHash,
		Image:      "ghcr.io/hashgraph/consensus-node:0.74.0@" + string(testResolvedDigest),
		Digest:     testResolvedDigest,
		Repository: "ghcr.io/hashgraph",
		Validity:   operatorv1alpha1.ImageValidityValid,
	}}

	// Execute patches the new version (0.74.2).
	newComp := deterministicComponent("0.74.2", "ghcr.io/hashgraph/consensus-node:0.74.2")
	newComp.Deterministic.LayerHashes = map[string][]operatorv1alpha1.Sha256Digest{"linux/amd64": {"sha256:aaa"}}
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{ConsensusNode: newComp},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "ghcr.io/hashgraph/consensus-node:0.74.2", Digest: executeDigest},
		},
	)
	if err := (&NetworkUpgradeExecuteReconciler{}).applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}
	if cn.SoftwareVersion != nil || cn.SoftwareVersionSource == nil || cn.SoftwareVersionSource.ImageRepositories[0].ImageTag != "0.74.2" {
		t.Fatalf("execute did not rebuild the source to 0.74.2: %+v", cn)
	}
	if cn.SoftwareVersionSource.Selected == nil || cn.SoftwareVersionSource.Selected.Digest != executeDigest {
		t.Fatalf("execute did not pin the selected digest: %+v", cn.SoftwareVersionSource.Selected)
	}

	// Capsule reconcile honors the upgrade-selected digest and does NOT re-probe. The fake
	// checker would return a different digest if called, so it must not be called and the
	// stale cache entry for 0.74.0 must not win.
	checker := &fakeImageChecker{fn: func(spec operatorv1alpha1.ImageRegistrySpec) registry.RegistryResult {
		return registry.RegistryResult{Image: spec.Image, State: registry.StateValid, Digest: altExecuteDigest, LayerHashesVerified: true}
	}}
	// Seed the fake client AFTER the patch so Status().Update reloads the rebuilt spec,
	// not the pre-patch one (client.Status().Update reloads the server's spec).
	r := newResolveReconciler(checker, capsule)
	if _, err := r.resolveImageSources(context.Background(), capsule); err != nil {
		t.Fatalf("resolveImageSources: %v", err)
	}

	if checker.calls != 0 {
		t.Errorf("expected the selected digest honored without probing, got %d checker calls", checker.calls)
	}
	// The node is pinned to exactly the upgrade-selected digest (0.74.2), not reverted to
	// 0.74.0 and not a re-resolved digest.
	sv := cn.SoftwareVersion
	if sv == nil {
		t.Fatal("expected SoftwareVersion pinned after reconcile")
	}
	if sv.ImageTag != "0.74.2" {
		t.Errorf("reverted: image tag = %q, want 0.74.2", sv.ImageTag)
	}
	if sv.Digest != executeDigest {
		t.Errorf("digest = %q, want the upgrade-selected %q", sv.Digest, executeDigest)
	}
	// The multi-registry source is retained for the next upgrade's re-selection.
	if cn.SoftwareVersionSource == nil || cn.SoftwareVersionSource.ImageRepositories[0].ImageTag != "0.74.2" {
		t.Errorf("source not retained after reconcile: %+v", cn.SoftwareVersionSource)
	}
}

// Focused rebuildSource unit tests.

func TestRebuildSource_CarriesStrategyAndPolicyFromOld(t *testing.T) {
	old := sourcedFor("reg-a")
	old.SelectionStrategy = "Sequential"
	old.ImagePullPolicy = "Always"
	comp := deterministicComponent("0.74.2", "reg-a/cn:0.74.2")
	comp.SelectionStrategy = "Random" // the upgrade must not override the install source's knob

	got, err := rebuildSource(old, comp)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectionStrategy != "Sequential" {
		t.Errorf("strategy = %q, want Sequential (from old source)", got.SelectionStrategy)
	}
	if got.ImagePullPolicy != "Always" {
		t.Errorf("pull policy = %q, want Always (from old source)", got.ImagePullPolicy)
	}
}

// A matched candidate that is public (no pull secrets) must stay public — it must not
// inherit the union of other candidates' secrets. Guards against reintroducing the
// nil-means-no-match fallback bug.
func TestRebuildSource_MatchedPublicCandidateGetsNoUnion(t *testing.T) {
	old := &operatorv1alpha1.SoftwareVersionSource{
		ImageRepositories: []operatorv1alpha1.ImageRepository{
			{Repository: "pub", ImageName: "cn", ImageTag: "0.74.0"}, // public: no pull secrets
			{Repository: "priv", ImageName: "cn", ImageTag: "0.74.0", ImagePullSecrets: []corev1.LocalObjectReference{{Name: "sec-priv"}}},
		},
		ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{{OS: "linux", Architecture: "amd64", LayerHashes: []string{"aa"}}},
	}
	got, err := rebuildSource(old, deterministicComponent("0.74.2", "pub/cn:0.74.2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ImageRepositories[0].ImagePullSecrets) != 0 {
		t.Errorf("matched public candidate got secrets %+v, want none", got.ImageRepositories[0].ImagePullSecrets)
	}
}

func TestRebuildSource_StripsDigestFromRef(t *testing.T) {
	got, err := rebuildSource(sourcedFor("reg-a"), deterministicComponent("0.74.2", "reg-a/cn:0.74.2@"+string(executeDigest)))
	if err != nil {
		t.Fatal(err)
	}
	repo := got.ImageRepositories[0]
	if repo.Repository != "reg-a" || repo.ImageName != "cn" || repo.ImageTag != "0.74.2" {
		t.Errorf("candidate = %+v, want reg-a/cn:0.74.2 (digest stripped)", repo)
	}
}

func TestRebuildSource_FailsClosedOnTaglessRef(t *testing.T) {
	if _, err := rebuildSource(sourcedFor("reg-a"), deterministicComponent("0.74.2", "reg-a/cn")); err == nil {
		t.Fatal("expected error for a tagless registry ref")
	}
}

func TestRebuildSource_FailsClosedOnEmptyHashes(t *testing.T) {
	comp := operatorv1alpha1.ImageComponentSpec{Version: "0.74.2", Registries: registriesFor("reg-a/cn:0.74.2")}
	if _, err := rebuildSource(sourcedFor("reg-a"), comp); err == nil {
		t.Fatal("expected error when the upgrade component has no layer hashes")
	}
}

func TestRebuildSource_PlatformsSortedLayerOrderPreserved(t *testing.T) {
	comp := deterministicComponent("0.74.2", "reg-a/cn:0.74.2")
	comp.Deterministic.LayerHashes = map[string][]operatorv1alpha1.Sha256Digest{
		"linux/arm64": {"c1", "c2"},
		"linux/amd64": {"a1", "a2"},
	}
	got, err := rebuildSource(sourcedFor("reg-a"), comp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ImageVerificationSpec) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(got.ImageVerificationSpec))
	}
	if got.ImageVerificationSpec[0].Architecture != "amd64" || got.ImageVerificationSpec[1].Architecture != "arm64" {
		t.Errorf("platform order = [%s, %s], want [amd64, arm64] (sorted)",
			got.ImageVerificationSpec[0].Architecture, got.ImageVerificationSpec[1].Architecture)
	}
	if !reflect.DeepEqual(got.ImageVerificationSpec[0].LayerHashes, []string{"a1", "a2"}) {
		t.Errorf("amd64 layers = %+v, want [a1 a2] (order preserved)", got.ImageVerificationSpec[0].LayerHashes)
	}
}

// Rebuilding then converting back through ToImageComponentSpec round-trips the registries
// and layer hashes, confirming rebuildSource is the inverse of that conversion.
func TestRebuildSource_RoundTripsThroughToImageComponentSpec(t *testing.T) {
	comp := deterministicComponent("0.74.2", "reg-a/cn:0.74.2")
	comp.Deterministic.LayerHashes = map[string][]operatorv1alpha1.Sha256Digest{
		"linux/amd64": {"a1", "a2"},
		"linux/arm64": {"b1"},
	}
	got, err := rebuildSource(sourcedFor("reg-a"), comp)
	if err != nil {
		t.Fatal(err)
	}
	round := got.ToImageComponentSpec()
	if !reflect.DeepEqual(round.Deterministic.LayerHashes, comp.Deterministic.LayerHashes) {
		t.Errorf("round-trip layer hashes = %+v, want %+v", round.Deterministic.LayerHashes, comp.Deterministic.LayerHashes)
	}
	if len(round.Registries) != 1 || round.Registries[0].Image != "reg-a/cn:0.74.2" {
		t.Errorf("round-trip registries = %+v, want reg-a/cn:0.74.2", round.Registries)
	}
}

// A container installed with a single softwareVersion keeps its pull policy and
// secrets: only the image and digest change.
func TestApplyImagePatch_UnsourcedKeepsPullSettings(t *testing.T) {
	execute := newExecuteWithImages(
		operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.74.2", Registries: registriesFor("reg-a/cn:0.74.2")},
		},
		operatorv1alpha1.SelectedSources{
			ConsensusNode: &operatorv1alpha1.SelectedSource{Image: "reg-a/cn:0.74.2", Digest: executeDigest},
		},
	)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	cn := &capsule.Spec.PodProperties.Containers.ConsensusNode
	cn.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
		Repository: "reg-a", ImageName: "cn", ImageTag: "0.74.0",
		ImagePullPolicy:  "IfNotPresent",
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "keep-me"}},
	}

	r := &NetworkUpgradeExecuteReconciler{}
	if err := r.applyImagePatch(execute, capsule); err != nil {
		t.Fatal(err)
	}
	sv := cn.SoftwareVersion
	if sv.ImageTag != "0.74.2" || sv.Digest != executeDigest {
		t.Errorf("image not upgraded: %+v", sv)
	}
	if sv.ImagePullPolicy != "IfNotPresent" || len(sv.ImagePullSecrets) != 1 || sv.ImagePullSecrets[0].Name != "keep-me" {
		t.Errorf("pull settings changed: policy=%q secrets=%+v", sv.ImagePullPolicy, sv.ImagePullSecrets)
	}
}
