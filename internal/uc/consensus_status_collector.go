// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ConsensusStatusCollector is a custom prometheus.Collector that emits
// uc_consensus_status{source,node_id} at scrape time by reading the current
// monitor state atomically. This avoids races between the monitor goroutine
// (which writes) and the HTTP scrape goroutine (which reads).
type ConsensusStatusCollector struct {
	nodeID     string
	monitor    *PlatformStatusMonitor
	logMonitor *LogMonitor

	statusDesc *prometheus.Desc
	scrapeDesc *prometheus.Desc
	logTSDesc  *prometheus.Desc
}

// NewConsensusStatusCollector creates a collector for uc_consensus_status,
// uc_platform_last_scrape_timestamp, and uc_consensus_log_timestamp.
func NewConsensusStatusCollector(nodeID int, monitor *PlatformStatusMonitor, logMon *LogMonitor) *ConsensusStatusCollector {
	return &ConsensusStatusCollector{
		nodeID:     strconv.Itoa(nodeID),
		monitor:    monitor,
		logMonitor: logMon,
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
	snap := c.monitor.ScrapeSnapshot()
	metricsCode := float64(operatorv1alpha1.PlatformStatusCodeUnknown)
	scrapeTS := 0.0
	if snap != nil {
		metricsCode = float64(snap.Code)
		scrapeTS = float64(snap.ScrapedAt.Unix())
	}
	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, metricsCode, "metrics", c.nodeID)
	ch <- prometheus.MustNewConstMetric(c.scrapeDesc, prometheus.GaugeValue, scrapeTS, c.nodeID)

	logCode := float64(operatorv1alpha1.PlatformStatusCodeUnknown)
	logTS := 0.0
	if c.logMonitor != nil {
		if ls := c.logMonitor.Snapshot(); ls != nil {
			logCode = float64(ls.Code)
		}
		logTS = c.logMonitor.LogFileMtime()
	}
	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, logCode, "log", c.nodeID)
	ch <- prometheus.MustNewConstMetric(c.logTSDesc, prometheus.GaugeValue, logTS, c.nodeID)
}
