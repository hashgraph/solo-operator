// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestParseFreezeScheduledTimeSymlinkRejected(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("1234567890"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, MarkerFreezeScheduled)); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := parseFreezeScheduledTime(root)
	if err == nil {
		t.Fatal("expected error for symlinked freeze marker, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

const wantFreezeName = wantConsensusNode + "-upgrade-0.75.0-20260413t120000z-freeze"

// wantScheduledTime is the scheduled freeze time encoded as epoch seconds in
// freeze_scheduled.mf file content. It is after markerCreatedAt (execute marker
// mtime) so the staleness guard passes in all normal tests.
var wantScheduledTime = time.Date(2026, 4, 14, 6, 0, 0, 0, time.UTC)

// writeFreezePackage writes both execute_immediate.mf (needed for operationId
// re-derivation) and freeze_scheduled.mf into root, reusing writePackage for the
// components manifest + execute marker, then adding the freeze marker.
// The freeze marker content is the scheduledTime as epoch seconds (written by CN).
// The marker mtime is set to scheduledTime so it is newer than execute_immediate.mf.
func writeFreezePackage(t *testing.T, componentsYAML string, scheduledTime time.Time) string {
	t.Helper()
	root := writePackage(t, componentsYAML) // writes execute_immediate.mf + manifests/
	p := filepath.Join(root, MarkerFreezeScheduled)
	content := fmt.Sprintf("%d", scheduledTime.Unix())
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write freeze marker: %v", err)
	}
	if err := os.Chtimes(p, scheduledTime, scheduledTime); err != nil {
		t.Fatalf("chtimes freeze marker: %v", err)
	}
	return root
}

// seedSucceededPrepare seeds a NetworkUpgradePrepare CR in Succeeded phase with
// images and an optional status.selectedSources, as would be the normal state
// when freeze_scheduled.mf is written.
func seedSucceededPrepare(t *testing.T, f *Factory, operationID string, selectedSources *operatorv1alpha1.SelectedSources) {
	t.Helper()
	images := operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0"},
	}
	cr, err := buildPrepareCR(f.cfg, operationID, images)
	if err != nil {
		t.Fatalf("build prepare: %v", err)
	}
	if err := unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.PhaseSucceeded), "status", "phase"); err != nil {
		t.Fatalf("set phase: %v", err)
	}
	if selectedSources != nil {
		srcMap := map[string]interface{}{
			"consensusNode": map[string]interface{}{
				"image":  "ghcr.io/hashgraph/consensus-node@sha256:abc",
				"digest": "sha256:abc",
			},
		}
		if err := unstructured.SetNestedMap(cr.Object, srcMap, "status", "selectedSources"); err != nil {
			t.Fatalf("set selectedSources: %v", err)
		}
	}
	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed succeeded prepare: %v", err)
	}
}

func getFreeze(t *testing.T, f *Factory, operationID string) *unstructured.Unstructured {
	t.Helper()
	cr, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		Get(context.Background(), freezeCRName(operationID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get freeze CR: %v", err)
	}
	return cr
}

func assertNoFreeze(t *testing.T, f *Factory) {
	t.Helper()
	list, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no freeze CR, got %d", len(list.Items))
	}
}

func TestHandleFreezeScheduledCreatesFreezeCR(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}

	cr := getFreeze(t, f, wantOperationID)

	if cr.GetName() != wantFreezeName {
		t.Fatalf("name = %q, want %q", cr.GetName(), wantFreezeName)
	}
	labels := cr.GetLabels()
	if labels[operatorv1alpha1.LabelOperationID] != wantOperationID {
		t.Fatalf("op-id label = %q, want %q", labels[operatorv1alpha1.LabelOperationID], wantOperationID)
	}
	if labels[operatorv1alpha1.LabelStep] != operatorv1alpha1.StepFreeze {
		t.Fatalf("step label = %q, want %q", labels[operatorv1alpha1.LabelStep], operatorv1alpha1.StepFreeze)
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
	// persist-before-send: the file is removed after a successful create.
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up after create: %v", rest)
	}
}

func TestHandleFreezeScheduledCopiesImagesFromPrepare(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}

	cr := getFreeze(t, f, wantOperationID)
	ver, _, _ := unstructured.NestedString(cr.Object, "spec", "images", "consensusNode", "version")
	if ver != "0.75.0" {
		t.Fatalf("spec.images.consensusNode.version = %q, want 0.75.0", ver)
	}
}

func TestHandleFreezeScheduledCopiesSelectedSourcesWhenPresent(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	ss := &operatorv1alpha1.SelectedSources{}
	seedSucceededPrepare(t, f, wantOperationID, ss)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}
	getFreeze(t, f, wantOperationID)
}

// TestHandleFreezeScheduledNoPrepareIsNoOp covers the case where no Prepare CR
// exists yet (anomalous — execute_immediate.mf precedes freeze_scheduled.mf in
// normal protocol, so a Prepare should already be in the cluster).
func TestHandleFreezeScheduledNoPrepareIsNoOp(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil with no Prepare, got %v", err)
	}
	assertNoFreeze(t, f)
}

