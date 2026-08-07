// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestParsePlatformStatusLog(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		wantCode int32
		wantName string
		wantOK   bool
	}{
		{
			"standard transition",
			`2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> StatusStateMachine - Platform spent 5s in STARTING_UP. {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}`,
			2, "ACTIVE", true,
		},
		{
			"freeze complete",
			`2024-07-01 00:01:00.000  123 INFO  PLATFORM_STATUS <main> StatusStateMachine - transition {"oldStatus":"FREEZING","newStatus":"FREEZE_COMPLETE"}`,
			6, "FREEZE_COMPLETE", true,
		},
		{
			"catastrophic failure",
			`2024-07-01 00:02:00.000  123 INFO  PLATFORM_STATUS <main> StatusStateMachine - {"oldStatus":"ACTIVE","newStatus":"CATASTROPHIC_FAILURE"}`,
			11, "CATASTROPHIC_FAILURE", true,
		},
		{
			"no PLATFORM_STATUS marker",
			`2024-07-01 00:00:00.000  123 INFO  OTHER_MARKER <main> Something {"oldStatus":"A","newStatus":"B"}`,
			0, "", false,
		},
		{
			"no JSON",
			`2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> no json here`,
			0, "", false,
		},
		{
			"empty newStatus",
			`2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> {"oldStatus":"ACTIVE","newStatus":""}`,
			0, "", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, name, ok := parsePlatformStatusLog(tc.line)
			if ok != tc.wantOK || code != tc.wantCode || name != tc.wantName {
				t.Errorf("parsePlatformStatusLog = (%d, %q, %v), want (%d, %q, %v)", code, name, ok, tc.wantCode, tc.wantName, tc.wantOK)
			}
		})
	}
}

func TestLogMonitor_TailAndSticky(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "swirlds.log")
	persistDir := filepath.Join(dir, "persist")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write initial log with a PLATFORM_STATUS transition.
	writeLog(t, logFile, `2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> StatusStateMachine - transition {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}
`)

	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = persistDir
	mon := NewLogMonitor(cfg)

	// Seed from tail picks up ACTIVE.
	mon.seedFromTail()
	snap := mon.Snapshot()
	if snap == nil {
		t.Fatal("expected snapshot after seed")
	}
	if snap.Code != 2 || snap.Status != "ACTIVE" {
		t.Errorf("snapshot = (%d, %q), want (2, ACTIVE)", snap.Code, snap.Status)
	}

	// Poll with no new data — snapshot stays.
	mon.poll()
	snap = mon.Snapshot()
	if snap.Code != 2 {
		t.Errorf("sticky: code = %d, want 2", snap.Code)
	}

	// Append another transition.
	appendLog(t, logFile, `2024-07-01 00:01:00.000  123 INFO  PLATFORM_STATUS <main> StatusStateMachine - transition {"oldStatus":"ACTIVE","newStatus":"FREEZING"}
`)
	mon.poll()
	snap = mon.Snapshot()
	if snap == nil || snap.Code != 5 || snap.Status != "FREEZING" {
		t.Errorf("after FREEZING: snapshot = %+v, want code=5 status=FREEZING", snap)
	}

	// Mtime check.
	mtime := mon.LogFileMtime()
	if mtime == 0 {
		t.Error("mtime = 0, expected non-zero for existing file")
	}
}

func TestLogMonitor_Rotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "swirlds.log")
	persistDir := filepath.Join(dir, "persist")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeLog(t, logFile, `2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> transition {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}
some other line
some other line
some other line
`)

	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = persistDir
	mon := NewLogMonitor(cfg)
	mon.seedFromTail()

	if snap := mon.Snapshot(); snap == nil || snap.Code != 2 {
		t.Fatalf("pre-rotation: expected ACTIVE, got %+v", snap)
	}

	// Simulate rotation: file is replaced with smaller content.
	writeLog(t, logFile, `2024-07-01 00:02:00.000  456 INFO  PLATFORM_STATUS <main> transition {"oldStatus":"ACTIVE","newStatus":"BEHIND"}
`)
	mon.poll()

	snap := mon.Snapshot()
	if snap == nil || snap.Code != 4 || snap.Status != "BEHIND" {
		t.Errorf("post-rotation: snapshot = %+v, want code=4 status=BEHIND", snap)
	}
}

