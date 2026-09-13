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

const wantAbortName = wantConsensusNode + "-upgrade-0.75.0-20260413t120000z-abort"

// abortMtime is the fixed mtime for freeze_aborted.mf in tests — after
// markerCreatedAt (execute_immediate.mf mtime) so the staleness guard passes.
var abortMtime = markerCreatedAt.Add(2 * time.Hour)

// writeAbortMarker writes freeze_aborted.mf with abortMtime into root.
func writeAbortMarker(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, MarkerFreezeAborted)
	if err := os.WriteFile(p, []byte("FREEZE_ABORTED"), 0o644); err != nil {
		t.Fatalf("write abort marker: %v", err)
	}
	if err := os.Chtimes(p, abortMtime, abortMtime); err != nil {
		t.Fatalf("chtimes abort marker: %v", err)
	}
}

// seedPrepareCR creates a NetworkUpgradePrepare CR in the fake cluster with the
// given operationId and optional status.phase, mirroring what a prior tick (or a
// prior process) would have created. An empty phase is non-terminal.
func seedPrepareCR(t *testing.T, f *Factory, operationID, phase string) {
	t.Helper()
	cr, err := buildPrepareCR(f.cfg, operationID, operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0"},
	})
	if err != nil {
		t.Fatalf("build prepare: %v", err)
	}
	if phase != "" {
		if err := unstructured.SetNestedField(cr.Object, phase, "status", "phase"); err != nil {
			t.Fatalf("set phase: %v", err)
		}
	}
	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed prepare: %v", err)
	}
}

func getAbort(t *testing.T, f *Factory, operationID string) *unstructured.Unstructured {
	t.Helper()
	cr, err := f.client.Resource(networkUpgradeFreezeAbortGVR).Namespace(f.cfg.Namespace).
		Get(context.Background(), abortCRName(operationID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get abort CR: %v", err)
	}
	return cr
}

func TestResolveOperationIDFromCluster(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	seedPrepareCR(t, f, wantOperationID, "")

	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != wantOperationID {
		t.Fatalf("operationId = %q, want %q", got, wantOperationID)
	}
}

func TestResolveOperationIDSkipsTerminalPrepare(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	// A completed prior upgrade must not be resolved as the in-flight one.
	seedPrepareCR(t, f, "upgrade-0.74.0-20260101t000000z", string(operatorv1alpha1.PhaseSucceeded))

	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "" {
		t.Fatalf("operationId = %q, want empty (only a terminal Prepare exists)", got)
	}
}

func TestResolveOperationIDDiskFallback(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	// No CR in-cluster, but a Prepare persisted on disk (crash window before its
	// Create was acknowledged).
	cr, err := buildPrepareCR(f.cfg, wantOperationID, operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0"},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := f.store.Persist(wantOperationID, operatorv1alpha1.StepPrepare, cr); err != nil {
		t.Fatalf("persist: %v", err)
	}

	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != wantOperationID {
		t.Fatalf("operationId = %q, want %q (disk fallback)", got, wantOperationID)
	}
}

func TestResolveOperationIDDiskFallbackPicksMostRecentByTimestamp(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))

	// Two persisted Prepares where lexical and chronological order DISAGREE:
	// the older upgrade has the lexically-larger id (version "0.9.0" sorts above
	// "0.75.0"), so a lexical max would wrongly pick it. Recency must come from
	// the embedded timestamp.
	older := wantConsensusNode + "-upgrade-0.9.0-20260101T000000Z"  // lexically larger, chronologically older
	newer := wantConsensusNode + "-upgrade-0.75.0-20260413T120000Z" // lexically smaller, chronologically newer
	for _, id := range []string{older, newer} {
		cr, err := buildPrepareCR(f.cfg, id, operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "x"},
		})
		if err != nil {
			t.Fatalf("build %s: %v", id, err)
		}
		if _, err := f.store.Persist(id, operatorv1alpha1.StepPrepare, cr); err != nil {
			t.Fatalf("persist %s: %v", id, err)
		}
	}

	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != newer {
		t.Fatalf("operationId = %q, want %q (most recent by timestamp, not lexical)", got, newer)
	}
}

func TestResolveOperationIDBreaksTimestampTieDeterministically(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	// Two non-terminal Prepares for this node with equal creationTimestamp (the fake
	// client leaves it zero). The winner must be a deterministic tie-break on the
	// operationId, not whichever the List happens to return first.
	a := wantConsensusNode + "-upgrade-0.75.0-20260413T120000Z-a"
	b := wantConsensusNode + "-upgrade-0.75.0-20260413T120000Z-b"
	seedPrepareCR(t, f, a, "")
	seedPrepareCR(t, f, b, "")

	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != b {
		t.Fatalf("operationId = %q, want %q (highest operationId wins on timestamp tie)", got, b)
	}
}

