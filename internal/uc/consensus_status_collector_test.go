// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func metricByLabels(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()
	for _, m := range family.GetMetric() {
		match := true
		for wantName, wantVal := range labels {
			found := false
			for _, lp := range m.GetLabel() {
				if lp.GetName() == wantName && lp.GetValue() == wantVal {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			return m
		}
	}
	return nil
}

func TestConsensusStatusCollector_MetricsSource_NoSnapshot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, nil))

	f := gatherFamily(t, reg, "uc_consensus_status")
	if f == nil {
		t.Fatal("uc_consensus_status not found")
	}
	m := metricByLabels(t, f, map[string]string{"source": "metrics", "node_id": "0"})
	if m == nil {
		t.Fatal("source=metrics series not found")
	}
	if got := m.GetGauge().GetValue(); got != float64(operatorv1alpha1.PlatformStatusCodeUnknown) {
		t.Errorf("value = %v, want %v (UNKNOWN when no snapshot)", got, operatorv1alpha1.PlatformStatusCodeUnknown)
	}

	scrape := gatherFamily(t, reg, "uc_platform_last_scrape_timestamp")
	if scrape == nil {
		t.Fatal("uc_platform_last_scrape_timestamp not found")
	}
	sm := metricByLabels(t, scrape, map[string]string{"node_id": "0"})
	if sm == nil {
		t.Fatal("scrape timestamp series not found")
	}
	if got := sm.GetGauge().GetValue(); got != 0 {
		t.Errorf("scrape timestamp = %v, want 0 (no successful scrape)", got)
	}
}

func TestConsensusStatusCollector_MetricsSource_WithSnapshot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	now := time.Now()
	mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: 2, Status: "ACTIVE", ScrapedAt: now})

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, nil))

	f := gatherFamily(t, reg, "uc_consensus_status")
	if f == nil {
		t.Fatal("uc_consensus_status not found")
	}
	m := metricByLabels(t, f, map[string]string{"source": "metrics", "node_id": "0"})
	if m == nil {
		t.Fatal("source=metrics series not found")
	}
	if got := m.GetGauge().GetValue(); got != 2 {
		t.Errorf("value = %v, want 2 (ACTIVE)", got)
	}

	scrape := gatherFamily(t, reg, "uc_platform_last_scrape_timestamp")
	sm := metricByLabels(t, scrape, map[string]string{"node_id": "0"})
	if sm == nil {
		t.Fatal("scrape timestamp series not found")
	}
	if got := sm.GetGauge().GetValue(); got != float64(now.Unix()) {
		t.Errorf("scrape timestamp = %v, want %v", got, now.Unix())
	}
}

func TestConsensusStatusCollector_MetricsSource_AllCodes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	codes := []int32{1, 2, 4, 5, 6, 7, 8, 9, 10, 11}
	for _, code := range codes {
		mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{
			Code:      code,
			Status:    nameForCode(code),
			ScrapedAt: time.Now(),
		})

		reg := prometheus.NewRegistry()
		reg.MustRegister(NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, nil))
		f := gatherFamily(t, reg, "uc_consensus_status")
		m := metricByLabels(t, f, map[string]string{"source": "metrics"})
		if m == nil {
			t.Fatalf("source=metrics not found for code %d", code)
		}
		if got := m.GetGauge().GetValue(); got != float64(code) {
			t.Errorf("code %d: value = %v, want %v", code, got, float64(code))
		}
	}
}

func TestConsensusStatusCollector_Effective_FreshMetrics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)
	mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: 2, Status: "ACTIVE", ScrapedAt: time.Now()})

	logMon := NewLogMonitor(cfg)
	logMon.snapshot.Store(&LogStatusSnapshot{Code: 5, Status: "FREEZING"})

	coll := NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, logMon)
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	f := gatherFamily(t, reg, "uc_consensus_status")
	m := metricByLabels(t, f, map[string]string{"source": "effective"})
	if m == nil {
		t.Fatal("source=effective not found")
	}
	if got := m.GetGauge().GetValue(); got != 2 {
		t.Errorf("effective = %v, want 2 (ACTIVE, metrics is fresh)", got)
	}
}

func TestConsensusStatusCollector_Effective_StaleMetricsFallsBackToLog(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)
	mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: 2, Status: "ACTIVE", ScrapedAt: time.Now().Add(-30 * time.Second)})

	logMon := NewLogMonitor(cfg)
	logMon.snapshot.Store(&LogStatusSnapshot{Code: 6, Status: "FREEZE_COMPLETE"})

	coll := NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, logMon)
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	f := gatherFamily(t, reg, "uc_consensus_status")
	m := metricByLabels(t, f, map[string]string{"source": "effective"})
	if m == nil {
		t.Fatal("source=effective not found")
	}
	if got := m.GetGauge().GetValue(); got != 6 {
		t.Errorf("effective = %v, want 6 (FREEZE_COMPLETE, metrics stale → log)", got)
	}
}

func TestConsensusStatusCollector_Effective_NeitherKnown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	coll := NewConsensusStatusCollector(0, DefaultMetricsStaleness, mon, nil)
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	f := gatherFamily(t, reg, "uc_consensus_status")
	m := metricByLabels(t, f, map[string]string{"source": "effective"})
	if m == nil {
		t.Fatal("source=effective not found")
	}
	if got := m.GetGauge().GetValue(); got != float64(operatorv1alpha1.PlatformStatusCodeUnknown) {
		t.Errorf("effective = %v, want -1 (UNKNOWN, nothing available)", got)
	}
}

func TestConsensusStatusCollector_Effective_StalenessEdge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	mon := NewPlatformStatusMonitor(cfg, nil, nil, nil)

	staleness := 15 * time.Second
	fixedNow := time.Date(2026, 7, 1, 0, 0, 30, 0, time.UTC)

	// Just within staleness: scrape at T-14s is fresh.
	mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: 2, Status: "ACTIVE", ScrapedAt: fixedNow.Add(-14 * time.Second)})
	coll := NewConsensusStatusCollector(0, staleness, mon, nil)
	coll.now = func() time.Time { return fixedNow }
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	f := gatherFamily(t, reg, "uc_consensus_status")
	m := metricByLabels(t, f, map[string]string{"source": "effective"})
	if got := m.GetGauge().GetValue(); got != 2 {
		t.Errorf("at -14s (within): effective = %v, want 2", got)
	}

	// Exactly at threshold: scrape at T-15s is stale (>= is not <).
	mon.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: 2, Status: "ACTIVE", ScrapedAt: fixedNow.Add(-15 * time.Second)})
	coll2 := NewConsensusStatusCollector(0, staleness, mon, nil)
	coll2.now = func() time.Time { return fixedNow }
	reg2 := prometheus.NewRegistry()
	reg2.MustRegister(coll2)

	f2 := gatherFamily(t, reg2, "uc_consensus_status")
	m2 := metricByLabels(t, f2, map[string]string{"source": "effective"})
	if got := m2.GetGauge().GetValue(); got != float64(operatorv1alpha1.PlatformStatusCodeUnknown) {
		t.Errorf("at -15s (boundary): effective = %v, want -1 (stale)", got)
	}
}
