// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	adoptLine  = "2026-10-17 12:00:01.123 INFO  105  V0540RosterSchema - Adopting candidate roster in round 1234\n"
	rejectLine = "2026-10-17 12:00:02.456 INFO  110  V0540RosterSchema - Rejecting candidate roster in round 1235\n"
	noneLine   = "2026-10-17 12:00:03.789 INFO  102  V0540RosterSchema - No candidate roster to adopt in round 1236\n"
)

func TestParseRosterAdoptionLog(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 1, 123_000_000, time.UTC)
	cases := []struct {
		name       string
		line       string
		wantOK     bool
		wantResult string
		wantRound  int64
		wantTime   bool
	}{
		{"adopting", adoptLine, true, operatorv1alpha1.RosterAdoptionAdopted, 1234, true},
		{"rejecting", rejectLine, true, operatorv1alpha1.RosterAdoptionRejected, 1235, true},
		{"none", noneLine, true, operatorv1alpha1.RosterAdoptionNone, 1236, true},
		{"no timestamp", "Adopting candidate roster in round 7", true, operatorv1alpha1.RosterAdoptionAdopted, 7, false},
		{"unrelated", "2026-10-17 12:00:01.123 INFO  105  Other - hello 1234\n", false, "", 0, false},
		{"platform status line", `2026-10-17 12:00:01.123 INFO PLATFORM_STATUS {"oldStatus":"A","newStatus":"ACTIVE"}`, false, "", 0, false},
		{"no round", "Adopting candidate roster in round abc", false, "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseRosterAdoptionLog(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Result != tc.wantResult || got.Round != tc.wantRound {
				t.Fatalf("got (%s, %d), want (%s, %d)", got.Result, got.Round, tc.wantResult, tc.wantRound)
			}
			if tc.wantTime && !got.LoggedAt.Equal(at) && tc.name == "adopting" {
				t.Fatalf("LoggedAt = %s, want %s", got.LoggedAt, at)
			}
			if tc.wantTime == got.LoggedAt.IsZero() {
				t.Fatalf("LoggedAt zero = %v, want timestamp present = %v", got.LoggedAt.IsZero(), tc.wantTime)
			}
		})
	}
}

func TestRosterAdoptionConditionPatch(t *testing.T) {
	a := RosterAdoption{Result: operatorv1alpha1.RosterAdoptionRejected, Round: 1235,
		LoggedAt: time.Date(2026, 10, 17, 12, 0, 2, 0, time.UTC)}
	raw := rosterAdoptionConditionPatch(a, "2026-10-17T12:00:05Z")

	var body patchBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("patch is not valid JSON: %v\n%s", err, raw)
	}
	if len(body.Status.Conditions) != 1 {
		t.Fatalf("want exactly one condition (merge by type), got %d", len(body.Status.Conditions))
	}
	c := body.Status.Conditions[0]
	if c.Type != operatorv1alpha1.PodConditionRosterAdoption {
		t.Errorf("type = %q", c.Type)
	}
	if c.Reason != operatorv1alpha1.RosterAdoptionRejected || c.Status != corev1.ConditionTrue {
		t.Errorf("reason/status = %q/%q", c.Reason, c.Status)
	}
	if c.Message == "" {
		t.Error("message must carry the human detail line")
	}
}

func newRosterLogMonitor(t *testing.T, content string) (*LogMonitor, string, string) {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "swirlds.log")
	persist := filepath.Join(dir, "persist")
	mkdirAll(t, persist)
	writeLog(t, logFile, content)
	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = persist
	return NewLogMonitor(cfg), logFile, persist
}

func TestLogMonitor_RosterAdoptionSeedAndTail(t *testing.T) {
	mon, logFile, _ := newRosterLogMonitor(t, "noise\n"+adoptLine)

	if mon.RosterAdoption() != nil {
		t.Fatal("no result expected before the log is read")
	}
	mon.seedFromTail()
	got := mon.RosterAdoption()
	if got == nil || got.Result != operatorv1alpha1.RosterAdoptionAdopted || got.Round != 1234 {
		t.Fatalf("after seed: %+v, want Adopted round 1234", got)
	}

	// A later result replaces it.
	appendLog(t, logFile, rejectLine)
	mon.poll()
	got = mon.RosterAdoption()
	if got == nil || got.Result != operatorv1alpha1.RosterAdoptionRejected || got.Round != 1235 {
		t.Fatalf("after poll: %+v, want Rejected round 1235", got)
	}
}