func TestFetchPreparePayload_DeterministicPickOnDuplicate(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	mk := func(name, version string) {
		cr, err := buildPrepareCR(f.cfg, wantOperationID, operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: version},
		})
		if err != nil {
			t.Fatalf("build %s: %v", name, err)
		}
		cr.SetName(name)
		if err := unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.PhaseSucceeded), "status", "phase"); err != nil {
			t.Fatalf("set phase: %v", err)
		}
		if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
			Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	// Two CRs share an operationId but differ by name (a defensive duplicate). Sorting
	// by name before picking Items[0] makes the selection deterministic: "aaa" wins.
	mk("zzz-dup", "0.75.0")
	mk("aaa-dup", "0.75.1")

	images, _, cr, ready, err := f.fetchPreparePayload(context.Background(), wantOperationID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready=true for a Succeeded Prepare")
	}
	if cr.GetName() != "aaa-dup" || images.ConsensusNode.Version != "0.75.1" {
		t.Fatalf("non-deterministic pick: name=%q version=%q, want aaa-dup/0.75.1", cr.GetName(), images.ConsensusNode.Version)
	}
}

func TestResolveOperationIDNoneReturnsEmpty(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	got, err := f.resolveOperationID(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "" {
		t.Fatalf("operationId = %q, want empty (no Prepare anywhere)", got)
	}
}

func TestHandleFreezeAbortedCreatesCR(t *testing.T) {
	// Abort handler re-derives operationId from execute_immediate.mf + manifest.
	root := writePackage(t, validComponentsYAML)
	writeAbortMarker(t, root)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("HandleFreezeAborted: %v", err)
	}

	cr := getAbort(t, f, wantOperationID)
	if got := cr.GetName(); got != wantAbortName {
		t.Fatalf("name = %q, want %q", got, wantAbortName)
	}
	if got := cr.GetNamespace(); got != "solo" {
		t.Fatalf("namespace = %q", got)
	}
	labels := cr.GetLabels()
	if labels[operatorv1alpha1.LabelOperationID] != wantOperationID {
		t.Fatalf("operation-id label = %q", labels[operatorv1alpha1.LabelOperationID])
	}
	if labels[operatorv1alpha1.LabelStep] != operatorv1alpha1.StepAbort {
		t.Fatalf("step label = %q, want %q", labels[operatorv1alpha1.LabelStep], operatorv1alpha1.StepAbort)
	}
	if cr.GetAnnotations()[annotationCreatedBy] != createdByUC {
		t.Fatalf("created-by annotation = %q", cr.GetAnnotations()[annotationCreatedBy])
	}
	orbit, _, _ := unstructured.NestedString(cr.Object, "spec", "orbit")
	if orbit != "my-orbit" {
		t.Fatalf("spec.orbit = %q", orbit)
	}
	opID, _, _ := unstructured.NestedString(cr.Object, "spec", "operationId")
	if opID != wantOperationID {
		t.Fatalf("spec.operationId = %q", opID)
	}
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up after create: %v", rest)
	}
}

// TestHandleFreezeAbortedNoExecuteMarkerIsNoOp covers the case where both
// execute_immediate.mf and the manifest are absent — all fallbacks exhausted.
func TestHandleFreezeAbortedNoExecuteMarkerIsNoOp(t *testing.T) {
	root := t.TempDir()
	writeAbortMarker(t, root) // no execute_immediate.mf, no manifest
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("want nil (no execute marker, no manifest), got %v", err)
	}
	list, err := f.client.Resource(networkUpgradeFreezeAbortGVR).Namespace("solo").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no abort CR, got %d", len(list.Items))
	}
}

// TestHandleFreezeAbortedStaleMarkerIsSkipped verifies that an abort marker
// whose mtime predates execute_immediate.mf is silently ignored (staleness guard).
func TestHandleFreezeAbortedStaleMarkerIsSkipped(t *testing.T) {
	root := writePackage(t, validComponentsYAML) // execute_immediate.mf mtime = markerCreatedAt
	// Write abort marker with mtime BEFORE execute_immediate.mf.
	staleMtime := markerCreatedAt.Add(-1 * time.Hour)
	p := filepath.Join(root, MarkerFreezeAborted)
	if err := os.WriteFile(p, []byte("FREEZE_ABORTED"), 0o644); err != nil {
		t.Fatalf("write abort marker: %v", err)
	}
	if err := os.Chtimes(p, staleMtime, staleMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("want nil (stale marker), got %v", err)
	}
	list, err := f.client.Resource(networkUpgradeFreezeAbortGVR).Namespace("solo").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no abort CR (stale), got %d", len(list.Items))
	}
}

func TestHandleFreezeAbortedIsIdempotent(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	writeAbortMarker(t, root)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	for i := 0; i < 3; i++ {
		if err := f.HandleFreezeAborted(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	list, err := f.client.Resource(networkUpgradeFreezeAbortGVR).Namespace("solo").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("re-dispatch created %d abort CRs, want exactly 1", len(list.Items))
	}
}

func TestHandleFreezeAbortedDoesNotDeleteSiblings(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	writeAbortMarker(t, root)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	// Seed a Prepare CR as a sibling audit record — must survive the abort.
	seedPrepareCR(t, f, wantOperationID, string(operatorv1alpha1.PhaseSucceeded))

	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("HandleFreezeAborted: %v", err)
	}

	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
		Get(context.Background(), prepareCRName(wantOperationID), metav1.GetOptions{}); err != nil {
		t.Fatalf("sibling Prepare CR was removed by abort: %v", err)
	}
}