func TestLogMonitor_Persistence(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "swirlds.log")
	persistDir := filepath.Join(dir, "persist")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeLog(t, logFile, `2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> transition {"oldStatus":"STARTING_UP","newStatus":"FREEZE_COMPLETE"}
`)

	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = persistDir
	mon := NewLogMonitor(cfg)
	mon.seedFromTail()

	if snap := mon.Snapshot(); snap == nil || snap.Code != 6 {
		t.Fatalf("expected FREEZE_COMPLETE, got %+v", snap)
	}

	// New monitor restores from persist.
	mon2 := NewLogMonitor(cfg)
	mon2.loadPersisted()
	snap := mon2.Snapshot()
	if snap == nil || snap.Code != 6 || snap.Status != "FREEZE_COMPLETE" {
		t.Errorf("after restart: snapshot = %+v, want code=6 status=FREEZE_COMPLETE", snap)
	}
}

func TestLogMonitor_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ConsensusLogFile = filepath.Join(dir, "nonexistent.log")
	cfg.CRPersistPath = dir
	mon := NewLogMonitor(cfg)

	mon.seedFromTail()
	if snap := mon.Snapshot(); snap != nil {
		t.Errorf("expected nil snapshot when file absent, got %+v", snap)
	}
	if mon.ConnectivityError() == nil {
		t.Error("expected connectivity error when file absent")
	}

	mtime := mon.LogFileMtime()
	if mtime != 0 {
		t.Errorf("mtime = %v, want 0 for absent file", mtime)
	}
}

func TestConsensusStatusCollector_LogSource(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "swirlds.log")
	writeLog(t, logFile, `2024-07-01 00:00:00.000  123 INFO  PLATFORM_STATUS <main> transition {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}
`)

	cfg := DefaultConfig()
	cfg.ConsensusLogFile = logFile
	cfg.CRPersistPath = dir
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	logMon := NewLogMonitor(cfg)
	logMon.seedFromTail()

	platMon := NewPlatformStatusMonitor(cfg, nil, nil, nil)
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewConsensusStatusCollector(0, DefaultMetricsStaleness, platMon, logMon))

	f := gatherFamily(t, reg, "uc_consensus_status")
	if f == nil {
		t.Fatal("uc_consensus_status not found")
	}
	m := metricByLabels(t, f, map[string]string{"source": "log", "node_id": "0"})
	if m == nil {
		t.Fatal("source=log not found")
	}
	if got := m.GetGauge().GetValue(); got != 2 {
		t.Errorf("source=log value = %v, want 2 (ACTIVE)", got)
	}

	logTS := gatherFamily(t, reg, "uc_consensus_log_timestamp")
	if logTS == nil {
		t.Fatal("uc_consensus_log_timestamp not found")
	}
	lm := metricByLabels(t, logTS, map[string]string{"node_id": "0"})
	if lm == nil {
		t.Fatal("log timestamp series not found")
	}
	if got := lm.GetGauge().GetValue(); got == 0 {
		t.Error("log timestamp = 0, expected non-zero for existing file")
	}
}

func TestConsensusStatusCollector_LogSource_NoMonitor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	platMon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewConsensusStatusCollector(0, DefaultMetricsStaleness, platMon, nil))

	f := gatherFamily(t, reg, "uc_consensus_status")
	m := metricByLabels(t, f, map[string]string{"source": "log"})
	if m == nil {
		t.Fatal("source=log not found")
	}
	if got := m.GetGauge().GetValue(); got != float64(operatorv1alpha1.PlatformStatusCodeUnknown) {
		t.Errorf("source=log value = %v, want -1 (UNKNOWN when no log monitor)", got)
	}
}

func writeLog(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure mtime is current.
	now := time.Now()
	_ = os.Chtimes(path, now, now)
}

func appendLog(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}
