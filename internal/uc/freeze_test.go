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

const wantFreezeName = "upgrade-0.75.0-20260413t120000z-freeze"

// wantScheduledTime is the freeze marker mtime pinned for stable tests.
var wantScheduledTime = time.Date(2026, 4, 14, 6, 0, 0, 0, time.UTC)

// writeFreezeMarker writes freeze_scheduled.mf with a pinned mtime.
func writeFreezeMarker(t *testing.T, dir string, mtime time.Time) {
	t.Helper()
	p := filepath.Join(dir, MarkerFreezeScheduled)
	if err := os.WriteFile(p, []byte("FREEZE_SCHEDULED"), 0o644); err != nil {
		t.Fatalf("write freeze marker: %v", err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("chtimes freeze marker: %v", err)
	}
}

// seedPrepareCRWithImages creates a NetworkUpgradePrepare CR pre-populated with
// images and an optional status.selectedSources.
func seedPrepareCRWithImages(t *testing.T, f *Factory, operationID, phase string, selectedSources *operatorv1alpha1.SelectedSources) {
	t.Helper()
	images := operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0"},
	}
	cr, err := buildPrepareCR(f.cfg, operationID, images)
	if err != nil {
		t.Fatalf("build prepare: %v", err)
	}
	if phase != "" {
		if err := unstructured.SetNestedField(cr.Object, phase, "status", "phase"); err != nil {
			t.Fatalf("set phase: %v", err)
		}
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
		t.Fatalf("seed prepare with images: %v", err)
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

func TestHandleFreezeScheduledCreatesFreezeCR(t *testing.T) {
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)
	seedPrepareCRWithImages(t, f, wantOperationID, "", nil)

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
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)
	seedPrepareCRWithImages(t, f, wantOperationID, "", nil)

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
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)
	ss := &operatorv1alpha1.SelectedSources{}
	seedPrepareCRWithImages(t, f, wantOperationID, "", ss)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}
	// The CR is created successfully whether selectedSources is populated or empty.
	getFreeze(t, f, wantOperationID)
}

func TestHandleFreezeScheduledNoPrepareIsNoOp(t *testing.T) {
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)

	// No Prepare in cluster: handler must return nil and create no CR.
	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("want nil with no Prepare, got %v", err)
	}
	list, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no freeze CR, got %d", len(list.Items))
	}
}

func TestHandleFreezeScheduledIsIdempotent(t *testing.T) {
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)
	seedPrepareCRWithImages(t, f, wantOperationID, "", nil)

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

func TestHandleFreezeScheduledSkipsTerminalPrepare(t *testing.T) {
	root := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	writeFreezeMarker(t, root, wantScheduledTime)
	// A terminal Prepare (Succeeded/Failed) must not be resolved as the in-flight
	// upgrade; the handler sees no active Prepare and is a no-op.
	seedPrepareCRWithImages(t, f, wantOperationID, string(operatorv1alpha1.PhaseSucceeded), nil)

	if err := f.HandleFreezeScheduled(context.Background()); err != nil {
		t.Fatalf("HandleFreezeScheduled: %v", err)
	}
	list, err := f.client.Resource(networkUpgradeFreezeGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no freeze CR for terminal Prepare, got %d", len(list.Items))
	}
}
