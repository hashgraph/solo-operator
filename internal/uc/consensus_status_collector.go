// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ConsensusStatusView is a point-in-time snapshot of consensus status from all
// sources, including the reconciled effective view. It is shared by the
// Prometheus collector and the control-plane /consensus/status handler.
type ConsensusStatusView struct {
	NodeID    int             `json:"nodeId"`
	Effective EffectiveStatus `json:"effective"`
	Metrics   MetricsStatus   `json:"metrics"`
	Log       LogStatus       `json:"log"`
	Staleness time.Duration   `json:"-"`
}

// EffectiveStatus is the reconciled consensus status.
type EffectiveStatus struct {
	Code   int32  `json:"code"`
	Status string `json:"status"`
	Source string `json:"source"`
}

// MetricsStatus is the platform status from the metrics scrape.
type MetricsStatus struct {
	Code      int32      `json:"code"`
	Status    string     `json:"status"`
	ScrapedAt *time.Time `json:"scrapedAt,omitempty"`
	Fresh     bool       `json:"fresh"`
}

// LogStatus is the platform status from the consensus log.
type LogStatus struct {
	Code           int32      `json:"code"`
	Status         string     `json:"status"`
	FileModifiedAt *time.Time `json:"fileModifiedAt,omitempty"`
}

// reconcileConsensusStatus reads the current monitor state and produces a
// unified view. Both the Prometheus collector and the control-plane handler
// call this so the effective-source logic lives in one place.
func reconcileConsensusStatus(nodeID int, staleness time.Duration, now time.Time, platform *PlatformStatusMonitor, logMon *LogMonitor) ConsensusStatusView {
	unknown := operatorv1alpha1.PlatformStatusCodeUnknown

	metricsCode := unknown
	metricsStatus := operatorv1alpha1.PlatformStatusNameForCode(unknown)
	var scrapedAt *time.Time
	metricsFresh := false
	if snap := platform.ScrapeSnapshot(); snap != nil {
		metricsCode = snap.Code
		metricsStatus = snap.Status
		t := snap.ScrapedAt
		scrapedAt = &t
		metricsFresh = now.Sub(snap.ScrapedAt) < staleness
	}

	logCode := unknown
	logStatus := operatorv1alpha1.PlatformStatusNameForCode(unknown)
	var logModifiedAt *time.Time
	if logMon != nil {
		if ls := logMon.Snapshot(); ls != nil {
			logCode = ls.Code
			logStatus = ls.Status
		}
		if mtime := logMon.LogFileMtime(); mtime > 0 {
			t := time.Unix(int64(mtime), 0).UTC()
			logModifiedAt = &t
		}
	}

	effectiveCode := unknown
	source := "none"
	if metricsFresh && metricsCode != unknown {
		effectiveCode = metricsCode
		source = "metrics"
	} else if logCode != unknown {
		effectiveCode = logCode
		source = "log"
	}

	return ConsensusStatusView{
		NodeID: nodeID,
		Effective: EffectiveStatus{
			Code:   effectiveCode,
			Status: operatorv1alpha1.PlatformStatusNameForCode(effectiveCode),
			Source: source,
		},
		Metrics: MetricsStatus{
			Code:      metricsCode,
			Status:    metricsStatus,
			ScrapedAt: scrapedAt,
			Fresh:     metricsFresh,
		},
		Log: LogStatus{
			Code:           logCode,
			Status:         logStatus,
			FileModifiedAt: logModifiedAt,
		},
		Staleness: staleness,
	}
}

// ConsensusStatusCollector is a custom prometheus.Collector that emits
// uc_consensus_status{source,node_id} at scrape time by reading the current
// monitor state atomically. This avoids races between the monitor goroutine
// (which writes) and the HTTP scrape goroutine (which reads).
type ConsensusStatusCollector struct {
	nodeID     string
	nodeIDInt  int
	staleness  time.Duration
	monitor    *PlatformStatusMonitor
	logMonitor *LogMonitor
	now        func() time.Time

	statusDesc *prometheus.Desc
	scrapeDesc *prometheus.Desc
	logTSDesc  *prometheus.Desc
}

// NewConsensusStatusCollector creates a collector for uc_consensus_status,
// uc_platform_last_scrape_timestamp, and uc_consensus_log_timestamp.
func NewConsensusStatusCollector(nodeID int, staleness time.Duration, monitor *PlatformStatusMonitor, logMon *LogMonitor) *ConsensusStatusCollector {
	return &ConsensusStatusCollector{
		nodeID:     strconv.Itoa(nodeID),
		nodeIDInt:  nodeID,
		staleness:  staleness,
		monitor:    monitor,
		logMonitor: logMon,
		now:        time.Now,
		statusDesc: prometheus.NewDesc(
			"uc_consensus_status",
			"Consensus node platform status code. -1 = UNKNOWN.",
			[]string{"source", "node_id"}, nil,
		),
		scrapeDesc: prometheus.NewDesc(
			"uc_platform_last_scrape_timestamp",
			"Unix timestamp of the last successful consensus node metrics scrape.",
			[]string{"node_id"}, nil,
		),
		logTSDesc: prometheus.NewDesc(
			"uc_consensus_log_timestamp",
			"Unix timestamp (mtime) of the consensus log file.",
			[]string{"node_id"}, nil,
		),
	}
}

func (c *ConsensusStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.statusDesc
	ch <- c.scrapeDesc
	ch <- c.logTSDesc
}

func (c *ConsensusStatusCollector) Collect(ch chan<- prometheus.Metric) {
	v := reconcileConsensusStatus(c.nodeIDInt, c.staleness, c.now(), c.monitor, c.logMonitor)

	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, float64(v.Metrics.Code), "metrics", c.nodeID)
	scrapeTS := 0.0
	if v.Metrics.ScrapedAt != nil {
		scrapeTS = float64(v.Metrics.ScrapedAt.Unix())
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeDesc, prometheus.GaugeValue, scrapeTS, c.nodeID)

	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, float64(v.Log.Code), "log", c.nodeID)
	logTS := 0.0
	if v.Log.FileModifiedAt != nil {
		logTS = float64(v.Log.FileModifiedAt.Unix())
	}
	ch <- prometheus.MustNewConstMetric(c.logTSDesc, prometheus.GaugeValue, logTS, c.nodeID)

	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, float64(v.Effective.Code), "effective", c.nodeID)
}
