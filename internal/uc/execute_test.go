// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const wantExecuteName = "upgrade-0.75.0-20260413t120000z-execute"

// wantConsensusNode is the ConsensusCapsule name derived from the UC pod name
// ("my-node-0" → "my-node") set by executeTestConfig.
const wantConsensusNode = "my-node"

// pinnedImage is a digest-pinned image seeded into the Freeze CR's
// spec.selectedSources so tests can assert it is copied verbatim into the
// Execute CR.
const pinnedImage = "ghcr.io/hashgraph/consensus-node@sha256:pinned"

// nowFrozenMtime is the fixed mtime for now_frozen.mf in tests — after
// markerCreatedAt (execute_immediate.mf mtime) so staleness guard 1 passes.
var nowFrozenMtime = markerCreatedAt.Add(3 * time.Hour)

// executeTestConfig returns a testConfig with PodName set so CapsuleName()
// (spec.consensusNode) resolves to a real value — Config.Validate would reject an
// empty pod name, and the Execute CR requires a non-empty consensusNode.
func executeTestConfig(markerRoot, crPersist string) Config {
	cfg := testConfig(markerRoot, crPersist)
	cfg.PodName = "my-node-0"
	return cfg
}

// writeNowFrozenMarker writes now_frozen.mf with the given mtime into root. The
// handler reads only the marker's mtime (not its content), so the body is a
// placeholder.
func writeNowFrozenMarker(t *testing.T, root string, mtime time.Time) {
	t.Helper()
	p := filepath.Join(root, MarkerNowFrozen)
	if err := os.WriteFile(p, []byte("NOW_FROZEN"), 0o644); err != nil {
		t.Fatalf("write now_frozen marker: %v", err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("chtimes now_frozen marker: %v", err)
	}
}

// writeNowFrozenPackage writes execute_immediate.mf + manifests/ (via writePackage)
// then adds now_frozen.mf with nowFrozenMtime (newer than execute_immediate.mf).
func writeNowFrozenPackage(t *testing.T, componentsYAML string) string {
	t.Helper()
	root := writePackage(t, componentsYAML)
	writeNowFrozenMarker(t, root, nowFrozenMtime)
	return root
}

// seedFreezeCR creates a NetworkUpgradeFreeze CR in the fake cluster with the
// given operationId, optional status.phase, and spec.selectedSources — mirroring
// what HandleFreezeScheduled would have created. An empty phase is non-terminal.
func seedFreezeCR(t *testing.T, f *Factory, operationID string, phase operatorv1alpha1.Phase, selectedSources operatorv1alpha1.SelectedSources) {
	t.Helper()
	images := operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0"},
	}
	cr, err := buildFreezeCR(f.cfg, operationID, wantScheduledTime, images, selectedSources)
	if err != nil {
		t.Fatalf("build freeze: %v", err)
	}
	if phase != "" {
		if err := unstructured.SetNestedField(cr.Object, string(phase), "status", "phase"); err != nil {
			t.Fatalf("set freeze phase: %v", err)
		}
	}
	if _, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed freeze: %v", err)
	}
}

func getExecute(t *testing.T, f *Factory, operationID string) *unstructured.Unstructured {
	t.Helper()
	cr, err := f.client.Resource(networkUpgradeExecuteGVR).Namespace(f.cfg.Namespace).
		Get(context.Background(), executeCRName(operationID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get execute CR: %v", err)
	}
	return cr
}

func assertNoExecute(t *testing.T, f *Factory) {
	t.Helper()
	list, err := f.client.Resource(networkUpgradeExecuteGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no execute CR, got %d", len(list.Items))
	}
}

func TestBuildExecuteCR(t *testing.T) {
	cfg := executeTestConfig(t.TempDir(), t.TempDir())
	images := operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0"},
	}
	ss := operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: pinnedImage, Digest: "sha256:pinned"},
	}

	cr, err := buildExecuteCR(cfg, wantOperationID, images, ss)
	if err != nil {
		t.Fatalf("buildExecuteCR: %v", err)
	}

	if cr.GetName() != wantExecuteName {
		t.Fatalf("name = %q, want %q", cr.GetName(), wantExecuteName)
	}
	if cr.GetKind() != kindNetworkUpgradeExecute {
		t.Fatalf("kind = %q, want %q", cr.GetKind(), kindNetworkUpgradeExecute)
	}
	labels := cr.GetLabels()
	if labels[operatorv1alpha1.LabelOperationID] != wantOperationID {
		t.Fatalf("op-id label = %q, want %q", labels[operatorv1alpha1.LabelOperationID], wantOperationID)
	}
	if labels[operatorv1alpha1.LabelStep] != operatorv1alpha1.StepExecute {
		t.Fatalf("step label = %q, want %q", labels[operatorv1alpha1.LabelStep], operatorv1alpha1.StepExecute)
	}
	if cr.GetAnnotations()[annotationCreatedBy] != createdByUC {
		t.Fatalf("created-by annotation = %q", cr.GetAnnotations()[annotationCreatedBy])
	}
	node, _, _ := unstructured.NestedString(cr.Object, "spec", "consensusNode")
	if node != wantConsensusNode {
		t.Fatalf("spec.consensusNode = %q, want %q", node, wantConsensusNode)
	}
	orbit, _, _ := unstructured.NestedString(cr.Object, "spec", "orbit")
	if orbit != "my-orbit" {
		t.Fatalf("spec.orbit = %q", orbit)
	}
	opID, _, _ := unstructured.NestedString(cr.Object, "spec", "operationId")
	if opID != wantOperationID {
		t.Fatalf("spec.operationId = %q", opID)
	}
	img, _, _ := unstructured.NestedString(cr.Object, "spec", "selectedSources", "consensusNode", "image")
	if img != pinnedImage {
		t.Fatalf("spec.selectedSources.consensusNode.image = %q, want %q", img, pinnedImage)
	}
}

