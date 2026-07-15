// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// TestObservePhaseWritesOnTransitionOnly verifies the #1003 phase monitor writes
// one JSONL line per actual phase transition and suppresses repeats of the same
// phase (re-delivered on every watch tick).
func TestObservePhaseWritesOnTransitionOnly(t *testing.T) {
	evDir := t.TempDir()
	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	cr := buildExecuteCR("upgrade-0.75.0-20260413T120000Z", crCreatedAt)
	setPhase := func(p string) {
		_ = unstructured.SetNestedField(cr.Object, p, "status", "phase")
	}

	setPhase("Running")
	m.observePhase(cr)
	m.observePhase(cr) // same phase — suppressed
	setPhase("ReadyForProvisionerDaemon")
	m.observePhase(cr) // transition — logged
	setPhase("Succeeded")
	m.observePhase(cr) // transition — logged
	m.events.Close()

	data, err := os.ReadFile(filepath.Join(evDir, "upgrade-0.75.0-20260413T120000Z.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	got := string(data)
	if c := strings.Count(got, `"reason":"PhaseChanged"`); c != 3 {
		t.Fatalf("PhaseChanged count = %d, want 3\n%s", c, got)
	}
	for _, want := range []string{"Running", "ReadyForProvisionerDaemon", "Succeeded"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing phase %q in log:\n%s", want, got)
		}
	}
}

// TestDaemonTriggeredEmittedOncePerOperation verifies DaemonTriggered is written
// to JSONL exactly once even though the Execute CR is re-observed on every watch
// tick while at ReadyForProvisionerDaemon, and that it fires with a nil proxy
// (mainnet mode).
func TestDaemonTriggeredEmittedOncePerOperation(t *testing.T) {
	evDir := t.TempDir()
	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	cr := buildExecuteCR("upgrade-0.75.0-20260413T120000Z", crCreatedAt)
	_ = unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon), "status", "phase")

	for i := 0; i < 3; i++ {
		m.handleExecuteEvent(context.Background(), cr, nil) // nil proxy = mainnet
	}
	m.events.Close()

	data, err := os.ReadFile(filepath.Join(evDir, "upgrade-0.75.0-20260413T120000Z.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if c := strings.Count(string(data), `"reason":"DaemonTriggered"`); c != 1 {
		t.Fatalf("DaemonTriggered count = %d, want 1\n%s", c, data)
	}
}

// buildExecuteCR builds a minimal NetworkUpgradeExecute Unstructured object with
// the given operationId and creationTimestamp. It mirrors the structure used
// throughout the UC (spec.operationId + label).
func buildExecuteCR(operationID string, createdAt time.Time) *unstructured.Unstructured {
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": operatorv1alpha1.GroupVersion.String(),
			"kind":       "NetworkUpgradeExecute",
			"metadata": map[string]interface{}{
				"name":      "test-execute",
				"namespace": "solo",
				"labels": map[string]interface{}{
					operatorv1alpha1.LabelOperationID: operationID,
					operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
				},
				"creationTimestamp": createdAt.UTC().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"operationId": operationID,
			},
		},
	}
	cr.SetCreationTimestamp(metav1.NewTime(createdAt))
	return cr
}

// newOutcomeMonitor builds an UpgradeOutcomeMonitor with a fake dynamic client
// and an upgradePath pointing at the given temp dir.
func newOutcomeMonitor(t *testing.T, upgradePath string) *UpgradeOutcomeMonitor {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := fake.NewSimpleDynamicClient(scheme)
	cfg := UpgradeOutcomeMonitorConfig{
		Namespace:   "solo",
		UpgradePath: upgradePath,
	}
	return NewUpgradeOutcomeMonitorWithClient(cfg, client)
}

// crCreatedAt is a fixed CR creation time used to drive temporal guard tests.
var crCreatedAt = time.Date(2026, 5, 21, 18, 25, 31, 0, time.UTC)

// ---------------------------------------------------------------------------
// matchesDiskOperationID tests
// ---------------------------------------------------------------------------

func TestMatchesDiskOperationIDMarkerPresent_OpIDMatches(t *testing.T) {
	// execute_immediate.mf present, opId derived from disk matches CR → true.
	root := writePackage(t, validComponentsYAML)
	m := newOutcomeMonitor(t, root)

	// The CR must carry the same opId that writePackage's marker mtime would mint.
	cr := buildExecuteCR(wantOperationID, crCreatedAt)
	if !m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=true when marker present and opId matches")
	}
}

