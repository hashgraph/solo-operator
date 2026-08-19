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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "test",
			Labels:    map[string]string{operatorv1alpha1.LabelOperationID: "op-001", operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
		},
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
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: prepare.Spec.Orbit},
	}
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare, orbit).
		WithStatusSubresource(prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.ConsensusCapsule{}, fieldCapsuleOrbit, capsuleOrbitIndexFunc).
		Build()

	rec := events.NewFakeRecorder(64)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = checker

	// Run until terminal (handles the init→requeue→work cycle).
	var err error
	for i := 0; i < 5; i++ {
		var res ctrl.Result
		res, err = r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(prepare),
		})
		if !res.Requeue && res.RequeueAfter == 0 {
			break
		}
	}

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

	// enabled=false → SidecarDisabled event, but no registry audit.
	if got := countContaining(res.events, "SidecarDisabled recordStreamUploader"); got != 1 {
		t.Errorf("expected one SidecarDisabled event for disabled RSU, got %d in %v", got, res.events)
	}
	if got := countContaining(res.events, "RegistryAuditComplete recordStreamUploader"); got != 0 {
		t.Errorf("disabled component should not trigger registry audit, got %d in %v", got, res.events)
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
		if cond.Reason != operatorv1alpha1.ReasonSelectionComplete.String() {
			t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonSelectionComplete.String(), cond.Reason)
		}
	})
}

func TestPrepareReconcile_AllInvalidRetries(t *testing.T) {
	checker := newPrepareChecker(map[string]bool{
		"gcr.io/a/cn:0.62.0":  true,
		"ghcr.io/b/cn:0.62.0": true,
		"reg.c.io/cn:0.62.0":  true,
	}, nil)
	res := reconcilePrepare(t, newPrepareForTest(), checker)

	if res.err == nil {
		t.Error("expected error when no valid registry")
	}
	// Registry failures are transient — phase stays Running for retry.
	if res.prepare.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected phase Running (retryable), got %s", res.prepare.Status.Phase)
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

func TestPrepareReconcile_SetsStartTime(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	if res.prepare.Status.StartTime == nil {
		t.Error("expected StartTime to be set")
	}
}

func TestPrepareReconcile_SetsManifestValidatedCondition(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondManifestValidated)
	if cond == nil {
		t.Fatal("ManifestValidated condition not set")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected ManifestValidated=True, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.ReasonSchemaValid.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonSchemaValid.String(), cond.Reason)
	}
}

func TestPrepareReconcile_SetsImageValidationConditionTrue(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondImageValidation)
	if cond == nil {
		t.Fatal("ImageValidation condition not set")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected ImageValidation=True, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.ReasonAllImagesValid.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonAllImagesValid.String(), cond.Reason)
	}
}

func TestPrepareReconcile_SetsImageValidationConditionFalse_OnHashMismatch(t *testing.T) {
	checker := newPrepareChecker(nil, map[string]bool{"ghcr.io/b/cn:0.62.0": true})
	res := reconcilePrepare(t, newPrepareForTest(), checker)

	// Hash mismatch: a winner is still selected (other registries valid), but allHashesValid=false.
	if res.prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected phase Succeeded even with hash mismatch (winner selected), got %s", res.prepare.Status.Phase)
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondImageValidation)
	if cond == nil {
		t.Fatal("ImageValidation condition not set")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected ImageValidation=False on hash mismatch, got %s", cond.Status)
	}
}

func TestPrepareReconcile_PhaseFailedOnAllInvalid_IsNoOp(t *testing.T) {
	prepare := newPrepareForTest()
	prepare.Status.Phase = operatorv1alpha1.PhaseFailed

	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare).
		WithStatusSubresource(prepare).
		Build()
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
		t.Errorf("already-failed CR should emit no events, got %v", recorded)
	}
}