func TestHandleNowFrozenCreatesExecuteCR(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, operatorv1alpha1.SelectedSources{})

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("HandleNowFrozen: %v", err)
	}

	cr := getExecute(t, f, wantOperationID)
	if cr.GetName() != wantExecuteName {
		t.Fatalf("name = %q, want %q", cr.GetName(), wantExecuteName)
	}
	labels := cr.GetLabels()
	if labels[operatorv1alpha1.LabelOperationID] != wantOperationID {
		t.Fatalf("op-id label = %q, want %q", labels[operatorv1alpha1.LabelOperationID], wantOperationID)
	}
	if labels[operatorv1alpha1.LabelStep] != operatorv1alpha1.StepExecute {
		t.Fatalf("step label = %q, want %q", labels[operatorv1alpha1.LabelStep], operatorv1alpha1.StepExecute)
	}
	if cr.GetAnnotations()[annotationCreatedBy] != createdByUC {
		t.Fatalf("created-by annotation = %q", cr.GetAnnotations()[annotationCreatedBy])
	}
	node, _, _ := unstructured.NestedString(cr.Object, "spec", "consensusNode")
	if node != wantConsensusNode {
		t.Fatalf("spec.consensusNode = %q, want %q", node, wantConsensusNode)
	}
	// Single-writer rule: the UC must not write status.phase on the Execute CR.
	if phase, found, _ := unstructured.NestedString(cr.Object, "status", "phase"); found && phase != "" {
		t.Fatalf("UC wrote status.phase = %q, want unset", phase)
	}
	// persist-before-send: the file is removed after a successful create.
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up after create: %v", rest)
	}
}

// TestHandleNowFrozenCopiesPayloadFromFreeze verifies spec.images and the pinned
// spec.selectedSources are copied verbatim from the Freeze CR (which carried them
// from Prepare), keeping the Execute CR self-contained.
func TestHandleNowFrozenCopiesPayloadFromFreeze(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	ss := operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: pinnedImage, Digest: "sha256:pinned"},
	}
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, ss)

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("HandleNowFrozen: %v", err)
	}

	cr := getExecute(t, f, wantOperationID)
	ver, _, _ := unstructured.NestedString(cr.Object, "spec", "images", "consensusNode", "version")
	if ver != "0.75.0" {
		t.Fatalf("spec.images.consensusNode.version = %q, want 0.75.0", ver)
	}
	img, _, _ := unstructured.NestedString(cr.Object, "spec", "selectedSources", "consensusNode", "image")
	if img != pinnedImage {
		t.Fatalf("spec.selectedSources.consensusNode.image = %q, want %q (pinned value from Freeze)", img, pinnedImage)
	}
}

// TestHandleNowFrozenNoFreezeIsNoOp covers the case where no Freeze CR exists yet
// (early tick): the handler retries, creating nothing.
func TestHandleNowFrozenNoFreezeIsNoOp(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("want nil with no Freeze, got %v", err)
	}
	assertNoExecute(t, f)
}

// TestHandleNowFrozenFreezeFailed verifies that a Failed Freeze blocks Execute
// creation (the gate): the handler halts without error and creates nothing.
func TestHandleNowFrozenFreezeFailed(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseFailed, operatorv1alpha1.SelectedSources{})

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("want nil (halt not error), got %v", err)
	}
	assertNoExecute(t, f)
}

