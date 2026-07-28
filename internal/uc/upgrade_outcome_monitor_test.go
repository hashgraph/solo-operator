// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// TestObservePhaseWritesOnTransitionOnly verifies the #1003 phase monitor writes
// one JSONL line per actual phase transition and suppresses repeats of the same
// phase (re-delivered on every watch tick).
func TestObservePhaseWritesOnTransitionOnly(t *testing.T) {
	evDir := t.TempDir()
	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	cr := newTestExecuteCR("upgrade-0.75.0-20260413T120000Z", crCreatedAt)
	setPhase := func(p string) {
		_ = unstructured.SetNestedField(cr.Object, p, "status", "phase")
	}

	setPhase("Running")
	m.observePhase(cr)
	m.observePhase(cr) // same phase — suppressed
	setPhase("ReadyForProvisionerDaemon")
	m.observePhase(cr) // transition — logged
	setPhase("Succeeded")
	m.observePhase(cr) // transition — logged; Execute Succeeded is operation-terminal

	// Terminal phase on the Execute step must release the per-operation writer.
	m.events.mu.Lock()
	openWriters := len(m.events.writers)
	m.events.mu.Unlock()
	if openWriters != 0 {
		t.Errorf("expected per-operation writer released after terminal phase, %d still open", openWriters)
	}

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

// TestPruneStaleOperations verifies the safety-net sweep evicts dedup state and
// releases the event-log handle for a never-terminal operation past the age
// window, while leaving a recent operation untouched.
func TestPruneStaleOperations(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	oldOp := "upgrade-20260406T000000Z"   // 100 days before now → stale
	freshOp := "upgrade-20260705T000000Z" // 10 days before now → kept

	// timelessOp has no parseable timestamp; it must still age out via the
	// wall-clock first-seen fallback.
	timelessOp := "manual_recovery_op"

	evDir := t.TempDir()
	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	// Seed dedup state, first-seen times, and an open writer for each operation.
	for _, op := range []string{oldOp, freshOp, timelessOp} {
		m.daemonTriggered[op] = struct{}{}
		m.lastPhase[op+"/execute"] = "Running"
		m.events.info(op, "MarkerDetected", "x")
	}
	m.opFirstSeen[oldOp] = time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)      // 100d → stale
	m.opFirstSeen[freshOp] = time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)    // 10d → kept
	m.opFirstSeen[timelessOp] = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // old → stale

	m.pruneStaleOperations(now)

	if _, ok := m.daemonTriggered[oldOp]; ok {
		t.Error("stale daemonTriggered entry not pruned")
	}
	if _, ok := m.lastPhase[oldOp+"/execute"]; ok {
		t.Error("stale lastPhase entry not pruned")
	}
	if _, ok := m.opFirstSeen[timelessOp]; ok {
		t.Error("timestampless operation not pruned via wall-clock fallback")
	}
	if _, ok := m.lastPhase[timelessOp+"/execute"]; ok {
		t.Error("timestampless operation's lastPhase entry not pruned")
	}
	if _, ok := m.daemonTriggered[freshOp]; !ok {
		t.Error("fresh daemonTriggered entry wrongly pruned")
	}
	if _, ok := m.lastPhase[freshOp+"/execute"]; !ok {
		t.Error("fresh lastPhase entry wrongly pruned")
	}
	m.events.mu.Lock()
	_, oldWriterOpen := m.events.writers[oldOp]
	_, freshWriterOpen := m.events.writers[freshOp]
	m.events.mu.Unlock()
	if oldWriterOpen {
		t.Error("stale operation's event-log handle not released")
	}
	if !freshWriterOpen {
		t.Error("fresh operation's event-log handle wrongly released")
	}

	// The stale op's JSONL trail records the eviction as its final line.
	data, err := os.ReadFile(filepath.Join(evDir, oldOp+".jsonl"))
	if err != nil {
		t.Fatalf("read stale op jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"reason":"StaleOperationPruned"`) {
		t.Errorf("stale op JSONL missing StaleOperationPruned line:\n%s", data)
	}
}

