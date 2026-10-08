// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/resources/prefetch"
)

func boolPtr(b bool) *bool { return &b }

func registriesFor(images ...string) []operatorv1alpha1.ImageRegistrySpec {
	out := make([]operatorv1alpha1.ImageRegistrySpec, len(images))
	for i, img := range images {
		out[i] = operatorv1alpha1.ImageRegistrySpec{Image: img}
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
				ConsensusNode: operatorv1alpha1.ImageComponentSpec{
					Version:    "0.62.0",
					Registries: registriesFor("gcr.io/a/cn:0.62.0", "ghcr.io/b/cn:0.62.0", "reg.c.io/cn:0.62.0"),
				},
				// present but disabled — must be skipped entirely
				RecordStreamUploader: &operatorv1alpha1.ImageComponentSpec{
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

func TestPrepareReconcile_NoEnabledComponents_Fails(t *testing.T) {
	prepare := newPrepareForTest()
	prepare.Spec.Images.ConsensusNode = operatorv1alpha1.ImageComponentSpec{
		Version:    "0.62.0",
		Enabled:    boolPtr(false),
		Registries: registriesFor("gcr.io/a/cn:0.62.0"),
	}

	res := reconcilePrepare(t, prepare, newPrepareChecker(nil, nil))

	if res.prepare.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed when no components enabled, got %s", res.prepare.Status.Phase)
	}
	cond := findCondition(res.prepare.Status.Conditions, operatorv1alpha1.CondManifestValidated)
	if cond == nil {
		t.Fatal("ManifestValidated condition not set")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected ManifestValidated=False, got %s", cond.Status)
	}
	if cond.Reason != operatorv1alpha1.ReasonSchemaInvalid.String() {
		t.Errorf("expected reason %s, got %s", operatorv1alpha1.ReasonSchemaInvalid.String(), cond.Reason)
	}
	if got := countContaining(res.events, operatorv1alpha1.EventManifestInvalid.String()); got != 1 {
		t.Errorf("expected one ManifestInvalid event, got %d in %v", got, res.events)
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

func TestCollectUniqueImages_PinsDigest(t *testing.T) {
	sources := &operatorv1alpha1.SelectedSources{}
	// Two components share a tag but resolved to different digests: they must be
	// prefetched as distinct pinned references, not collapsed by tag.
	sources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:aaa"})
	sources.Set(operatorv1alpha1.ComponentRecordStreamUploader,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:bbb"})

	images := collectUniqueImages(sources)
	want := map[string]bool{"gcr.io/cn:v1@sha256:aaa": true, "gcr.io/cn:v1@sha256:bbb": true}
	if len(images) != len(want) {
		t.Fatalf("expected %d pinned images, got %d: %v", len(want), len(images), images)
	}
	for _, img := range images {
		if !want[img] {
			t.Errorf("unexpected image ref %q; want one of %v", img, want)
		}
	}
}

func TestAllSourcesPinned(t *testing.T) {
	pinned := &operatorv1alpha1.SelectedSources{}
	pinned.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:aaa"})
	pinned.Set(operatorv1alpha1.ComponentUC,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/uc:v1", Digest: "sha256:bbb"})
	if !allSourcesPinned(pinned) {
		t.Error("expected allSourcesPinned=true when every source carries a digest")
	}

	missing := &operatorv1alpha1.SelectedSources{}
	missing.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:aaa"})
	missing.Set(operatorv1alpha1.ComponentUC,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/uc:v1"}) // no digest
	if allSourcesPinned(missing) {
		t.Error("expected allSourcesPinned=false when a source lacks a digest (must fall through to revalidation)")
	}

	if allSourcesPinned(nil) {
		t.Error("expected allSourcesPinned=false for nil sources")
	}
	if allSourcesPinned(&operatorv1alpha1.SelectedSources{}) {
		t.Error("expected allSourcesPinned=false for an empty source set")
	}
}

func TestReconcilePrefetch_NoCapsules_SkipsPrefetch(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})
	prepare.Status.Conditions = []operatorv1alpha1.Condition{{
		Type:   operatorv1alpha1.CondRegistrySelected,
		Status: metav1.ConditionTrue,
		Reason: operatorv1alpha1.ReasonSelectionComplete.String(),
	}}

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
	if completedCond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondImagePrefetchCompleted=False (skipped), got %s", completedCond.Status)
	}
	if completedCond.Reason != operatorv1alpha1.ReasonPrefetchSkipped.String() {
		t.Errorf("expected reason PrefetchSkipped, got %s", completedCond.Reason)
	}
	// A skipped prefetch must not read as succeeded to Freeze/Execute.
	if _, ok := prefetchCompleted(&got); ok {
		t.Error("prefetchCompleted should report not-succeeded when prefetch was skipped")
	}
}

// TestCheckPrefetchJobs_EmptyListInconclusive verifies that when the created
// condition is set (Jobs were created) but no Jobs remain after the cache-lag
// grace window, checkPrefetchJobs does NOT declare success: an empty list is
// ambiguous (a failed Job may have been TTL-cleaned), so it must mark the
// outcome inconclusive (Completed=False) to force downstream registry
// validation rather than skipping it on an unverifiable assumption.
func TestCheckPrefetchJobs_EmptyListInconclusive(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	// Pre-set the finalizer so a single Reconcile proceeds past ensureFinalizer
	// straight into the prefetch check.
	prepare.Finalizers = []string{FinalizerNetworkUpgradePrepare}
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})
	// created=True set well beyond the 30s cache-lag grace window, with no Jobs
	// present in the cluster.
	prepare.Status.Conditions = []operatorv1alpha1.Condition{
		{
			Type:   operatorv1alpha1.CondRegistrySelected,
			Status: metav1.ConditionTrue,
			Reason: operatorv1alpha1.ReasonSelectionComplete.String(),
		},
		{
			Type:               operatorv1alpha1.CondImagePrefetchCreated,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonPrefetchCreated.String(),
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute)),
		},
	}

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

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(prepare), &got); err != nil {
		t.Fatalf("get: %v", err)
	}

	completedCond := findCondition(got.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
	if completedCond == nil {
		t.Fatal("expected CondImagePrefetchCompleted to be set")
	}
	if completedCond.Status != metav1.ConditionFalse {
		t.Errorf("expected CondImagePrefetchCompleted=False, got %s", completedCond.Status)
	}
	if completedCond.Reason != operatorv1alpha1.ReasonPrefetchInconclusive.String() {
		t.Errorf("expected reason PrefetchInconclusive, got %s", completedCond.Reason)
	}

	// prefetchCompleted must report not-succeeded so Freeze/Execute fall back to
	// registry validation.
	if _, ok := prefetchCompleted(&got); ok {
		t.Error("prefetchCompleted should report not-succeeded for an inconclusive outcome")
	}
}