// TestHandleNowFrozenFreezeInProgress verifies the handler retries (nil, no CR)
// when the Freeze CR exists but has not yet Succeeded.
func TestHandleNowFrozenFreezeInProgress(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, "", operatorv1alpha1.SelectedSources{}) // empty phase = in-progress

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("want nil (retry), got %v", err)
	}
	assertNoExecute(t, f)
}

// TestHandleNowFrozenStaleMarkerIsSkipped verifies that a now_frozen.mf whose
// mtime predates execute_immediate.mf is silently ignored (staleness guard 1).
func TestHandleNowFrozenStaleMarkerIsSkipped(t *testing.T) {
	root := writePackage(t, validComponentsYAML) // execute_immediate.mf mtime = markerCreatedAt
	writeNowFrozenMarker(t, root, markerCreatedAt.Add(-1*time.Hour))
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, operatorv1alpha1.SelectedSources{})

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("want nil (stale marker skipped), got %v", err)
	}
	assertNoExecute(t, f)
}

// TestHandleNowFrozenFallbackWhenNoExecuteMarker verifies that when
// execute_immediate.mf is absent the handler resolves the operationId via
// version-based cluster lookup and still creates the Execute CR.
func TestHandleNowFrozenFallbackWhenNoExecuteMarker(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, manifestsSubdir)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, componentsManifestFile), []byte(validComponentsYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	writeNowFrozenMarker(t, root, nowFrozenMtime)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	// Fallback resolves operationId via version prefix from a non-Failed Prepare.
	seedSucceededPrepare(t, f, wantOperationID, nil)
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, operatorv1alpha1.SelectedSources{})

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("HandleNowFrozen: %v", err)
	}
	getExecute(t, f, wantOperationID)
}

func TestHandleNowFrozenIsIdempotent(t *testing.T) {
	root := writeNowFrozenPackage(t, validComponentsYAML)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, operatorv1alpha1.SelectedSources{})

	for i := 0; i < 3; i++ {
		if err := f.HandleNowFrozen(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	list, err := f.client.Resource(networkUpgradeExecuteGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("re-dispatch created %d execute CRs, want exactly 1", len(list.Items))
	}
}

// TestHandleNowFrozenEqualMtimeProceeds verifies a now_frozen.mf sharing
// execute_immediate.mf's mtime (possible at 1s filesystem resolution) is NOT
// treated as stale — the handler proceeds and creates the Execute CR.
func TestHandleNowFrozenEqualMtimeProceeds(t *testing.T) {
	root := writePackage(t, validComponentsYAML)   // execute_immediate.mf mtime = markerCreatedAt
	writeNowFrozenMarker(t, root, markerCreatedAt) // equal mtime
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))
	seedFreezeCR(t, f, wantOperationID, operatorv1alpha1.PhaseSucceeded, operatorv1alpha1.SelectedSources{})

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("HandleNowFrozen: %v", err)
	}
	getExecute(t, f, wantOperationID) // must exist — equal mtime is not stale
}

// TestHandleNowFrozenFallbackInvalidManifestHalts verifies that when
// execute_immediate.mf is absent and the manifest is invalid, the handler halts
// terminally (nil, no CR) — not retried and not mislabeled as "both absent".
func TestHandleNowFrozenFallbackInvalidManifestHalts(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, manifestsSubdir)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	// Invalid: unknown schemaVersion.
	if err := os.WriteFile(filepath.Join(manifestDir, componentsManifestFile),
		[]byte("schemaVersion: 99\nimages:\n  consensusNode:\n    version: \"0.75.0\"\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	writeNowFrozenMarker(t, root, nowFrozenMtime)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))

	if err := f.HandleNowFrozen(context.Background()); err != nil {
		t.Fatalf("want nil (terminal halt on invalid manifest), got %v", err)
	}
	assertNoExecute(t, f)
}

// TestHandleNowFrozenFallbackManifestAbsentRetries verifies that a transient
// manifest read failure (manifest absent) surfaces as an error so the tick is
// retried/reportable, rather than being silently swallowed.
func TestHandleNowFrozenFallbackManifestAbsentRetries(t *testing.T) {
	root := t.TempDir() // no execute_immediate.mf, no manifest
	writeNowFrozenMarker(t, root, nowFrozenMtime)
	f := newFakeFactory(t, executeTestConfig(root, t.TempDir()))

	if err := f.HandleNowFrozen(context.Background()); err == nil {
		t.Fatal("want transient error (retry) when manifest is absent, got nil")
	}
	assertNoExecute(t, f)
}
