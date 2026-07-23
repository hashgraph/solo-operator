// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"encoding/json"
	"net/http"
	"time"
)

// consensusStatusResponse is the JSON body returned by GET /consensus/status.
type consensusStatusResponse struct {
	NodeID     int             `json:"nodeId"`
	Now        time.Time       `json:"now"`
	SnapshotAt time.Time       `json:"snapshotAt"`
	Effective  EffectiveStatus `json:"effective"`
	Metrics    MetricsStatus   `json:"metrics"`
	Log        LogStatus       `json:"log"`
	Staleness  string          `json:"staleness"`
}

// ConsensusStatusHandler implements daemonkit.ComponentHandler, registering the
// GET /consensus/status endpoint on the Unix-socket control plane.
//
// The endpoint returns the last-known state from the PlatformStatusMonitor and
// LogMonitor — it does NOT trigger a fresh scrape. Data freshness depends on
// each monitor's poll interval (PlatformStatusMonitor ≈ poll cycle, LogMonitor
// ≈ 1 s). The "fresh" field in the response indicates whether the metrics
// scrape is within the configured staleness window.
type ConsensusStatusHandler struct {
	nodeID     int
	staleness  time.Duration
	platform   *PlatformStatusMonitor
	logMonitor *LogMonitor
}

// NewConsensusStatusHandler creates the handler.
func NewConsensusStatusHandler(nodeID int, staleness time.Duration, platform *PlatformStatusMonitor, logMon *LogMonitor) *ConsensusStatusHandler {
	return &ConsensusStatusHandler{
		nodeID:     nodeID,
		staleness:  staleness,
		platform:   platform,
		logMonitor: logMon,
	}
}

func (h *ConsensusStatusHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /consensus/status", h.handle)
}

func (h *ConsensusStatusHandler) handle(w http.ResponseWriter, _ *http.Request) {
	now := time.Now().UTC()
	v := reconcileConsensusStatus(h.nodeID, h.staleness, now, h.platform, h.logMonitor)
	resp := consensusStatusResponse{
		NodeID:     v.NodeID,
		Now:        now,
		SnapshotAt: now,
		Effective:  v.Effective,
		Metrics:    v.Metrics,
		Log:        v.Log,
		Staleness:  v.Staleness.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
