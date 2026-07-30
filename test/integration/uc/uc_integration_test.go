// SPDX-License-Identifier: Apache-2.0

package uc_integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/sanity"
	"github.com/hashgraph/solo-operator/internal/uc"
)

const testConsensusNode = "node0"

// componentsYAML renders a minimal, schema-valid consensus-node-components.yaml
// for the given consensus-node version.
func componentsYAML(version string) string {
	return fmt.Sprintf(`schemaVersion: 1
images:
  consensusNode:
    version: %q
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:6b8e29e8fa5489c7beeff1b789175e7c2e23e369e6902eb2e36cece38fe34843"
    registries:
      - image: %q
`, version, "gcr.io/hedera-registry/consensus-node:"+version)
}

// writePackage lays out a marker directory mirroring the consensus node's
// extracted package (marker + manifests/ in the same root) and returns the root
// and the operationId the handler will mint from (version, pinned marker mtime).
func writePackage(t *testing.T, version string, mtime time.Time) (root, operationID string) {
	t.Helper()
	root = t.TempDir()
	manifestsDir := filepath.Join(root, "manifests")
	if err := os.MkdirAll(manifestsDir, 0o755); err != nil {
		t.Fatalf("mkdir manifests: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestsDir, "consensus-node-components.yaml"), []byte(componentsYAML(version)), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	marker := filepath.Join(root, uc.MarkerExecuteImmediate)
	if err := os.WriteFile(marker, []byte("PREPARE_UPGRADE"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.Chtimes(marker, mtime, mtime); err != nil {
		t.Fatalf("chtimes marker: %v", err)
	}
	return root, fmt.Sprintf("%s-upgrade-%s-%s", testConsensusNode, version, mtime.UTC().Format(sanity.TimestampLayout))
}

// newNamespace creates a fresh namespace named after the test so each test is
// isolated — resolveOperationID and the field-selector counts are namespace
// scoped, so a shared namespace would let one test's CRs leak into another's.
func newNamespace(t *testing.T) string {
	t.Helper()
	ns := "uc-it-" + sanitizeDNS(t.Name())
	nsGVR := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]interface{}{"name": ns},
	}}
	if _, err := dynClient.Resource(nsGVR).Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create namespace %s: %v", ns, err)
	}
	return ns
}

// sanitizeDNS lower-cases and replaces any non [a-z0-9-] rune with '-' so a test
// name yields a valid DNS-1123 label for a namespace.
func sanitizeDNS(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func newFactory(t *testing.T, root, ns string) *uc.Factory {
	t.Helper()
	return uc.NewFactory(uc.Config{
		Mode:          uc.ModeClusterOnly,
		NodeID:        0,
		Orbit:         "my-orbit",
		PodName:       "node0-0",
		Namespace:     ns,
		UpgradePath:   root,
		CRPersistPath: t.TempDir(),
	}, dynClient)
}

func listByOperationID(t *testing.T, g schema.GroupVersionResource, ns, operationID string) []unstructured.Unstructured {
	t.Helper()
	l, err := dynClient.Resource(g).Namespace(ns).List(context.Background(), metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.operationId", operationID).String(),
	})
	if err != nil {
		t.Fatalf("list %s by operationId %q: %v", g.Resource, operationID, err)
	}
	return l.Items
}

// TestSelectableFieldFiltersServerSide is the core of #1057: it proves that the
// spec.operationId selectableField (added to the CRDs in #1050) is honoured by a
// REAL apiserver — something the fake dynamic client cannot validate because it
// ignores field selectors entirely. If the selectableField were removed from
// the CRDs, the apiserver would reject the spec.operationId selector and this
// test would fail, guarding the dedup mechanism against regression.
func TestSelectableFieldFiltersServerSide(t *testing.T) {
	ctx := context.Background()
	ns := newNamespace(t)
	rootA, opA := writePackage(t, "0.75.0", time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC))
	rootB, opB := writePackage(t, "0.76.0", time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC))

	if err := newFactory(t, rootA, ns).HandleExecuteImmediate(ctx); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if err := newFactory(t, rootB, ns).HandleExecuteImmediate(ctx); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// The field selector must narrow to exactly the matching CR, server-side.
	a := listByOperationID(t, prepareGVR, ns, opA)
	if len(a) != 1 {
		t.Fatalf("field selector spec.operationId=%s returned %d items, want 1", opA, len(a))
	}
	if got, _, _ := unstructured.NestedString(a[0].Object, "spec", "operationId"); got != opA {
		t.Fatalf("returned CR has spec.operationId=%q, want %q", got, opA)
	}
	if b := listByOperationID(t, prepareGVR, ns, opB); len(b) != 1 {
		t.Fatalf("field selector spec.operationId=%s returned %d items, want 1", opB, len(b))
	}

	// Sanity: an unfiltered list sees both, so the selector genuinely narrowed.
	all, err := dynClient.Resource(prepareGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all.Items) != 2 {
		t.Fatalf("unfiltered list returned %d, want 2 (the selector narrowed this to 1 above)", len(all.Items))
	}

	// A field selector on a NON-selectable field must be REJECTED by the apiserver
	// — this confirms field-selector enforcement is real (not silently ignored),
	// so the positive assertions above are meaningful.
	_, err = dynClient.Resource(prepareGVR).Namespace(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.orbit", "my-orbit").String(),
	})
	if err == nil {
		t.Fatal("expected apiserver to reject a field selector on the non-selectable field spec.orbit, got nil error")
	}
}

