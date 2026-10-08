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
	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

// platformStatusOperationID is the fixed pseudo operation id under which the
// platform-status sensor records its transition audit trail. Platform-status
// changes are not upgrade operations, so they share a single JSONL file
// (platform-status.jsonl) rather than one per upgrade.
const platformStatusOperationID = "platform-status"

// metricsScrapeTimeout bounds a single metrics scrape so a hung endpoint cannot
// stall the poll loop. It is generous relative to a localhost GET.
const metricsScrapeTimeout = 10 * time.Second

// PlatformScrapeSnapshot is an atomically-published view of the last successful
// platform-status scrape. The metrics collector reads it at scrape time; the
// monitor goroutine writes it — the atomic.Pointer handoff avoids races.
type PlatformScrapeSnapshot struct {
	Code      int32
	Status    string
	ScrapedAt time.Time
}

// PlatformStatusMonitor is the UC's platform-status sensor. It continuously
// scrapes the consensus node's metrics endpoint (localhost:9999) for
// platform_PlatformStatus and, on each state TRANSITION, patches the
// operator.solo.hiero.org/platform-status condition onto its OWN pod (the pod named
// by UC_POD_NAME, via pods/status). The operator proxies that condition into the
// ConsensusCapsule status, so the operator stays the sole writer of the capsule
// status while the UC owns only a surface on its own pod.
//
// The condition is informational only — it is deliberately NOT a readiness gate,
// so it never affects pod readiness ("detection is active polling, not a probe").
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

	// logMonitor provides the log-based fallback status. When the metrics
	// endpoint is unreachable, the effective status is derived from the log
	// monitor instead of reporting UNKNOWN.
	logMonitor *LogMonitor

	// last is the last platform-status name written to the pod condition, and lastCode
	// the numeric code that accompanied it. A patch is issued when a freshly-scraped
	// reading differs in EITHER field, so a code change that maps to the same name (only
	// possible for UNKNOWN: an unreachable endpoint vs. an unmapped code, or two different
	// unmapped codes) still refreshes the condition's message instead of leaving a stale
	// code. last is empty before the first successful write, so the first reading always
	// writes.
	last     string
	lastCode int32

	connectivityErr atomic.Pointer[daemonkit.StatusError]
	scrapeSnapshot  atomic.Pointer[PlatformScrapeSnapshot]
}

// NewPlatformStatusMonitor constructs the platform-status sensor. logMon may be
// nil (the monitor falls back to raw-scrape-only behaviour).
func NewPlatformStatusMonitor(cfg Config, client dynamic.Interface, sink *eventSink, logMon *LogMonitor) *PlatformStatusMonitor {
	return &PlatformStatusMonitor{
		cfg:        cfg,
		client:     client,
		http:       &http.Client{Timeout: metricsScrapeTimeout},
		events:     sink,
		logMonitor: logMon,
	}
}

// Name implements daemonkit.MonitorRunner.
func (m *PlatformStatusMonitor) Name() string { return "platform-status-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (m *PlatformStatusMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

// ScrapeSnapshot returns the last successful scrape snapshot, or nil if no
// successful scrape has occurred yet.
func (m *PlatformStatusMonitor) ScrapeSnapshot() *PlatformScrapeSnapshot {
	return m.scrapeSnapshot.Load()
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
				Msg("Recovering from panic in platform-status monitor")
			retErr = errx.WithReason(errPanic.New("recovered panic in platform-status monitor: %v", r), ReasonPlatformStatusMonitorPanic)
		}
	}()

	logx.As().Info().
		Str("reason", ReasonPlatformStatusMonitorStarted.String()).
		Str("capsule", m.cfg.CapsuleName()).
		Str("metrics_url", m.cfg.MetricsURL).
		Dur("poll_interval", m.cfg.PlatformPollInterval).
		Msg("Started platform-status monitor")

	ticker := time.NewTicker(m.cfg.PlatformPollInterval)
	defer ticker.Stop()

	// Scrape once immediately so status.platformStatus is populated without waiting
	// a full interval.
	m.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			logx.As().Info().Str("reason", ReasonPlatformStatusMonitorStopped.String()).
				Msg("Stopped platform-status monitor")
			return nil
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