// TestObservePhaseTerminalCleanupWithoutStepLabel verifies terminal cleanup keys
// off the CR Kind, not the step label: a label-less Execute CR reaching a
// terminal phase must still release the event-log handle and drop the
// daemonTriggered entry (not leak them until the 90-day sweep).
func TestObservePhaseTerminalCleanupWithoutStepLabel(t *testing.T) {
	op := "upgrade-0.75.0-20260413T120000Z"
	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(t.TempDir(), "0")

	cr := newTestExecuteCR(op, crCreatedAt)
	cr.SetLabels(nil) // strip the step (and op-id) labels — spec.operationId remains
	_ = unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.ExecutePhaseSucceeded), "status", "phase")

	m.daemonTriggered[op] = struct{}{} // pretend DaemonTriggered already fired
	m.observePhase(cr)

	m.mu.Lock()
	_, stillTracked := m.daemonTriggered[op]
	m.mu.Unlock()
	if stillTracked {
		t.Error("daemonTriggered not cleaned for a label-less terminal Execute CR")
	}
	m.events.mu.Lock()
	_, open := m.events.writers[op]
	m.events.mu.Unlock()
	if open {
		t.Error("event-log handle not released for a label-less terminal Execute CR")
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

	cr := newTestExecuteCR("upgrade-0.75.0-20260413T120000Z", crCreatedAt)
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

	// A terminal Execute phase must drop the dedup entry to bound memory.
	_ = unstructured.SetNestedField(cr.Object, string(operatorv1alpha1.ExecutePhaseSucceeded), "status", "phase")
	m.handleExecuteEvent(context.Background(), cr, nil)
	m.mu.Lock()
	_, stillTracked := m.daemonTriggered["upgrade-0.75.0-20260413T120000Z"]
	_, lastPhaseKept := m.lastPhase["upgrade-0.75.0-20260413T120000Z/"+operatorv1alpha1.StepExecute]
	_, firstSeenKept := m.opFirstSeen["upgrade-0.75.0-20260413T120000Z"]
	m.mu.Unlock()
	if stillTracked {
		t.Error("daemonTriggered entry not pruned after terminal phase")
	}
	// lastPhase is retained to dedup relists; opFirstSeen MUST be retained with it
	// so the stale sweep can still age the retained lastPhase out (no orphan leak).
	if !lastPhaseKept {
		t.Error("lastPhase entry wrongly dropped at terminal (needed to dedup relists)")
	}
	if !firstSeenKept {
		t.Error("opFirstSeen dropped at terminal — orphans the retained lastPhase from the sweep")
	}
}

// newTestExecuteCR builds a minimal NetworkUpgradeExecute Unstructured object with
// the given operationId and creationTimestamp. It mirrors the structure used
// throughout the UC (spec.operationId + label).
func newTestExecuteCR(operationID string, createdAt time.Time) *unstructured.Unstructured {
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
	cr := newTestExecuteCR(wantOperationID, crCreatedAt)
	if !m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=true when marker present and opId matches")
	}
}

func TestMatchesDiskOperationIDMarkerPresent_OpIDMismatches(t *testing.T) {
	// execute_immediate.mf present, opId mismatches → false.
	root := writePackage(t, validComponentsYAML)
	m := newOutcomeMonitor(t, root)

	cr := newTestExecuteCR("upgrade-0.99.0-20260101T000000Z", crCreatedAt)
	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false when opId does not match disk")
	}
}

func TestMatchesDiskOperationIDNoMarkerEmptyDir(t *testing.T) {
	// execute_immediate.mf absent, current/ empty → false.
	root := t.TempDir()
	m := newOutcomeMonitor(t, root)

	cr := newTestExecuteCR(wantOperationID, crCreatedAt)
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
	cr := newTestExecuteCR(wantOperationID, crCreatedAt)

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
	cr := newTestExecuteCR(wantOperationID, crCreatedAt) // wantOperationID encodes version 0.75.0

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
	cr := newTestExecuteCR(wantOperationID, crCreatedAt)

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
	cr := newTestExecuteCR(wantOperationID, crCreatedAt)

	if m.matchesDiskOperationID(cr, nil) {
		t.Fatal("expected matchesDiskOperationID=false when VERSION mismatches CR operationId")
	}
}

