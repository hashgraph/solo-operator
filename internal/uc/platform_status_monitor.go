// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// platformStatusOperationID is the fixed pseudo operation id under which the
// platform-status sensor records its transition audit trail. Platform-status
// changes are not upgrade operations, so they share a single JSONL file
// (platform-status.jsonl) rather than one per upgrade.
const platformStatusOperationID = "platform-status"

// metricsScrapeTimeout bounds a single metrics scrape so a hung endpoint cannot
// stall the poll loop. It is generous relative to a localhost GET.
const metricsScrapeTimeout = 10 * time.Second

// PlatformStatusMonitor is the UC's platform-status sensor. It continuously
// scrapes the consensus node's metrics endpoint (localhost:9999) for
// platform_PlatformStatus and, on each state TRANSITION, patches its owning
// ConsensusCapsule's status.platformStatus (+ platformStatusCode). It is the
// SOLE writer of status.platformStatus.
//
// It is fail-soft: when the endpoint is unreachable it records UNKNOWN and keeps
// polling (the failure is surfaced via /status ConnectivityError). It emits NO
// Kubernetes Events (repo convention for the UC); transitions go to the JSONL
// event log and the structured logs only.
type PlatformStatusMonitor struct {
	cfg    Config
	client dynamic.Interface
	http   *http.Client
	events *eventSink

	// last is the last platform-status name written to the capsule. A patch is
	// issued only when a freshly-scraped name differs from it — transition-only
	// writes. Empty before the first successful write, so the first reading always
	// writes.
	last string

	connectivityErr atomic.Pointer[daemonkit.StatusError]
}

// NewPlatformStatusMonitor constructs the platform-status sensor.
func NewPlatformStatusMonitor(cfg Config, client dynamic.Interface, sink *eventSink) *PlatformStatusMonitor {
	return &PlatformStatusMonitor{
		cfg:    cfg,
		client: client,
		http:   &http.Client{Timeout: metricsScrapeTimeout},
		events: sink,
	}
}

// Name implements daemonkit.MonitorRunner.
func (m *PlatformStatusMonitor) Name() string { return "platform-status-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (m *PlatformStatusMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

// Run scrapes the metrics endpoint on PlatformPollInterval until ctx is cancelled.
func (m *PlatformStatusMonitor) Run(ctx context.Context) (retErr error) {
	// Convert an unexpected panic in the poll loop into a returned error so the
	// daemonkit supervisor restarts this monitor with back-off, instead of the
	// panic unwinding to the top of its goroutine and crashing the whole UC
	// process (daemonkit does not recover; a panic in any monitor kills the
	// daemon). Mirrors UpgradeOutcomeMonitor.runWatch.
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", ReasonPlatformStatusMonitorPanic.String()).
				Str("capsule", m.cfg.CapsuleName()).Interface("panic", r).
				Msg("panic in platform-status monitor — recovering")
			retErr = errx.WithReason(errPanic.New("recovered panic in platform-status monitor: %v", r), ReasonPlatformStatusMonitorPanic)
		}
	}()

	logx.As().Info().
		Str("reason", ReasonPlatformStatusMonitorStarted.String()).
		Str("capsule", m.cfg.CapsuleName()).
		Str("metrics_url", m.cfg.MetricsURL).
		Dur("poll_interval", m.cfg.PlatformPollInterval).
		Msg("platform-status monitor started")

	ticker := time.NewTicker(m.cfg.PlatformPollInterval)
	defer ticker.Stop()

	// Scrape once immediately so status.platformStatus is populated without waiting
	// a full interval.
	m.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			logx.As().Info().Str("reason", ReasonPlatformStatusMonitorStopped.String()).
				Msg("platform-status monitor stopped")
			return nil
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

// poll scrapes the current platform status and, if it differs from the last
// written value, patches the capsule and records the transition.
func (m *PlatformStatusMonitor) poll(ctx context.Context) {
	code, name := m.scrape(ctx)
	if name == m.last {
		return // no transition — nothing to write
	}
	if err := m.patch(ctx, name, code); err != nil {
		common.LogErr(logx.As().Error(), err, ReasonPlatformStatusPatchFailed).
			Str("capsule", m.cfg.CapsuleName()).
			Str("platform_status", name).
			Msg("failed to patch ConsensusCapsule platform status")
		return // leave m.last unchanged so the next tick retries
	}
	m.recordTransition(m.last, name, code)
	m.last = name
}