func TestLogMonitor_RosterAdoptionDedupesRereads(t *testing.T) {
	mon, _, _ := newRosterLogMonitor(t, adoptLine)
	mon.seedFromTail()
	first := mon.RosterAdoption()

	// Seeding again (a restart reading the same tail) must keep the same object, so
	// nothing downstream re-warns or re-patches.
	mon.seedFromTail()
	if mon.RosterAdoption() != first {
		t.Fatal("re-reading the same line replaced the held result")
	}
}

func TestLogMonitor_RosterAdoptionAbsentIsUnknown(t *testing.T) {
	mon, _, _ := newRosterLogMonitor(t, "just some other log output\n")
	mon.seedFromTail()
	if mon.RosterAdoption() != nil {
		t.Fatal("no line means unknown, never an adoption result")
	}
}

func TestLogMonitor_RosterAdoptionPersistsAcrossRestart(t *testing.T) {
	mon, logFile, persist := newRosterLogMonitor(t, rejectLine)
	mon.seedFromTail()

	// A new monitor over an emptied log still knows the last result.
	writeLog(t, logFile, "rolled over\n")
	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = persist
	restarted := NewLogMonitor(cfg)
	restarted.loadPersisted()
	got := restarted.RosterAdoption()
	if got == nil || got.Result != operatorv1alpha1.RosterAdoptionRejected || got.Round != 1235 {
		t.Fatalf("restored: %+v, want Rejected round 1235", got)
	}
}

func TestLogMonitor_LoadsStateFileWithoutRosterField(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.CRPersistPath = dir
	cfg.ConsensusLogFile = filepath.Join(dir, "none.log")
	// A state file written before the roster field existed.
	writeLog(t, filepath.Join(dir, logPersistFile), `{"code":2,"status":"ACTIVE"}`)
	mon := NewLogMonitor(cfg)
	mon.loadPersisted()
	if snap := mon.Snapshot(); snap == nil || snap.Status != "ACTIVE" {
		t.Fatalf("platform status not restored: %+v", snap)
	}
	if mon.RosterAdoption() != nil {
		t.Fatal("no roster result expected from an old state file")
	}
}

func TestPlatformStatusMonitor_PatchesRosterAdoptionConditionOncePerResult(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleDynamicClient(scheme)

	var (
		mu      sync.Mutex
		patches []k8stesting.PatchAction
	)
	client.PrependReactor("patch", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		patches = append(patches, a.(k8stesting.PatchAction))
		mu.Unlock()
		return true, &unstructured.Unstructured{Object: map[string]interface{}{}}, nil
	})
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(patches) }

	logMon, logFile, _ := newRosterLogMonitor(t, adoptLine)
	logMon.seedFromTail()

	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	m := NewPlatformStatusMonitor(cfg, client, nil, logMon)
	ctx := context.Background()

	m.syncRosterAdoption(ctx)
	if count() != 1 {
		t.Fatalf("want 1 patch for the first result, got %d", count())
	}
	pa := patches[0]
	if pa.GetPatchType() != types.StrategicMergePatchType || pa.GetSubresource() != "status" {
		t.Errorf("patch type/subresource = %q/%q, want strategic merge on status", pa.GetPatchType(), pa.GetSubresource())
	}

	// Unchanged result: no write.
	m.syncRosterAdoption(ctx)
	if count() != 1 {
		t.Fatalf("an unchanged result must not patch again, got %d", count())
	}

	// A new result: one more write.
	appendLog(t, logFile, rejectLine)
	logMon.poll()
	m.syncRosterAdoption(ctx)
	if count() != 2 {
		t.Fatalf("a new result must patch once, got %d", count())
	}
}

func TestPlatformStatusMonitor_RosterAdoptionPatchRetriedAfterFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleDynamicClient(scheme)

	var mu sync.Mutex
	calls, fail := 0, true
	client.PrependReactor("patch", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if fail {
			return true, nil, context.DeadlineExceeded
		}
		return true, &unstructured.Unstructured{Object: map[string]interface{}{}}, nil
	})

	logMon, _, _ := newRosterLogMonitor(t, rejectLine)
	logMon.seedFromTail()
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	m := NewPlatformStatusMonitor(cfg, client, nil, logMon)

	m.syncRosterAdoption(context.Background()) // fails
	mu.Lock()
	fail = false
	mu.Unlock()
	m.syncRosterAdoption(context.Background()) // retried
	m.syncRosterAdoption(context.Background()) // now unchanged

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("want 2 attempts (one failure, one retry), got %d", calls)
	}
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}