func capsuleForPrefetch(name, ns, orbit string, nodeID int64, nodeSelector map[string]string, sa string) operatorv1alpha1.ConsensusCapsule {
	c := operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	}
	c.Spec.Orbit = orbit
	c.Spec.NodeId = nodeID
	c.Spec.PodProperties.NodeSelector = nodeSelector
	if sa != "" {
		c.Spec.PodProperties.ServiceAccount = &operatorv1alpha1.ServiceAccountConfig{Name: sa}
	}
	return c
}

func consensusPodOn(capsule *operatorv1alpha1.ConsensusCapsule, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      capsule.Name + "-0",
			Namespace: capsule.Namespace,
			Labels:    consensus.SelectorLabels(capsule),
		},
		Spec: corev1.PodSpec{NodeName: nodeName},
	}
}

func labeledNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

// When the nodeSelector can't bound the pool (here: absent), targeting falls
// back to pinning one Job per node currently hosting a consensus pod, using each
// capsule's own ServiceAccount.
func TestResolvePrefetchTargets_FallbackToPodNodes(t *testing.T) {
	scheme := newControllerTestScheme()
	cap0 := capsuleForPrefetch("cap-0", "test", "test-orbit", 0, nil, "sa-cn")
	cap1 := capsuleForPrefetch("cap-1", "test", "test-orbit", 1, nil, "sa-cn")
	pod0 := consensusPodOn(&cap0, "node-a")
	pod1 := consensusPodOn(&cap1, "node-b")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&cap0, &cap1, pod0, pod1).Build()
	r := NewNetworkUpgradePrepareReconciler(c, scheme)

	targets := r.resolvePrefetchTargets(context.Background(),
		[]operatorv1alpha1.ConsensusCapsule{cap0, cap1}, 30)

	if len(targets) != 2 {
		t.Fatalf("expected 2 pod-node targets, got len=%d", len(targets))
	}
	got := map[string]prefetch.PrefetchTarget{}
	for _, tg := range targets {
		got[tg.id] = tg.target
	}
	for _, n := range []string{"node-a", "node-b"} {
		tgt, ok := got[n]
		if !ok {
			t.Fatalf("expected a target for %s, got %+v", n, got)
		}
		if tgt.NodeName != n {
			t.Errorf("target %s: NodeName=%q, want %q", n, tgt.NodeName, n)
		}
		if tgt.ServiceAccountName != "sa-cn" {
			t.Errorf("target %s: ServiceAccountName=%q, want sa-cn", n, tgt.ServiceAccountName)
		}
	}
}