func TestPrepareReconcile_EmitsComponentSkipped(t *testing.T) {
	prepare := newPrepareForTest()
	// Remove the optional recordStreamUploader entirely to trigger ComponentSkipped.
	prepare.Spec.Images.RecordStreamUploader = nil
	res := reconcilePrepare(t, prepare, newPrepareChecker(nil, nil))

	if got := countContaining(res.events, "ComponentSkipped recordStreamUploader"); got != 1 {
		t.Errorf("expected one ComponentSkipped event for recordStreamUploader, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_EmitsSidecarDisabled(t *testing.T) {
	res := reconcilePrepare(t, newPrepareForTest(), newPrepareChecker(nil, nil))

	// newPrepareForTest has RSU present with enabled=false → SidecarDisabled.
	if got := countContaining(res.events, "SidecarDisabled recordStreamUploader"); got != 1 {
		t.Errorf("expected one SidecarDisabled event for recordStreamUploader, got %d in %v", got, res.events)
	}
}

func TestPrepareReconcile_OperationIDLabelMissing(t *testing.T) {
	prepare := newPrepareForTest()
	prepare.Labels = nil

	res := reconcilePrepare(t, prepare, newPrepareChecker(nil, nil))

	if res.prepare.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.prepare.Status.Phase)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventOperationIDLabelInvalid.String()); got != 1 {
		t.Errorf("expected one OperationIDLabelInvalid event, got %d in %v", got, res.events)
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondOperationIDLabelValid=False, got %v", cond)
	}
	if cond != nil && cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonOperationIDLabelInvalid.String(), cond.Reason)
	}
	// Invalid identity must not advance to registry auditing.
	if got := countContaining(res.events, operatorv1alpha1.EventRegistryAuditComplete.String()); got != 0 {
		t.Errorf("invalid CR must not audit registries, got %d audit events", got)
	}
}

func TestPrepareReconcile_OperationIDLabelMismatch(t *testing.T) {
	prepare := newPrepareForTest()
	prepare.Labels[operatorv1alpha1.LabelOperationID] = "op-does-not-match"

	res := reconcilePrepare(t, prepare, newPrepareChecker(nil, nil))

	if res.prepare.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.prepare.Status.Phase)
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondOperationIDLabelValid)
	if cond == nil || cond.Reason != operatorv1alpha1.ReasonOperationIDLabelInvalid.String() {
		t.Errorf("expected reason %s, got %v", operatorv1alpha1.ReasonOperationIDLabelInvalid.String(), cond)
	}
}

func TestPrepareReconcile_StepLabelInvalid(t *testing.T) {
	prepare := newPrepareForTest()
	// Wrong step value for a Prepare CR — must be rejected terminally (#1133).
	prepare.Labels[operatorv1alpha1.LabelStep] = operatorv1alpha1.StepFreeze

	res := reconcilePrepare(t, prepare, newPrepareChecker(nil, nil))

	if res.prepare.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", res.prepare.Status.Phase)
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondStepLabelValid)
	if cond == nil || cond.Reason != operatorv1alpha1.ReasonStepLabelInvalid.String() {
		t.Errorf("expected reason %s, got %v", operatorv1alpha1.ReasonStepLabelInvalid.String(), cond)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventStepLabelInvalid.String()); got != 1 {
		t.Errorf("expected one StepLabelInvalid event, got %d", got)
	}
	// Invalid identity must not advance to registry auditing.
	if got := countContaining(res.events, operatorv1alpha1.EventRegistryAuditComplete.String()); got != 0 {
		t.Errorf("invalid CR must not audit registries, got %d audit events", got)
	}
}

