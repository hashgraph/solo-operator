// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/crashtest"
	"github.com/hashgraph/solo-operator/internal/operations/genesis"
	"github.com/hashgraph/solo-operator/internal/resources/applicationproperties"
	"github.com/hashgraph/solo-operator/internal/resources/log4j2config"
	"github.com/hashgraph/solo-operator/internal/resources/nodesettings"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

func newRestartExecuteReconciler(c client.Client, checker fakeUpgradeChecker) *NetworkUpgradeExecuteReconciler {
	r := NewNetworkUpgradeExecuteReconciler(c, newControllerTestScheme())
	r.Recorder = events.NewFakeRecorder(64)
	r.Checker = checker
	return r
}

func TestRestart_Execute_CrashAfterCapsulePatchKeepsSelection(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.NodeUpgradeStartTime = &metav1.Time{Time: time.Now()}
	execute.Spec.Images.ConsensusNode.Registries = registriesFor("reg-a/cn:0.75.0", "reg-b/cn:0.75.0")

	capsule := newOfflineCapsule("node0")
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{Repository: "old-repo", ImageName: "cn", ImageTag: "0.74.0"}

	base := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(newControllerTestScheme()).
		WithObjects(execute, capsule, newTestOrbit()).
		WithStatusSubresource(execute)).Build()
	ctx := context.Background()
	if err := base.Status().Update(ctx, execute); err != nil {
		t.Fatalf("seed status: %v", err)
	}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)}

	var dead atomic.Bool
	dying := crashtest.Wrap(base, &dead, func(obj client.Object) bool {
		_, isCapsule := obj.(*operatorv1alpha1.ConsensusCapsule)
		return isCapsule
	})
	first := newRestartExecuteReconciler(dying, newExecuteChecker(map[string]bool{"reg-a/cn:0.75.0": true}, nil))
	for i := 0; i < 5 && !dead.Load(); i++ {
		_, _ = first.Reconcile(ctx, req)
	}
	if !dead.Load() {
		t.Fatal("first process never reached the capsule patch")
	}

	restarted := newRestartExecuteReconciler(base, newExecuteChecker(map[string]bool{"reg-b/cn:0.75.0": true}, nil))
	for i := 0; i < 5; i++ {
		if _, err := restarted.Reconcile(ctx, req); err != nil {
			t.Fatalf("reconcile after restart: %v", err)
		}
	}

	var got operatorv1alpha1.ConsensusCapsule
	if err := base.Get(ctx, client.ObjectKeyFromObject(capsule), &got); err != nil {
		t.Fatalf("get capsule: %v", err)
	}
	if repo := got.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.Repository; repo != "reg-b" {
		t.Errorf("capsule repository = %q after restart, want the pre-crash selection reg-b", repo)
	}
	var ex operatorv1alpha1.NetworkUpgradeExecute
	if err := base.Get(ctx, req.NamespacedName, &ex); err != nil {
		t.Fatalf("get execute: %v", err)
	}
	if ex.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		t.Fatal("execute failed after restart; registry selection was re-run")
	}
	if !apimeta.IsStatusConditionTrue(ex.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched) {
		t.Errorf("expected ConsensusCapsulePatched=True after restart, got %v", ex.Status.Conditions)
	}
}

func TestRestart_Execute_InitKeepsStartTime(t *testing.T) {
	execute := newExecuteForTest()
	execute.Status.Phase = ""
	original := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
	execute.Status.StartTime = &original

	pre := prereqs()
	objs := append([]client.Object{execute, newOfflineCapsule("node0"), newTestOrbit()}, pre...)
	base := withExecuteIndexes(fake.NewClientBuilder().
		WithScheme(newControllerTestScheme()).
		WithObjects(objs...).
		WithStatusSubresource(execute, &operatorv1alpha1.NetworkUpgradePrepare{}, &operatorv1alpha1.NetworkUpgradeFreeze{})).Build()
	ctx := context.Background()
	for _, o := range append([]client.Object{execute}, pre...) {
		if err := base.Status().Update(ctx, o); err != nil {
			t.Fatalf("seed status: %v", err)
		}
	}

	r := newRestartExecuteReconciler(base, newExecuteChecker(nil, nil))
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(execute)}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var ex operatorv1alpha1.NetworkUpgradeExecute
	if err := base.Get(ctx, client.ObjectKeyFromObject(execute), &ex); err != nil {
		t.Fatalf("get execute: %v", err)
	}
	if ex.Status.Phase != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		t.Fatalf("phase = %q, want ReadyForProvisionerDaemon", ex.Status.Phase)
	}
	if ex.Status.StartTime == nil || !ex.Status.StartTime.Equal(&original) {
		t.Errorf("startTime = %v, want preserved %v", ex.Status.StartTime, original)
	}
}