// With a scoped nodeSelector within the cap, node-pinned coverage includes every
// matching node — including a spare node with no pod on it — so a pod that
// migrates to the spare still hits the cache.
func TestResolvePrefetchTargets_NodePinnedCoversSpareNodes(t *testing.T) {
	scheme := newControllerTestScheme()
	sel := map[string]string{"role": "consensus"}
	cap0 := capsuleForPrefetch("cap-0", "test", "test-orbit", 0, sel, "sa-cn")
	pod0 := consensusPodOn(&cap0, "node-a")

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&cap0, labeledNode("node-a", sel), labeledNode("node-spare", sel), pod0).Build()
	r := NewNetworkUpgradePrepareReconciler(c, scheme)

	targets := r.resolvePrefetchTargets(context.Background(),
		[]operatorv1alpha1.ConsensusCapsule{cap0}, 30)

	if len(targets) != 2 {
		t.Fatalf("expected 2 node-pinned targets (incl. spare), got len=%d", len(targets))
	}
	ids := map[string]bool{}
	for _, tg := range targets {
		ids[tg.id] = true
	}
	if !ids["node-a"] || !ids["node-spare"] {
		t.Errorf("expected node-a and node-spare covered, got %+v", ids)
	}
}

// The pod-node fallback deduplicates nodes shared across capsules and skips pods
// that are not yet scheduled.
func TestResolvePrefetchTargets_FallbackDedupsAndSkipsUnscheduled(t *testing.T) {
	scheme := newControllerTestScheme()
	cap0 := capsuleForPrefetch("cap-0", "test", "test-orbit", 0, nil, "sa-cn")
	cap1 := capsuleForPrefetch("cap-1", "test", "test-orbit", 1, nil, "sa-cn")
	cap2 := capsuleForPrefetch("cap-2", "test", "test-orbit", 2, nil, "sa-cn")
	pod0 := consensusPodOn(&cap0, "node-a")
	pod1 := consensusPodOn(&cap1, "node-a") // same node → dedup
	pod2 := consensusPodOn(&cap2, "")       // unscheduled → skip

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&cap0, &cap1, &cap2, pod0, pod1, pod2).Build()
	r := NewNetworkUpgradePrepareReconciler(c, scheme)

	targets := r.resolvePrefetchTargets(context.Background(),
		[]operatorv1alpha1.ConsensusCapsule{cap0, cap1, cap2}, 30)

	if len(targets) != 1 || targets[0].id != "node-a" {
		t.Fatalf("expected exactly one deduped target node-a, got targets=%+v", targets)
	}
}

