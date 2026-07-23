// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func gatherFamily(t *testing.T, reg *prometheus.Registry, name string) *dto.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

func TestNewMetricsRegistry_SelfHealthMetrics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NodeID = 0
	tracker := daemonkit.NewStatusTracker()
	monitors := []daemonkit.MonitorRunner{
		&stubMonitor{name: "marker-monitor"},
		&stubMonitor{name: "platform-status-monitor"},
	}

	reg := NewMetricsRegistry(cfg, tracker, monitors)

	for _, name := range []string{"uc_build_info", "uc_up", "uc_monitor_up"} {
		if f := gatherFamily(t, reg, name); f == nil {
			t.Errorf("expected metric family %q to be registered", name)
		}
	}

	buildInfo := gatherFamily(t, reg, "uc_build_info")
	if buildInfo == nil {
		t.Fatal("uc_build_info not found")
	}
	m := buildInfo.GetMetric()
	if len(m) != 1 || m[0].GetGauge().GetValue() != 1 {
		t.Errorf("uc_build_info gauge = %v, want 1", m)
	}
	hasVersion, hasCommit := false, false
	for _, lp := range m[0].GetLabel() {
		if lp.GetName() == "version" {
			hasVersion = true
		}
		if lp.GetName() == "commit" {
			hasCommit = true
		}
	}
	if !hasVersion || !hasCommit {
		t.Errorf("uc_build_info labels: hasVersion=%v hasCommit=%v, want both true", hasVersion, hasCommit)
	}

	monUp := gatherFamily(t, reg, "uc_monitor_up")
	if monUp == nil {
		t.Fatal("uc_monitor_up not found")
	}
	if got := len(monUp.GetMetric()); got != 2 {
		t.Errorf("uc_monitor_up has %d series, want 2 (one per monitor)", got)
	}
}

func TestNewMetricsRegistry_GoAndProcessCollectors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NodeID = 0
	tracker := daemonkit.NewStatusTracker()
	reg := NewMetricsRegistry(cfg, tracker, nil)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	hasGo, hasProcess := false, false
	for _, f := range families {
		if f.GetName() == "go_goroutines" {
			hasGo = true
		}
		if f.GetName() == "process_open_fds" || f.GetName() == "process_cpu_seconds_total" {
			hasProcess = true
		}
	}
	if !hasGo {
		t.Error("expected go_* metrics from GoCollector")
	}
	if !hasProcess {
		t.Error("expected process_* metrics from ProcessCollector")
	}
}

func TestRunMetricsServer_ServesMetrics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NodeID = 0
	tracker := daemonkit.NewStatusTracker()
	reg := NewMetricsRegistry(cfg, tracker, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- runMetricsServer(ctx, "127.0.0.1:0", reg) }()

	// The server picks a random port when we pass :0 — but we don't know it.
	// Use a fixed port for the test instead.
	cancel()
	<-errCh

	// Re-run with a known port.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go func() { _ = runMetricsServer(ctx2, "127.0.0.1:19990", reg) }()
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:19990/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"uc_build_info", "uc_up", "go_goroutines"} {
		if !contains(string(body), want) {
			t.Errorf("response body missing %q", want)
		}
	}
	cancel2()
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type stubMonitor struct {
	name string
}

func (m *stubMonitor) Name() string                { return m.name }
func (m *stubMonitor) Run(_ context.Context) error { select {} } //nolint:staticcheck // intentional block