// poll scrapes the current platform status and, if the reading differs from the
// last written one (in name OR numeric code), patches the pod condition and — on a
// genuine name change — records the transition. When the metrics scrape reports
// UNKNOWN (endpoint unreachable or sample absent), the log monitor's status is
// used as a fallback so a dead :9999 no longer masks a status the log already knows.
func (m *PlatformStatusMonitor) poll(ctx context.Context) {
	code, name := m.scrape(ctx)

	if code == platformStatusCodeUnknown && m.logMonitor != nil {
		if snap := m.logMonitor.Snapshot(); snap != nil && snap.Status != operatorv1alpha1.PlatformStatusUnknown {
			code = snap.Code
			name = snap.Status
		}
	}

	if name == m.last && code == m.lastCode {
		return // nothing observable changed — no write
	}
	if err := m.patch(ctx, name, code); err != nil {
		common.LogErr(logx.As().Error(), err, ReasonPlatformStatusPatchFailed).
			Str("pod", m.cfg.PodName).
			Str("platform_status", name).
			Msg("Failed to patch pod platform-status condition")
		return // leave m.last/m.lastCode unchanged so the next tick retries
	}
	// Only a name change is a real platform-status transition worth recording. A
	// code-only refresh under the same name (an UNKNOWN sub-state change) still repatches
	// to keep the message current, but is not logged as a transition to avoid from==to
	// audit-trail noise.
	if name != m.last {
		m.recordTransition(m.last, name, code)
	}
	m.last = name
	m.lastCode = code
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
	m.scrapeSnapshot.Store(&PlatformScrapeSnapshot{Code: code, Status: nameForCode(code), ScrapedAt: time.Now()})
	return code, nameForCode(code)
}

// patch writes the current platform status as the operator.solo.hiero.org/platform-status
// condition on the UC's OWN pod (pods/status). It is the only place that condition
// is written; the operator proxies it into the ConsensusCapsule status.
//
// The reason carries the raw state name (machine-readable — the operator reads it);
// the message carries a human detail line including the numeric code; status is True
// for a concrete reading and Unknown when the state is UNKNOWN (metrics unreachable /
// sample absent). The condition is not a readiness gate, so its status never affects
// pod readiness.
func (m *PlatformStatusMonitor) patch(ctx context.Context, name string, code int32) error {
	patch := platformStatusConditionPatch(name, code, time.Now().UTC().Format(time.RFC3339))
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	resource := m.client.Resource(podGVR).Namespace(m.cfg.Namespace)
	_, err := resource.Patch(ctx, m.cfg.PodName, types.StrategicMergePatchType, patch,
		metav1.PatchOptions{}, "status")
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "patch pod %s status condition %s", m.cfg.PodName, operatorv1alpha1.PodConditionPlatformStatus), ReasonPlatformStatusPatchFailed),
			"Confirm the UC ServiceAccount has patch permission on pods/status in its namespace",
		)
	}
	return nil
}

// platformStatusConditionPatch builds the STRATEGIC merge patch body that sets the
// operator.solo.hiero.org/platform-status condition on the UC's pod. Pod conditions
// merge by their `type` key, so this updates only that condition and leaves the
// kubelet-owned Ready / ContainersReady / PodScheduled conditions intact — a plain
// JSON merge patch would replace the whole conditions array and break the pod. The
// reason carries the raw state name (machine-readable), the message a human detail
// line including the numeric code, and status is Unknown for UNKNOWN, True otherwise.
func platformStatusConditionPatch(name string, code int32, lastTransition string) []byte {
	condStatus := corev1.ConditionTrue
	if name == operatorv1alpha1.PlatformStatusUnknown {
		condStatus = corev1.ConditionUnknown
	}
	message := fmt.Sprintf("Platform status %s (code %d)", name, code)
	return []byte(fmt.Sprintf(
		`{"status":{"conditions":[{"type":%q,"status":%q,"reason":%q,"message":%q,"lastTransitionTime":%q}]}}`,
		operatorv1alpha1.PodConditionPlatformStatus, condStatus, name, message, lastTransition))
}

// recordTransition writes the transition to the structured logs and the JSONL
// audit trail. It emits no Kubernetes Events (UC convention).
func (m *PlatformStatusMonitor) recordTransition(prev, name string, code int32) {
	if prev == "" {
		prev = nonePhase
	}
	msg := fmt.Sprintf("Detected platform status change: %s → %s (code %d)", prev, name, code)
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
