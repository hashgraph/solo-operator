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

	// Create RetainCount+5 upgrade logs with ascending embedded timestamps.
	const extra = 5
	var names []string
	for i := 0; i < RetainCount+extra; i++ {
		ts := time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(sanity.TimestampLayout)
		name := fmt.Sprintf("upgrade-%s.jsonl", ts)
		names = append(names, name)
		writeFile(t, filepath.Join(dir, name))
	}
	// A non-upgrade file and a file with no timestamp must be left untouched.
	writeFile(t, filepath.Join(dir, "migration-20260101T000000Z.jsonl"))
	writeFile(t, filepath.Join(dir, "upgrade-notimestamp.jsonl"))

	if err := Prune(dir); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	// The oldest `extra` upgrade-with-timestamp files should be gone; the newest
	// RetainCount kept, plus the protected files.
	for i, name := range names {
		_, err := os.Stat(filepath.Join(dir, name))
		if i < extra && !os.IsNotExist(err) {
			t.Errorf("%s should have been pruned", name)
		}
		if i >= extra && err != nil {
			t.Errorf("%s should have been retained: %v", name, err)
		}
	}
	mustExist(t, filepath.Join(dir, "migration-20260101T000000Z.jsonl"))
	mustExist(t, filepath.Join(dir, "upgrade-notimestamp.jsonl"))
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