// TestHandleDeleteEventEvictsAllCaches verifies that a watch.Deleted event
// evicts per-operation state from all three cache layers: the monitor's own
// maps (lastPhase, opFirstSeen, daemonTriggered), the ProvisionerProxy
// completedOpIDs, and the Factory dedup cache (via the onDelete callback).
func TestHandleDeleteEventEvictsAllCaches(t *testing.T) {
	op := "upgrade-0.75.0-20260413T120000Z"
	otherOp := "upgrade-0.76.0-20260501T000000Z"
	evDir := t.TempDir()

	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	// Wire a ProvisionerProxy and seed it with the target operation.
	proxy := NewProvisionerProxyWithClient(ProvisionerProxyConfig{Namespace: "solo"}, m.client)
	proxy.completedOpIDs[op] = struct{}{}
	proxy.completedOpIDs[otherOp] = struct{}{}
	m.mu.Lock()
	m.proxy = proxy
	m.mu.Unlock()

	// Track whether onDelete is called with the correct operationID.
	var deletedOpID string
	m.onDelete = func(operationID string) { deletedOpID = operationID }

	// Seed monitor-local state for both operations.
	m.daemonTriggered[op] = struct{}{}
	m.lastPhase[op+"/"+operatorv1alpha1.StepExecute] = "Running"
	m.opFirstSeen[op] = crCreatedAt
	m.events.info(op, "MarkerDetected", "x") // open a writer

	m.daemonTriggered[otherOp] = struct{}{}
	m.lastPhase[otherOp+"/"+operatorv1alpha1.StepExecute] = "Succeeded"
	m.opFirstSeen[otherOp] = crCreatedAt

	// Simulate a Deleted event for the target operation.
	cr := newTestExecuteCR(op, crCreatedAt)
	m.handleDeleteEvent(cr)

	// --- Assert: target operation evicted from monitor maps ---
	m.mu.Lock()
	_, dtPresent := m.daemonTriggered[op]
	_, lpPresent := m.lastPhase[op+"/"+operatorv1alpha1.StepExecute]
	_, fsPresent := m.opFirstSeen[op]
	m.mu.Unlock()
	if dtPresent {
		t.Error("daemonTriggered not evicted for deleted operation")
	}
	if lpPresent {
		t.Error("lastPhase not evicted for deleted operation")
	}
	if fsPresent {
		t.Error("opFirstSeen not evicted for deleted operation")
	}

	// --- Assert: event-log writer closed ---
	m.events.mu.Lock()
	_, writerOpen := m.events.writers[op]
	m.events.mu.Unlock()
	if writerOpen {
		t.Error("event-log handle not released for deleted operation")
	}

	// --- Assert: ProvisionerProxy completedOpIDs evicted ---
	proxy.mu.Lock()
	_, proxyPresent := proxy.completedOpIDs[op]
	proxy.mu.Unlock()
	if proxyPresent {
		t.Error("ProvisionerProxy.completedOpIDs not evicted for deleted operation")
	}

	// --- Assert: onDelete callback invoked (Factory cache invalidation) ---
	if deletedOpID != op {
		t.Errorf("onDelete called with %q, want %q", deletedOpID, op)
	}

	// --- Assert: other operation's state is untouched ---
	m.mu.Lock()
	_, otherDT := m.daemonTriggered[otherOp]
	_, otherLP := m.lastPhase[otherOp+"/"+operatorv1alpha1.StepExecute]
	m.mu.Unlock()
	if !otherDT {
		t.Error("other operation's daemonTriggered wrongly evicted")
	}
	if !otherLP {
		t.Error("other operation's lastPhase wrongly evicted")
	}
	proxy.mu.Lock()
	_, otherProxy := proxy.completedOpIDs[otherOp]
	proxy.mu.Unlock()
	if !otherProxy {
		t.Error("other operation's proxy entry wrongly evicted")
	}

	// --- Assert: JSONL records the deletion as the operation's final audit line ---
	data, err := os.ReadFile(filepath.Join(evDir, op+".jsonl"))
	if err != nil {
		t.Fatalf("read deleted op jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"reason":"CRDeleted"`) {
		t.Errorf("deleted op JSONL missing CRDeleted line:\n%s", data)
	}
}

// TestHandleDeleteEventRecordsDeletionForUntrackedOp verifies deletion is always
// recorded on the JSONL channel — even for an operation the UC never previously
// tracked (no prior writer open). Deletion is a first-class terminal audit event,
// so the op's <operationId>.jsonl is opened, gets the CRDeleted line, and is then
// released by eviction; a same-operationId recreate reopens it in append mode to
// continue the recovery trail.
func TestHandleDeleteEventRecordsDeletionForUntrackedOp(t *testing.T) {
	op := "upgrade-0.75.0-20260413T120000Z"
	evDir := t.TempDir()

	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(evDir, "0")

	// No prior events for op → no writer open. The delete records the event anyway.
	cr := newTestExecuteCR(op, crCreatedAt)
	m.handleDeleteEvent(cr)

	data, err := os.ReadFile(filepath.Join(evDir, op+".jsonl"))
	if err != nil {
		t.Fatalf("read deleted op jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"reason":"CRDeleted"`) {
		t.Errorf("delete did not record a CRDeleted line for an untracked op:\n%s", data)
	}

	// Eviction must release the writer it just opened to record the deletion.
	m.events.mu.Lock()
	_, open := m.events.writers[op]
	m.events.mu.Unlock()
	if open {
		t.Error("event-log handle not released after recording deletion")
	}
}

// TestHandleDeleteEventNoOpWithoutOperationID verifies that a Deleted event for
// a CR with no operationId is silently ignored: it must neither panic nor evict
// any existing cache entry. An unrelated operation is seeded across every cache
// layer first, so a bug that wipes state regardless of operationId would be
// caught — rather than passing vacuously against empty caches.
func TestHandleDeleteEventNoOpWithoutOperationID(t *testing.T) {
	other := "upgrade-0.76.0-20260501T000000Z"

	m := newOutcomeMonitor(t, t.TempDir())
	m.events = newEventSink(t.TempDir(), "0")

	// Wire a proxy and seed the unrelated operation into every cache layer.
	proxy := NewProvisionerProxyWithClient(ProvisionerProxyConfig{Namespace: "solo"}, m.client)
	proxy.completedOpIDs[other] = struct{}{}
	m.mu.Lock()
	m.proxy = proxy
	m.mu.Unlock()

	var onDeleteCalled bool
	m.onDelete = func(string) { onDeleteCalled = true }

	m.daemonTriggered[other] = struct{}{}
	m.lastPhase[other+"/"+operatorv1alpha1.StepExecute] = "Running"
	m.opFirstSeen[other] = crCreatedAt

	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": operatorv1alpha1.GroupVersion.String(),
			"kind":       "NetworkUpgradeExecute",
			"metadata": map[string]interface{}{
				"name":      "no-opid",
				"namespace": "solo",
			},
			"spec": map[string]interface{}{},
		},
	}
	// Must not panic and must not touch any existing state.
	m.handleDeleteEvent(cr)

	m.mu.Lock()
	_, dt := m.daemonTriggered[other]
	_, lp := m.lastPhase[other+"/"+operatorv1alpha1.StepExecute]
	_, fs := m.opFirstSeen[other]
	m.mu.Unlock()
	if !dt || !lp || !fs {
		t.Error("no-opId delete evicted unrelated monitor state (daemonTriggered/lastPhase/opFirstSeen)")
	}
	proxy.mu.Lock()
	_, pp := proxy.completedOpIDs[other]
	proxy.mu.Unlock()
	if !pp {
		t.Error("no-opId delete evicted unrelated ProvisionerProxy state")
	}
	if onDeleteCalled {
		t.Error("no-opId delete invoked onDelete (Factory cache) — should be a no-op")
	}
}

