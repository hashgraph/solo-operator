// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandlerEmitsMarkerDetectedAndCRDeliveredJSONL runs a real marker handler
// and verifies both #1001 events land in the operation's JSONL, once each even
// across repeated (idempotent) ticks.
func TestHandlerEmitsMarkerDetectedAndCRDeliveredJSONL(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	evDir := t.TempDir()
	f := newFakeFactory(t, testConfig(root, t.TempDir()))
	f.events = newEventSink(evDir, "0")

	for i := 0; i < 3; i++ { // ticks 2 and 3 are idempotent no-ops
		if err := f.HandleExecuteImmediate(context.Background()); err != nil {
			t.Fatalf("HandleExecuteImmediate tick %d: %v", i, err)
		}
	}
	f.events.Close()

	data, err := os.ReadFile(filepath.Join(evDir, wantOperationID+".jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	got := string(data)
	if c := strings.Count(got, `"reason":"MarkerDetected"`); c != 1 {
		t.Errorf("MarkerDetected count = %d, want 1\n%s", c, got)
	}
	if c := strings.Count(got, `"reason":"CRDelivered"`); c != 1 {
		t.Errorf("CRDelivered count = %d, want 1\n%s", c, got)
	}
}

// TestEventSinkWritesPerOperationJSONL verifies the sink opens one file per
// operationId, appends INFO/ERROR lines, and reuses the cached writer.
func TestEventSinkWritesPerOperationJSONL(t *testing.T) {
	dir := t.TempDir()
	s := newEventSink(dir, "0")

	s.info("upgrade-20260415T143000Z", "CRDelivered", "created NetworkUpgradePrepare/x")
	s.info("upgrade-20260415T143000Z", "MarkerDetected", "detected execute_immediate.mf")
	s.error("upgrade-20260415T143000Z", "ManifestInvalid", "bad schema")
	s.Close()

	data, err := os.ReadFile(filepath.Join(dir, "upgrade-20260415T143000Z.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	got := string(data)
	for _, want := range []string{`"reason":"CRDelivered"`, `"reason":"MarkerDetected"`, `"reason":"ManifestInvalid"`, `"level":"ERROR"`, `"nodeId":"0"`} {
		if !strings.Contains(got, want) {
			t.Errorf("jsonl missing %q; got:\n%s", want, got)
		}
	}
	if n := strings.Count(strings.TrimSpace(got), "\n"); n != 2 { // 3 lines => 2 newlines between
		t.Errorf("expected 3 JSONL lines, got %d newlines:\n%s", n, got)
	}
}

// TestEventSinkDisableStopsFurtherWrites verifies that once an operation's log is
// disabled (as happens after the first write failure) its writer is dropped,
// it is marked failed, and subsequent writes no-op instead of reopening.
func TestEventSinkDisableStopsFurtherWrites(t *testing.T) {
	dir := t.TempDir()
	s := newEventSink(dir, "0")
	op := "upgrade-20260415T143000Z"

	s.info(op, "MarkerDetected", "detected")
	s.disable(op)

	s.mu.Lock()
	_, hasWriter := s.writers[op]
	_, failed := s.failed[op]
	s.mu.Unlock()
	if hasWriter {
		t.Error("writer not dropped after disable")
	}
	if !failed {
		t.Error("operation not marked failed after disable")
	}
	if w := s.writer(op); w != nil {
		t.Error("writer() should return nil for a disabled operation")
	}

	s.info(op, "CRDelivered", "created") // must no-op, not reopen
	s.Close()

	data, err := os.ReadFile(filepath.Join(dir, op+".jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if c := strings.Count(string(data), "reason"); c != 1 {
		t.Fatalf("expected only the pre-disable line, got %d lines:\n%s", c, data)
	}
}

// TestEventSinkNilAndDisabledAreNoops verifies best-effort semantics: a nil sink,
// an empty-dir sink, and an empty operationId never panic and never write.
func TestEventSinkNilAndDisabledAreNoops(t *testing.T) {
	var nilSink *eventSink
	nilSink.info("op", "R", "m")
	nilSink.error("op", "R", "m")
	nilSink.Close()

	dir := t.TempDir()
	disabled := newEventSink("", "0") // no dir → disabled
	disabled.info("op", "R", "m")
	empty := newEventSink(dir, "0")
	empty.info("", "R", "m") // empty operationId → no file
	empty.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files written, got %d", len(entries))
	}
}