func TestRestart_Genesis_CrashMidConfigMapsConverges(t *testing.T) {
	scheme := ngTestScheme(t)
	ng := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "genesis-1", Namespace: "test", Finalizers: []string{NetworkGenesisFinalizer}},
		Spec:       operatorv1alpha1.NetworkGenesisSpec{Orbit: "test-orbit", OperationID: "genesis-2026-05-20"},
	}
	objs := []client.Object{newTestOrbit(), ngCapsule("node0", 0), ng}
	objs = append(objs, ngCapsuleSecrets("node0", ngSelfSignedCertPEM(t), ngSelfSignedCertPEM(t))...)
	base := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&operatorv1alpha1.NetworkGenesis{}).
		WithObjects(objs...).Build()
	ctx := context.Background()
	name := types.NamespacedName{Namespace: "test", Name: "genesis-1"}

	cmWrites := 0
	var dead atomic.Bool
	dying := crashtest.Wrap(base, &dead, func(obj client.Object) bool {
		if _, ok := obj.(*corev1.ConfigMap); ok {
			cmWrites++
		}
		return cmWrites == 2
	})
	if _, err := NewNetworkGenesisReconciler(dying, scheme).Reconcile(ctx, ctrl.Request{NamespacedName: name}); err == nil {
		t.Fatal("expected the crashed reconcile to error")
	}

	var got operatorv1alpha1.NetworkGenesis
	if err := base.Get(ctx, name, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Status.Phase = operatorv1alpha1.PhaseFailed
	setNetworkGenesisCondition(&got.Status.Conditions, metav1.Condition{
		Type: networkGenesisConditionFailed, Status: metav1.ConditionTrue, Reason: ReasonGenesisFailed.String(), LastTransitionTime: metav1.Now(),
	})
	if err := base.Status().Update(ctx, &got); err != nil {
		t.Fatalf("seed failed status: %v", err)
	}

	reconcileToCompletion(t, NewNetworkGenesisReconciler(base, scheme), name)

	if err := base.Get(ctx, name, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Fatalf("phase = %s, want Succeeded", got.Status.Phase)
	}
	if apimeta.FindStatusCondition(got.Status.Conditions, networkGenesisConditionFailed) != nil {
		t.Error("stale Failed condition survived a successful run")
	}
	for _, cm := range []string{genesis.GenesisConfigMapName, genesis.AddressBookBinConfigMapName, genesis.PublicKeysConfigMapName, genesis.ClusterConfigMapName} {
		if err := base.Get(ctx, types.NamespacedName{Namespace: "test", Name: cm}, &corev1.ConfigMap{}); err != nil {
			t.Errorf("ConfigMap %s missing after restart: %v", cm, err)
		}
	}
}