// scrape fetches the metrics endpoint and extracts the platform-status code and
// name. It is fail-soft: on any fetch/parse problem it returns UNKNOWN and
// records a ConnectivityError; on success it clears the error.
func (m *PlatformStatusMonitor) scrape(ctx context.Context) (code int32, name string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.MetricsURL, http.NoBody)
	if err != nil {
		m.setConnErr(ReasonPlatformMetricsUnreachable, err.Error())
		return platformStatusCodeUnknown, operatorv1alpha1.PlatformStatusUnknown
	}
	resp, err := m.http.Do(req)
	if err != nil {
		m.setConnErr(ReasonPlatformMetricsUnreachable, err.Error())
		return platformStatusCodeUnknown, operatorv1alpha1.PlatformStatusUnknown
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Drain a little so the connection can be reused, then treat as unreachable.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		m.setConnErr(ReasonPlatformMetricsUnreachable, fmt.Sprintf("metrics endpoint returned HTTP %d", resp.StatusCode))
		return platformStatusCodeUnknown, operatorv1alpha1.PlatformStatusUnknown
	}

	code, found := parsePlatformStatus(resp.Body)
	// Endpoint was reachable — clear any prior connectivity error even when the
	// sample is not yet present (the node may still be initialising its metrics).
	m.clearConnErr()
	if !found {
		return platformStatusCodeUnknown, operatorv1alpha1.PlatformStatusUnknown
	}
	return code, nameForCode(code)
}

// patch writes the new platform status onto the capsule's status subresource. It
// is the only place status.platformStatus is written.
func (m *PlatformStatusMonitor) patch(ctx context.Context, name string, code int32) error {
	patch := []byte(fmt.Sprintf(`{"status":{"platformStatus":%q,"platformStatusCode":%d,"platformStatusRevision":%q}}`, name, code, m.cfg.PodRevision))
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	resource := m.client.Resource(consensusCapsuleGVR).Namespace(m.cfg.Namespace)
	_, err := resource.Patch(ctx, m.cfg.CapsuleName(), types.MergePatchType, patch,
		metav1.PatchOptions{}, "status")
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "patch %s status.platformStatus", m.cfg.CapsuleName()), ReasonPlatformStatusPatchFailed),
			"Confirm the UC ServiceAccount has patch permission on consensuscapsules."+operatorv1alpha1.GroupVersion.Group+"/status",
		)
	}
	return nil
}

// recordTransition writes the transition to the structured logs and the JSONL
// audit trail. It emits no Kubernetes Events (UC convention).
func (m *PlatformStatusMonitor) recordTransition(prev, name string, code int32) {
	if prev == "" {
		prev = nonePhase
	}
	msg := fmt.Sprintf("platform status %s -> %s (code %d)", prev, name, code)
	logx.As().Info().
		Str("reason", ReasonPlatformStatusChanged.String()).
		Str("capsule", m.cfg.CapsuleName()).
		Str("from", prev).
		Str("to", name).
		Int32("code", code).
		Msg(msg)
	m.events.info(platformStatusOperationID, ReasonPlatformStatusChanged.String(), msg)
}

// setConnErr records a connectivity failure visible via /status. The scrape path
// is fail-soft, so this never stops the monitor.
func (m *PlatformStatusMonitor) setConnErr(reason errx.Reason, message string) {
	m.connectivityErr.Store(&daemonkit.StatusError{
		Reason:     reason.String(),
		Message:    message,
		Resolution: "Confirm the consensus node exposes " + m.cfg.MetricsURL + " (platform_PlatformStatus)",
		Since:      time.Now().UTC().Format(time.RFC3339),
	})
}

// clearConnErr clears any recorded connectivity failure after a successful scrape.
func (m *PlatformStatusMonitor) clearConnErr() {
	m.connectivityErr.Store(nil)
}