// TestUpgradeOutcomeMonitor_RunRecoversListPanic verifies a panic on the
// non-watch path (listAndSeed) is recovered by Run's top-level defer and returned
// as an error — which the daemonkit supervisor turns into a back-off restart —
// rather than crashing the UC process. A prepended "list" reactor panics on the
// first List the monitor issues.
func TestUpgradeOutcomeMonitor_RunRecoversListPanic(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := fake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		panic("boom in list")
	})
	m := NewUpgradeOutcomeMonitorWithClient(
		UpgradeOutcomeMonitorConfig{Namespace: "solo", UpgradePath: t.TempDir()}, client)
	m.events = newEventSink(t.TempDir(), "0")

	err := m.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run to return a recovered-panic error, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonUpgradeOutcomeMonitorPanic {
		t.Errorf("reason = %q, want %q", reason, ReasonUpgradeOutcomeMonitorPanic)
	}
}

// TestUpgradeOutcomeMonitor_RunWatchRecoversPanic verifies a panic in the watch
// loop is caught by runWatch's own recover and surfaced with the watch-specific
// reason (ProvisionerProxyWatchPanic) — keeping the two recover scopes distinct
// from Run's. A prepended watch reactor panics on the first Watch.
func TestUpgradeOutcomeMonitor_RunWatchRecoversPanic(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := fake.NewSimpleDynamicClient(scheme)
	client.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		panic("boom in watch")
	})
	m := NewUpgradeOutcomeMonitorWithClient(
		UpgradeOutcomeMonitorConfig{Namespace: "solo", UpgradePath: t.TempDir()}, client)
	m.events = newEventSink(t.TempDir(), "0")

	err := m.runWatch(context.Background(), "", "", "", "")
	if err == nil {
		t.Fatal("expected runWatch to return a recovered-panic error, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonProvisionerProxyWatchPanic {
		t.Errorf("reason = %q, want %q", reason, ReasonProvisionerProxyWatchPanic)
	}
}