func TestPrepareReconcile_DuplicateOperationID_SetsPhaseFailed(t *testing.T) {
	scheme := newControllerTestScheme()

	existing := newPrepareForTest()
	existing.Name = "existing"
	existing.UID = "uid-existing"

	prepare := newPrepareForTest()
	prepare.Name = "duplicate"
	prepare.UID = "uid-duplicate"

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existing, prepare).
		WithStatusSubresource(existing, prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		Build()

	rec := events.NewFakeRecorder(16)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newPrepareChecker(nil, nil)

	// First reconcile stamps the finalizer and requeues before any phase work.
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "duplicate", Namespace: "test"},
	}); err != nil {
		t.Fatalf("finalizer reconcile: %v", err)
	}

	// Advance "duplicate" to Running phase so uniqueness check runs.
	var dup operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKey{Name: "duplicate", Namespace: "test"}, &dup); err != nil {
		t.Fatalf("get duplicate: %v", err)
	}
	dup.Status.Phase = operatorv1alpha1.PhaseRunning
	if err := c.Status().Update(context.Background(), &dup); err != nil {
		t.Fatalf("pre-set phase: %v", err)
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "duplicate", Namespace: "test"},
	})
	if err == nil {
		t.Error("expected error for duplicate operationId")
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if getErr := c.Get(context.Background(), client.ObjectKey{Name: "duplicate", Namespace: "test"}, &got); getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected PhaseFailed for duplicate operationId, got %s", got.Status.Phase)
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

func TestPrepareReconcile_UniqueOperationID_IgnoresSibling(t *testing.T) {
	scheme := newControllerTestScheme()

	sibling := newPrepareForTest()
	sibling.Name = "other"
	sibling.UID = "uid-other"
	sibling.Spec.OperationID = "op-different"

	prepare := newPrepareForTest()
	prepare.Name = "mine"
	prepare.UID = "uid-mine"
	// prepare.Spec.OperationID == "op-001" (from newPrepareForTest)

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: prepare.Spec.Orbit},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sibling, prepare, orbit).
		WithStatusSubresource(sibling, prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.ConsensusCapsule{}, fieldCapsuleOrbit, capsuleOrbitIndexFunc).
		Build()

	rec := events.NewFakeRecorder(64)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec
	r.Checker = newPrepareChecker(nil, nil)

	// Run until terminal.
	for i := 0; i < 5; i++ {
		res, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKey{Name: "mine", Namespace: "test"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Requeue && res.RequeueAfter == 0 {
			break
		}
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKey{Name: "mine", Namespace: "test"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		t.Errorf("expected PhaseSucceeded when sibling has different operationId, got %s", got.Status.Phase)
	}
}

func TestCollectUniqueImages(t *testing.T) {
	sources := &operatorv1alpha1.SelectedSources{}
	sources.Set(operatorv1alpha1.ComponentConsensusNode, &operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1"})
	sources.Set(operatorv1alpha1.ComponentRecordStreamUploader, &operatorv1alpha1.SelectedSource{Image: "gcr.io/rsu:v1"})
	sources.Set(operatorv1alpha1.ComponentEventStreamUploader, &operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1"})

	images := collectUniqueImages(sources)
	if len(images) != 2 {
		t.Fatalf("expected 2 unique images, got %d: %v", len(images), images)
	}

	if images := collectUniqueImages(nil); len(images) != 0 {
		t.Errorf("expected 0 images for nil sources, got %d", len(images))
	}
}

func TestReconcilePrefetch_NoCapsules_SkipsPrefetch(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	prepare.Status.Phase = operatorv1alpha1.PhaseSucceeded
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: prepare.Spec.Orbit},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare, orbit).
		WithStatusSubresource(prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.ConsensusCapsule{}, fieldCapsuleOrbit, capsuleOrbitIndexFunc).
		Build()

	rec := events.NewFakeRecorder(64)
	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = rec

	for i := 0; i < 5; i++ {
		res, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(prepare),
		})
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if !res.Requeue && res.RequeueAfter == 0 {
			break
		}
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(prepare), &got); err != nil {
		t.Fatalf("get: %v", err)
	}

	completedCond := findCondition(got.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
	if completedCond == nil {
		t.Fatal("expected CondImagePrefetchCompleted to be set")
	}
	if completedCond.Status != metav1.ConditionTrue {
		t.Errorf("expected CondImagePrefetchCompleted=True, got %s", completedCond.Status)
	}
	if completedCond.Reason != operatorv1alpha1.ReasonPrefetchSkipped.String() {
		t.Errorf("expected reason PrefetchSkipped, got %s", completedCond.Reason)
	}
}