func TestRestart_Prepare_CrashMidPrefetchJobCreation(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	prepare.Finalizers = []string{FinalizerNetworkUpgradePrepare}
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})
	prepare.Status.Conditions = []operatorv1alpha1.Condition{{
		Type: operatorv1alpha1.CondRegistrySelected, Status: metav1.ConditionTrue, Reason: operatorv1alpha1.ReasonSelectionComplete.String(),
	}}
	cap0 := capsuleForPrefetch("cap-0", "test", "test-orbit", 0, nil, "sa-cn")
	cap1 := capsuleForPrefetch("cap-1", "test", "test-orbit", 1, nil, "sa-cn")

	base := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(prepare, newTestOrbit(), &cap0, &cap1, consensusPodOn(&cap0, "node-a"), consensusPodOn(&cap1, "node-b")).
		WithStatusSubresource(prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.ConsensusCapsule{}, fieldCapsuleOrbit, capsuleOrbitIndexFunc).
		Build()
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(prepare)}

	var dead atomic.Bool
	dying := crashtest.Wrap(base, &dead, func(obj client.Object) bool {
		_, isJob := obj.(*batchv1.Job)
		return isJob
	})
	first := NewNetworkUpgradePrepareReconciler(dying, scheme)
	first.Recorder = events.NewFakeRecorder(64)
	if _, err := first.Reconcile(ctx, req); err == nil || !dead.Load() {
		t.Fatalf("expected crash after the first Job create, err=%v", err)
	}

	restarted := NewNetworkUpgradePrepareReconciler(base, scheme)
	restarted.Recorder = events.NewFakeRecorder(64)
	if _, err := restarted.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile after restart: %v", err)
	}

	var jobs batchv1.JobList
	if err := base.List(ctx, &jobs, client.InNamespace("test")); err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 2 {
		t.Errorf("got %d prefetch Jobs after restart, want 2 (one per node, no duplicates)", len(jobs.Items))
	}
	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := base.Get(ctx, req.NamespacedName, &got); err != nil {
		t.Fatalf("get prepare: %v", err)
	}
	if !apimeta.IsStatusConditionTrue(got.Status.Conditions, operatorv1alpha1.CondImagePrefetchCreated) {
		t.Errorf("expected ImagePrefetchCreated=True after restart, got %v", got.Status.Conditions)
	}
}

type svProbe struct {
	subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
	seen *operatorv1alpha1.SoftwareVersion
}

func (p *svProbe) Reconcile(_ context.Context, c *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	p.seen = c.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.DeepCopy()
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func TestRestart_ConsensusCapsule_PinsReachChildBuild(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	capsule.Finalizers = []string{ConsensusCapsuleFinalizer}
	capsule.Spec.Orbit = "test-orbit"
	capsule.Spec.Log4j2ConfigRef = "log4j2"
	capsule.Spec.SettingsConfigRef = "settings"
	capsule.Spec.ApplicationPropertiesRef = "app"
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
		Enabled:         true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "ghcr.io/hashgraph", ImageName: "uc", ImageTag: "1.0.0"},
	}
	cm := func(name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"}}
	}

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(capsule, newTestOrbit(),
			cm(log4j2config.ConfigMapNameForRef("log4j2")),
			cm(nodesettings.ConfigMapNameForRef("settings")),
			cm(applicationproperties.ConfigMapNameForRef("app"))).
		WithStatusSubresource(capsule).Build()

	probe := &svProbe{BaseSubReconciler: *subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "probe")}
	r := &ConsensusCapsuleReconciler{Client: c, Scheme: scheme, Checker: &fakeImageChecker{},
		subreconciler: subreconciler.NewChain[*operatorv1alpha1.ConsensusCapsule](probe)}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(capsule)}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if probe.seen == nil || probe.seen.Digest != testResolvedDigest {
		t.Fatalf("child build saw SoftwareVersion %+v, want digest %s", probe.seen, testResolvedDigest)
	}
}

func TestOrbit_RepeatReconcileDoesNotRewriteStatus(t *testing.T) {
	orbit := newTestOrbit()
	orbit.Finalizers = []string{OrbitFinalizer}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).
		WithObjects(orbit).WithStatusSubresource(orbit).Build()
	r := NewOrbitReconciler(c, newControllerTestScheme())
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(orbit)}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	var first operatorv1alpha1.Orbit
	if err := c.Get(ctx, req.NamespacedName, &first); err != nil {
		t.Fatalf("get: %v", err)
	}
	if apimeta.FindStatusCondition(first.Status.Conditions, "Ready") == nil {
		t.Fatal("expected Ready condition after first reconcile")
	}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	var second operatorv1alpha1.Orbit
	if err := c.Get(ctx, req.NamespacedName, &second); err != nil {
		t.Fatalf("get: %v", err)
	}
	if second.ResourceVersion != first.ResourceVersion {
		t.Errorf("unchanged Orbit status was rewritten: resourceVersion %s -> %s", first.ResourceVersion, second.ResourceVersion)
	}
}