// TestDedupIdempotentAndRecovery verifies the full dedup path against a real
// apiserver: repeated ticks create exactly one CR, and a fresh process (cache
// reseeded via RecoverOnStartup, which lists + reads spec.operationId) does not
// re-create it.
func TestDedupIdempotentAndRecovery(t *testing.T) {
	ctx := context.Background()
	ns := newNamespace(t)
	root, op := writePackage(t, "0.77.0", time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))

	f := newFactory(t, root, ns)
	for i := 0; i < 3; i++ {
		if err := f.HandleExecuteImmediate(ctx); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if got := listByOperationID(t, prepareGVR, ns, op); len(got) != 1 {
		t.Fatalf("after 3 ticks: %d CRs for %s, want exactly 1", len(got), op)
	}

	// New process: empty cache, reseeded from the cluster, must not duplicate.
	f2 := newFactory(t, root, ns)
	if err := f2.RecoverOnStartup(ctx); err != nil {
		t.Fatalf("recover on startup: %v", err)
	}
	if err := f2.HandleExecuteImmediate(ctx); err != nil {
		t.Fatalf("post-recovery tick: %v", err)
	}
	if got := listByOperationID(t, prepareGVR, ns, op); len(got) != 1 {
		t.Fatalf("after recovery+tick: %d CRs for %s, want exactly 1", len(got), op)
	}
}

// TestHandleFreezeAbortedResolvesOperationID verifies HandleFreezeAborted
// resolves the operationId from the existing Prepare's spec.operationId (read
// back from the real apiserver) and creates a matching FreezeAbort CR.
func TestHandleFreezeAbortedResolvesOperationID(t *testing.T) {
	ctx := context.Background()
	ns := newNamespace(t)
	root, op := writePackage(t, "0.78.0", time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC))
	f := newFactory(t, root, ns)

	if err := f.HandleExecuteImmediate(ctx); err != nil {
		t.Fatalf("create prepare: %v", err)
	}
	// Drop the abort marker alongside the package and run the handler.
	if err := os.WriteFile(filepath.Join(root, uc.MarkerFreezeAborted), []byte("FREEZE_ABORTED"), 0o644); err != nil {
		t.Fatalf("write abort marker: %v", err)
	}
	if err := f.HandleFreezeAborted(ctx); err != nil {
		t.Fatalf("HandleFreezeAborted: %v", err)
	}

	aborts := listByOperationID(t, freezeAbortGVR, ns, op)
	if len(aborts) != 1 {
		t.Fatalf("got %d FreezeAbort CRs for %s, want 1", len(aborts), op)
	}
	name := aborts[0].GetName()
	if want := strings.ToLower(op) + "-abort"; name != want {
		t.Fatalf("FreezeAbort name = %q, want %q", name, want)
	}
	if got := aborts[0].GetLabels()[operatorv1alpha1.LabelStep]; got != operatorv1alpha1.StepAbort {
		t.Fatalf("step label = %q, want %q", got, operatorv1alpha1.StepAbort)
	}
}

// TestCRStoreListSkipsCorruptFile verifies the #1050 robustness fix: a single
// corrupt persisted CR file is skipped (with a warning) rather than failing the
// whole recovery scan — so a good CR is still recovered.
func TestCRStoreListSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := uc.NewCRStore(dir)

	goodOp := "upgrade-0.79.0-20260101T000000Z"
	cr := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": operatorv1alpha1.GroupVersion.String(),
		"kind":       "NetworkUpgradePrepare",
		"metadata":   map[string]interface{}{"name": "good", "namespace": "uc-it"},
		"spec":       map[string]interface{}{"operationId": goodOp},
	}}
	if _, err := store.Persist(goodOp, operatorv1alpha1.StepPrepare, cr); err != nil {
		t.Fatalf("persist good CR: %v", err)
	}

	// Write a file that matches the "<operationId>.<step>.yaml" naming scheme so
	// List attempts to parse it, but whose body is not valid YAML.
	corrupt := filepath.Join(dir, "upgrade-0.80.0-20260101T000000Z.prepare.yaml")
	if err := os.WriteFile(corrupt, []byte("{ : not : valid : yaml"), 0o640); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List should skip the corrupt file, not error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d CRs, want 1 (good recovered, corrupt skipped)", len(got))
	}
	if got[0].OperationID != goodOp {
		t.Fatalf("recovered operationId = %q, want %q", got[0].OperationID, goodOp)
	}
}
