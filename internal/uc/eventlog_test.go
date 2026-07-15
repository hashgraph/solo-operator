// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