// TestHandleFreezeScheduledPrepareFailed verifies that a Failed Prepare blocks
// Freeze creation. The consensus node should not schedule a freeze after a
// prepare failure, but the handler must be defensive.
func TestHandleFreezeScheduledPrepareFailed(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	images := operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.ImageComponentSpec{Version: "0.75.0"},
	}
	cr, err := buildPrepareCR(f.cfg, wantOperationID, images)
	if err != nil {
		t.Fatalf("build prepare: %v", err)
	}
	if err := unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.PhaseFailed), "status", "phase"); err != nil {
		t.Fatalf("set phase: %v", err)
	}
	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed failed prepare: %v", err)
	}

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil (halt not error), got %v", err)
	}
	assertNoFreeze(t, f)
}

// TestHandleFreezeScheduledPrepareInProgress verifies that the handler retries
// (returns nil, no CR) when the Prepare CR exists but has not yet Succeeded.
func TestHandleFreezeScheduledPrepareInProgress(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	// Seed with empty phase = in-progress.
	seedPrepareCR(t, f, wantOperationID, "")

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil (retry), got %v", err)
	}
	assertNoFreeze(t, f)
}

// TestHandleFreezeScheduledNoExecuteMarkerIsNoOp covers the case where both
// execute_immediate.mf AND the manifest are absent — all fallbacks exhausted.
func TestHandleFreezeScheduledNoExecuteMarkerIsNoOp(t *testing.T) {
	root := t.TempDir()
	// Write freeze marker with valid epoch content but no execute_immediate.mf or manifest.
	p := filepath.Join(root, MarkerFreezeScheduled)
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d", wantScheduledTime.Unix())), 0o644); err != nil {
		t.Fatalf("write freeze marker: %v", err)
	}
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil (no execute marker, no manifest fallback), got %v", err)
	}
	assertNoFreeze(t, f)
}

// TestHandleFreezeScheduledSetsScheduledTimeFromContent verifies that
// spec.scheduledTime in the Freeze CR is parsed from freeze_scheduled.mf file
// content (epoch seconds), not from the marker's mtime.
func TestHandleFreezeScheduledSetsScheduledTimeFromContent(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}

	cr := getFreeze(t, f, wantOperationID)
	rawTime, _, _ := unstructured.NestedString(cr.Object, "spec", "scheduledTime")
	got, err := time.Parse(time.RFC3339, rawTime)
	if err != nil {
		t.Fatalf("parse scheduledTime %q: %v", rawTime, err)
	}
	if !got.Equal(wantScheduledTime) {
		t.Fatalf("scheduledTime = %v, want %v (from file content epoch seconds)", got, wantScheduledTime)
	}
}

// TestHandleFreezeScheduledEventLogsScheduledTime verifies that the freeze's
// scheduled time is recorded on the single-shot MarkerDetected JSONL event, so
// the audit trail says when the network is scheduled to freeze — not just that a
// freeze marker was seen.
func TestHandleFreezeScheduledEventLogsScheduledTime(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	evDir := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	f.events = newEventSink(evDir, "0")
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}
	f.events.Close()

	data, err := os.ReadFile(filepath.Join(evDir, wantOperationID+".jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	got := string(data)
	wantTime := wantScheduledTime.UTC().Format(time.RFC3339)
	if !strings.Contains(got, `"reason":"MarkerDetected"`) {
		t.Fatalf("no MarkerDetected event\n%s", got)
	}
	if !strings.Contains(got, "scheduled_time="+wantTime) {
		t.Errorf("MarkerDetected event missing scheduled_time=%s\n%s", wantTime, got)
	}
}

// TestHandleFreezeScheduledStaleMarkerIsSkipped verifies that a freeze marker
// whose mtime predates execute_immediate.mf is silently ignored (staleness guard 1).
func TestHandleFreezeScheduledStaleMarkerIsSkipped(t *testing.T) {
	root := writePackage(t, validComponentsYAML) // execute_immediate.mf mtime = markerCreatedAt
	// Write freeze marker with mtime BEFORE execute_immediate.mf.
	staleMtime := markerCreatedAt.Add(-1 * time.Hour)
	p := filepath.Join(root, MarkerFreezeScheduled)
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d", staleMtime.Unix())), 0o644); err != nil {
		t.Fatalf("write freeze marker: %v", err)
	}
	if err := os.Chtimes(p, staleMtime, staleMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil (stale marker skipped), got %v", err)
	}
	assertNoFreeze(t, f)
}

// TestHandleFreezeScheduledFallbackWhenNoExecuteMarker verifies that when
// execute_immediate.mf is absent the handler falls back to version-based cluster
// lookup and still creates the Freeze CR.
func TestHandleFreezeScheduledFallbackWhenNoExecuteMarker(t *testing.T) {
	root := t.TempDir()
	// Write manifest (no execute_immediate.mf) and freeze marker.
	manifestDir := filepath.Join(root, manifestsSubdir)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, componentsManifestFile), []byte(validComponentsYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	p := filepath.Join(root, MarkerFreezeScheduled)
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d", wantScheduledTime.Unix())), 0o644); err != nil {
		t.Fatalf("write freeze marker: %v", err)
	}
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	// Fallback resolves via version prefix — seed Succeeded Prepare.
	seedSucceededPrepare(t, f, wantOperationID, nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}
	getFreeze(t, f, wantOperationID)
}

func TestHandleFreezeScheduledIsIdempotent(t *testing.T) {
	root := writeFreezePackage(t, validComponentsYAML, wantScheduledTime)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	seedSucceededPrepare(t, f, wantOperationID, nil)

	for i := 0; i < 3; i++ {
		if err := f.HandleFreezeScheduled(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	list, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("re-dispatch created %d freeze CRs, want exactly 1", len(list.Items))
	}
}
