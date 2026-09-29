// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	"github.com/automa-saga/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	DefaultMetricsAddr = ":9990"

	metricsShutdownTimeout = 5 * time.Second
)

// NewMetricsRegistry creates a Prometheus registry with Go/process collectors
// and the UC self-health metrics. The returned registry is ready to be served
// via promhttp. Domain collectors (e.g. ConsensusStatusCollector) are
// registered by the caller before the server starts.
func NewMetricsRegistry(cfg Config, tracker *daemonkit.StatusTracker, monitors []daemonkit.MonitorRunner) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	info := version.Get()
	reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "uc_build_info",
			Help:        "Build information for the UC sidecar.",
			ConstLabels: prometheus.Labels{"version": info.Version, "commit": info.Commit},
		},
		func() float64 { return 1 },
	))
	reg.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "uc_up",
			Help: "Always 1 when the UC sidecar is running.",
		},
		func() float64 { return 1 },
	))
	reg.MustRegister(&monitorUpCollector{
		tracker:  tracker,
		monitors: monitors,
		desc: prometheus.NewDesc(
			"uc_monitor_up",
			"Per-monitor liveness: 1 = running, 0 = not running.",
			[]string{"monitor"}, nil,
		),
	})

	return reg
}

// monitorUpCollector reports one gauge per monitor from the daemonkit tracker.
type monitorUpCollector struct {
	tracker  *daemonkit.StatusTracker
	monitors []daemonkit.MonitorRunner
	desc     *prometheus.Desc
}

func (c *monitorUpCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c *monitorUpCollector) Collect(ch chan<- prometheus.Metric) {
	snap := c.tracker.Snapshot()
	for _, mon := range c.monitors {
		val := 0.0
		if s, ok := snap[mon.Name()]; ok && s.State == "running" {
			val = 1
		}
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, val, mon.Name())
	}
}

// runMetricsServer starts the HTTP server for /metrics and blocks until ctx is
// cancelled. It performs a graceful shutdown on context cancellation.
func runMetricsServer(ctx context.Context, addr string, reg *prometheus.Registry) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.Wrap(err, "listen on %s", addr), ReasonMetricsServerFailed),
			"Confirm --metrics-addr (UC_METRICS_ADDR) port is available",
		)
	}

	errCh := make(chan error, 1)
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	logx.As().Info().
		Str("reason", ReasonMetricsServerStarted.String()).
		Str("addr", addr).
		Msg("Started metrics server")

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
		defer cancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			logx.As().Warn().Err(shutErr).Msg("Failed to shut down metrics server")
		}
		return nil
	case serveErr := <-errCh:
		return errx.WithHints(
			errx.WithReason(errMetricsServer.Wrap(serveErr, "metrics server exited unexpectedly"), ReasonMetricsServerFailed),
			"Check the metrics server logs for the root cause",
		)
	}
}