// resolvePrefetchTargets sorts capsules by name so the union of tolerations placed on
// each Job pod spec is deterministic regardless of the (unstable) List/input order.
func TestResolvePrefetchTargets_DeterministicTolerationOrder(t *testing.T) {
	scheme := newControllerTestScheme()
	sel := map[string]string{"role": "consensus"}
	capA := capsuleForPrefetch("cap-a", "test", "test-orbit", 0, sel, "sa-cn")
	capB := capsuleForPrefetch("cap-b", "test", "test-orbit", 1, sel, "sa-cn")
	capA.Spec.PodProperties.Tolerations = []corev1.Toleration{{Key: "A", Operator: corev1.TolerationOpExists}}
	capB.Spec.PodProperties.Tolerations = []corev1.Toleration{{Key: "B", Operator: corev1.TolerationOpExists}}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(&capA, &capB, labeledNode("node-1", sel)).Build()
	r := NewNetworkUpgradePrepareReconciler(c, scheme)

	tolKeys := func(caps []operatorv1alpha1.ConsensusCapsule) []string {
		targets := r.resolvePrefetchTargets(context.Background(), caps, 30)
		if len(targets) == 0 {
			t.Fatalf("expected at least one node-pinned target")
		}
		var out []string
		for _, tol := range targets[0].target.Tolerations {
			out = append(out, tol.Key)
		}
		return out
	}

	// Both input orders must yield the sorted-capsule union order [A, B].
	want := []string{"A", "B"}
	for _, in := range [][]operatorv1alpha1.ConsensusCapsule{{capA, capB}, {capB, capA}} {
		got := tolKeys(in)
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("non-deterministic toleration order: got %v, want %v", got, want)
		}
	}
}

// checkPrefetchJobs must derive terminal state from Job conditions: all complete
// → success; any failed → failed (upgrade falls back to registry pull); any
// still running → requeue without deciding.
func TestCheckPrefetchJobs_TerminalDetection(t *testing.T) {
	complete := []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	failed := []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	running := []batchv1.JobCondition(nil)

	tests := []struct {
		name          string
		jobConds      [][]batchv1.JobCondition
		wantCompleted bool
		wantStatus    metav1.ConditionStatus
		wantReason    string
	}{
		{
			name:          "all complete → success",
			jobConds:      [][]batchv1.JobCondition{complete, complete},
			wantCompleted: true,
			wantStatus:    metav1.ConditionTrue,
			wantReason:    operatorv1alpha1.ReasonPrefetchCompleted.String(),
		},
		{
			name:          "one failed → failed",
			jobConds:      [][]batchv1.JobCondition{complete, failed},
			wantCompleted: true,
			wantStatus:    metav1.ConditionFalse,
			wantReason:    operatorv1alpha1.ReasonPrefetchFailed.String(),
		},
		{
			name:          "one still running → requeue, undecided",
			jobConds:      [][]batchv1.JobCondition{complete, running},
			wantCompleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newControllerTestScheme()
			prepare := newPrepareForTest()
			prepare.Status.Phase = operatorv1alpha1.PhaseSucceeded
			// created=True (and completed unset) is the state in which
			// checkPrefetchJobs runs.
			prepare.Status.Conditions = []operatorv1alpha1.Condition{{
				Type:   operatorv1alpha1.CondImagePrefetchCreated,
				Status: metav1.ConditionTrue,
				Reason: operatorv1alpha1.ReasonPrefetchCreated.String(),
			}}

			objs := []client.Object{prepare}
			for i, conds := range tt.jobConds {
				objs = append(objs, &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("prefetch-%d", i),
						Namespace: prepare.Namespace,
						Labels: map[string]string{
							operatorv1alpha1.LabelOperationID:  prepare.Spec.OperationID,
							operatorv1alpha1.LabelPrefetchRole: operatorv1alpha1.RolePrefetch,
						},
					},
					Status: batchv1.JobStatus{Conditions: conds},
				})
			}

			c := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(objs...).WithStatusSubresource(prepare).Build()
			r := NewNetworkUpgradePrepareReconciler(c, scheme)
			r.Recorder = events.NewFakeRecorder(64)

			res, err := r.checkPrefetchJobs(context.Background(), prepare)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			completed := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
			if !tt.wantCompleted {
				if completed != nil {
					t.Errorf("expected no Completed condition while a Job is running, got %+v", completed)
				}
				if res.RequeueAfter == 0 {
					t.Errorf("expected a requeue while a Job is running, got %+v", res)
				}
				return
			}
			if completed == nil {
				t.Fatal("expected CondImagePrefetchCompleted to be set")
			}
			if completed.Status != tt.wantStatus {
				t.Errorf("Completed.Status = %s, want %s", completed.Status, tt.wantStatus)
			}
			if completed.Reason != tt.wantReason {
				t.Errorf("Completed.Reason = %s, want %s", completed.Reason, tt.wantReason)
			}
		})
	}
}