func TestMatchesDiskOperationIDMarkerPresent_OpIDMismatches(t *testing.T) {
	// execute_immediate.mf present, opId mismatches → false.
	root := writePackage(t, validComponentsYAML)
	m := newOutcomeMonitor(t, root)

	cr := buildExecuteCR("upgrade-0.99.0-20260101T000000Z", crCreatedAt)
	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false when opId does not match disk")
	}
}

func TestMatchesDiskOperationIDNoMarkerEmptyDir(t *testing.T) {
	// execute_immediate.mf absent, current/ empty → false.
	root := t.TempDir()
	m := newOutcomeMonitor(t, root)

	cr := buildExecuteCR(wantOperationID, crCreatedAt)
	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false for empty upgrade dir")
	}
}

func TestMatchesDiskOperationIDNoMarkerFilesOlderThanCR_NoVersion(t *testing.T) {
	// execute_immediate.mf absent, files all older than CR, no VERSION file → true
	// (temporal guard passed, no version to reject).
	root := t.TempDir()

	// Write a file with mtime before the CR creation.
	oldFile := filepath.Join(root, "some_old_file.bin")
	if err := os.WriteFile(oldFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	oldMtime := crCreatedAt.Add(-10 * time.Minute)
	if err := os.Chtimes(oldFile, oldMtime, oldMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	m := newOutcomeMonitor(t, root)
	cr := buildExecuteCR(wantOperationID, crCreatedAt)

	if !m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=true when files predate CR and no VERSION file")
	}
}

func TestMatchesDiskOperationIDNoMarkerFilesOlderThanCR_VersionMatches(t *testing.T) {
	// execute_immediate.mf absent, files all older than CR, VERSION matches → true.
	root := t.TempDir()

	oldMtime := crCreatedAt.Add(-10 * time.Minute)
	oldFile := filepath.Join(root, "some_file.bin")
	if err := os.WriteFile(oldFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(oldFile, oldMtime, oldMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	vfPath := filepath.Join(root, versionFile)
	if err := os.WriteFile(vfPath, []byte("VERSION=0.75.0\nCOMMIT=abc\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	if err := os.Chtimes(vfPath, oldMtime, oldMtime); err != nil {
		t.Fatalf("chtimes VERSION: %v", err)
	}

	m := newOutcomeMonitor(t, root)
	cr := buildExecuteCR(wantOperationID, crCreatedAt) // wantOperationID encodes version 0.75.0

	if !m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=true when files predate CR and VERSION matches")
	}
}

func TestMatchesDiskOperationIDNoMarkerFileNewerThanCR(t *testing.T) {
	// execute_immediate.mf absent, any file newer than CR → false (CN writing).
	root := t.TempDir()

	newFile := filepath.Join(root, "new_file.bin")
	if err := os.WriteFile(newFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	newMtime := crCreatedAt.Add(5 * time.Minute)
	if err := os.Chtimes(newFile, newMtime, newMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	m := newOutcomeMonitor(t, root)
	cr := buildExecuteCR(wantOperationID, crCreatedAt)

	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false when file is newer than CR (CN writing)")
	}
}

func TestMatchesDiskOperationIDNoMarkerVersionMismatches(t *testing.T) {
	// execute_immediate.mf absent, files older than CR, VERSION mismatches → false.
	root := t.TempDir()

	oldMtime := crCreatedAt.Add(-10 * time.Minute)
	oldFile := filepath.Join(root, "some_file.bin")
	if err := os.WriteFile(oldFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(oldFile, oldMtime, oldMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	vfPath := filepath.Join(root, versionFile)
	if err := os.WriteFile(vfPath, []byte("VERSION=0.99.0\nCOMMIT=xyz\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	if err := os.Chtimes(vfPath, oldMtime, oldMtime); err != nil {
		t.Fatalf("chtimes VERSION: %v", err)
	}

	m := newOutcomeMonitor(t, root)
	// wantOperationID encodes version 0.75.0, but VERSION says 0.99.0 → mismatch.
	cr := buildExecuteCR(wantOperationID, crCreatedAt)

	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false when VERSION mismatches CR operationId")
	}
}
