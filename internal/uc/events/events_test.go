// SPDX-License-Identifier: Apache-2.0

package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashgraph/solo-operator/internal/sanity"
)

func TestWriterWritesBaseFields(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "upgrade-20260415T143000Z", "3")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()

	if got := filepath.Base(w.Path()); got != "upgrade-20260415T143000Z.jsonl" {
		t.Fatalf("filename = %q, want upgrade-20260415T143000Z.jsonl", got)
	}

	if err := w.Info("FreezeScheduled", "freeze marker observed"); err != nil {
		t.Fatalf("Info: %v", err)
	}
	if err := w.Error("UpgradeFailed", "package missing"); err != nil {
		t.Fatalf("Error: %v", err)
	}

	lines := readJSONL(t, w.Path())
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	first := lines[0]
	for _, field := range []string{"ts", "level", "reason", "msg", "operationId", "nodeId"} {
		if _, ok := first[field]; !ok {
			t.Errorf("first line missing base field %q: %v", field, first)
		}
	}
	if first["level"] != string(LevelInfo) {
		t.Errorf("level = %v, want INFO", first["level"])
	}
	if first["operationId"] != "upgrade-20260415T143000Z" {
		t.Errorf("operationId = %v", first["operationId"])
	}
	if first["nodeId"] != "3" {
		t.Errorf("nodeId = %v, want 3", first["nodeId"])
	}
	if first["reason"] != "FreezeScheduled" {
		t.Errorf("reason = %v", first["reason"])
	}
	if _, err := time.Parse(time.RFC3339Nano, first["ts"].(string)); err != nil {
		t.Errorf("ts not RFC3339: %v", first["ts"])
	}
	if lines[1]["level"] != string(LevelError) {
		t.Errorf("second line level = %v, want ERROR", lines[1]["level"])
	}
}

func TestWriterAppends(t *testing.T) {
	dir := t.TempDir()
	id := "upgrade-20260415T143000Z"

	w1, err := Open(dir, id, "0")
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if err := w1.Info("First", "one"); err != nil {
		t.Fatal(err)
	}
	w1.Close()

	// Reopening the same operation (e.g. after a UC restart) must append, not truncate.
	w2, err := Open(dir, id, "0")
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if err := w2.Info("Second", "two"); err != nil {
		t.Fatal(err)
	}
	w2.Close()

	if n := len(readJSONL(t, w2.Path())); n != 2 {
		t.Fatalf("got %d lines, want 2 (append across reopen)", n)
	}
}

func TestOpenValidatesInput(t *testing.T) {
	dir := t.TempDir()
	// Unsafe operation ID (shell metacharacters) is rejected by sanity validation.
	if _, err := Open(dir, "upgrade-$(rm)", "0"); err == nil {
		t.Fatal("expected error for unsafe operation ID")
	}
	// Empty nodeID is rejected.
	if _, err := Open(dir, "upgrade-20260415T143000Z", ""); err == nil {
		t.Fatal("expected error for empty nodeID")
	}
	// A safe id without a timestamp is allowed (it only warns; retention skips it).
	w, err := Open(dir, "upgrade-latest", "0")
	if err != nil {
		t.Fatalf("timestampless id should be allowed: %v", err)
	}
	w.Close()
}

func TestPruneRetainsLastN(t *testing.T) {
	dir := t.TempDir()

	// Create RetainCount+extra upgrade logs with ascending embedded timestamps.
	const extra = 5
	var names []string
	for i := 0; i < RetainCount+extra; i++ {
		ts := time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(sanity.TimestampLayout)
		name := fmt.Sprintf("upgrade-%s.jsonl", ts)
		names = append(names, name)
		writeFile(t, filepath.Join(dir, name))
	}
	// A non-.jsonl file is outside the retention glob entirely and must be untouched.
	writeFile(t, filepath.Join(dir, "notes.txt"))

	if err := Prune(dir); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	// The oldest `extra` timestamped files should be gone; the newest RetainCount kept.
	for i, name := range names {
		_, err := os.Stat(filepath.Join(dir, name))
		if i < extra && !os.IsNotExist(err) {
			t.Errorf("%s should have been pruned", name)
		}
		if i >= extra && err != nil {
			t.Errorf("%s should have been retained: %v", name, err)
		}
	}
	mustExist(t, filepath.Join(dir, "notes.txt"))
}

func TestPruneMtimeFallbackForTimestamplessFiles(t *testing.T) {
	dir := t.TempDir()

	// Two event logs whose names carry no parseable timestamp, so retention falls
	// back to mtime: one stale (beyond the max-age window) and one fresh.
	stale := filepath.Join(dir, "upgrade-notimestamp-old.jsonl")
	fresh := filepath.Join(dir, "upgrade-notimestamp-new.jsonl")
	writeFile(t, stale)
	writeFile(t, fresh)

	old := time.Now().Add(-2 * retainMaxAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := Prune(dir); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale timestampless log should have been pruned by mtime fallback")
	}
	mustExist(t, fresh)
}

func readJSONL(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}\n"), 0o640); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}
