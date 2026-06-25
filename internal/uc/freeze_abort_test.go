// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const wantAbortName = "upgrade-0.75.0-20260413t120000z-abort"

// seedPrepareCR creates a NetworkUpgradePrepare CR in the fake cluster with the
// given operationId and optional status.phase, mirroring what a prior tick (or a
// prior process) would have created. An empty phase is non-terminal.
func seedPrepareCR(t *testing.T, f *Factory, operationID, phase string) {
	t.Helper()
	cr, err := buildPrepareCR(f.cfg, operationID, operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0"},
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
		ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "0.75.0"},
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
	older := "upgrade-0.9.0-20260101T000000Z"  // lexically larger, chronologically older
	newer := "upgrade-0.75.0-20260413T120000Z" // lexically smaller, chronologically newer
	for _, id := range []string{older, newer} {
		cr, err := buildPrepareCR(f.cfg, id, operatorv1alpha1.UpgradeImagesSpec{
			ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{Version: "x"},
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
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	seedPrepareCR(t, f, wantOperationID, "")

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

	// persist-before-send: the file is removed after a successful create.
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up after create: %v", rest)
	}
}

func TestHandleFreezeAbortedNoPrepareCreatesNoCR(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))

	// No Prepare to inherit identity from: the handler is a no-op (returns nil,
	// retries on a later tick) and creates nothing.
	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("want nil with no Prepare, got %v", err)
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

func TestHandleFreezeAbortedIsIdempotent(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	seedPrepareCR(t, f, wantOperationID, "")

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
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	seedPrepareCR(t, f, wantOperationID, "")

	if err := f.HandleFreezeAborted(context.Background()); err != nil {
		t.Fatalf("HandleFreezeAborted: %v", err)
	}

	// The Prepare CR is an audit-trail record and must survive the abort.
	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
		Get(context.Background(), prepareCRName(wantOperationID), metav1.GetOptions{}); err != nil {
		t.Fatalf("sibling Prepare CR was removed by abort: %v", err)
	}
}