func TestReconcilePrefetch_PendingJobsKeepsPhaseRunning(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	prepare.Finalizers = []string{FinalizerNetworkUpgradePrepare}
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})
	prepare.Status.Conditions = []operatorv1alpha1.Condition{
		{
			Type:   operatorv1alpha1.CondRegistrySelected,
			Status: metav1.ConditionTrue,
			Reason: operatorv1alpha1.ReasonSelectionComplete.String(),
		},
		{
			Type:   operatorv1alpha1.CondImagePrefetchCreated,
			Status: metav1.ConditionTrue,
			Reason: operatorv1alpha1.ReasonPrefetchCreated.String(),
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prefetch-running",
			Namespace: prepare.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID:  prepare.Spec.OperationID,
				operatorv1alpha1.LabelPrefetchRole: operatorv1alpha1.RolePrefetch,
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare, job).
		WithStatusSubresource(prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		Build()

	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = events.NewFakeRecorder(64)

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected requeue while prefetch Job is still running, got %+v", res)
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(prepare), &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != operatorv1alpha1.PhaseRunning {
		t.Fatalf("expected phase Running while prefetch is incomplete, got %s", got.Status.Phase)
	}
}

// When prefetch is disabled via orbit config, createPrefetchJobs must create no
// Jobs and mark the outcome disabled (not succeeded) so Freeze/Execute fall
// through to normal registry validation.
func TestReconcilePrefetch_Disabled(t *testing.T) {
	scheme := newControllerTestScheme()
	prepare := newPrepareForTest()
	prepare.Finalizers = []string{FinalizerNetworkUpgradePrepare}
	prepare.Status.Phase = operatorv1alpha1.PhaseRunning
	prepare.Status.SelectedSources = &operatorv1alpha1.SelectedSources{}
	prepare.Status.SelectedSources.Set(operatorv1alpha1.ComponentConsensusNode,
		&operatorv1alpha1.SelectedSource{Image: "gcr.io/cn:v1", Digest: "sha256:abc"})
	prepare.Status.Conditions = []operatorv1alpha1.Condition{{
		Type:   operatorv1alpha1.CondRegistrySelected,
		Status: metav1.ConditionTrue,
		Reason: operatorv1alpha1.ReasonSelectionComplete.String(),
	}}

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: prepare.Spec.Orbit},
		Spec: operatorv1alpha1.OrbitSpec{
			Upgrade: &operatorv1alpha1.OrbitUpgrade{
				ConsensusNode: &operatorv1alpha1.ConsensusNodeUpgrade{
					Prefetch: &operatorv1alpha1.PrefetchConfig{Enabled: boolPtr(false)},
				},
			},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(prepare, orbit).
		WithStatusSubresource(prepare).
		WithIndex(&operatorv1alpha1.NetworkUpgradePrepare{}, fieldOperationID, prepareIndexFunc).
		WithIndex(&operatorv1alpha1.ConsensusCapsule{}, fieldCapsuleOrbit, capsuleOrbitIndexFunc).
		Build()

	r := NewNetworkUpgradePrepareReconciler(c, scheme)
	r.Recorder = events.NewFakeRecorder(64)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(prepare),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jobs batchv1.JobList
	if err := c.List(context.Background(), &jobs, client.InNamespace(prepare.Namespace)); err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 0 {
		t.Errorf("expected no prefetch Jobs when disabled, got %d", len(jobs.Items))
	}

	var got operatorv1alpha1.NetworkUpgradePrepare
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(prepare), &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	completed := findCondition(got.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
	if completed == nil || completed.Status != metav1.ConditionFalse ||
		completed.Reason != operatorv1alpha1.ReasonPrefetchDisabled.String() {
		t.Errorf("expected Completed=False/PrefetchDisabled, got %+v", completed)
	}
	if _, ok := prefetchCompleted(&got); ok {
		t.Error("prefetchCompleted should report not-succeeded when prefetch is disabled")
	}
}
