// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/logx"
	"github.com/rs/zerolog"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

const (
	outcomeBackoffInitial = 2 * time.Second
	outcomeBackoffMax     = 5 * time.Minute
	outcomeBackoffFactor  = 2.0

	// outcomeWatchTimeoutSeconds is the server-side watch timeout. The server
	// closes the channel cleanly after this window; Run reconnects.
	outcomeWatchTimeoutSeconds = int64(5 * 60)
)

// UpgradeOutcomeMonitorConfig configures the upgrade-outcome monitor.
type UpgradeOutcomeMonitorConfig struct {
	// Namespace is the orbit namespace to watch for NetworkUpgrade* CRs.
	Namespace string
	// NodeID is the Hedera node identifier, used in log context.
	NodeID int
	// UpgradePath is the current upgrade-package directory (data/upgrade/current).
	UpgradePath string
	// PreviousUpgradePath is the rotation target after a successful install.
	PreviousUpgradePath string
}

// UpgradeOutcomeMonitor is a core UC monitor that always runs (all deployment
// modes). It watches NetworkUpgradeExecute and NetworkUpgradeFreezeAbort CRs
// and reacts to terminal phases with filesystem actions:
//
//   - Execute.Succeeded  → rotate current/ into previous/ (upgrade applied)
//   - FreezeAbort.Succeeded → discard current/ (freeze cancelled, package unused)
//   - Execute.Failed     → no action; current/ preserved for human diagnosis
//
// It also drives two callbacks:
//
//   - onPhaseUpdate: called on every phase transition so the Factory dedup cache
//     stays current without extra API calls.
//   - onReadyForProvisionerDaemon: called when an Execute CR reaches
//     ReadyForProvisionerDaemon. Nil in mainnet mode; set to ProvisionerProxy.Handle
//     in cluster-only mode.
type UpgradeOutcomeMonitor struct {
	cfg    UpgradeOutcomeMonitorConfig
	client dynamic.Interface

	mu sync.Mutex
	// proxy is set when cluster-only mode is active. Seeded from the startup List.
	proxy *ProvisionerProxy

	// onPhaseUpdate is called on every Execute watch event to keep the Factory
	// dedup cache current. Nil is safe.
	onPhaseUpdate func(operationID, step, phase string)

	connectivityErr atomic.Pointer[daemonkit.StatusError]
}

// NewUpgradeOutcomeMonitor constructs the monitor with an in-cluster or
// kubeconfig-based dynamic client (empty kubeconfigPath = in-cluster).
func NewUpgradeOutcomeMonitor(cfg UpgradeOutcomeMonitorConfig, kubeconfigPath string) (*UpgradeOutcomeMonitor, error) {
	client, err := buildDynamicClient(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return NewUpgradeOutcomeMonitorWithClient(cfg, client), nil
}

// NewUpgradeOutcomeMonitorWithClient constructs the monitor with an injected
// client (used in unit tests).
func NewUpgradeOutcomeMonitorWithClient(cfg UpgradeOutcomeMonitorConfig, client dynamic.Interface) *UpgradeOutcomeMonitor {
	return &UpgradeOutcomeMonitor{cfg: cfg, client: client}
}

// Name implements daemonkit.MonitorRunner.
func (m *UpgradeOutcomeMonitor) Name() string { return "upgrade-outcome-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (m *UpgradeOutcomeMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

func (m *UpgradeOutcomeMonitor) setConnectivityError(reason, message, resolution string) {
	since := time.Now().UTC().Format(time.RFC3339)
	if prev := m.connectivityErr.Load(); prev != nil {
		since = prev.Since
	}
	m.connectivityErr.Store(&daemonkit.StatusError{
		Reason:     reason,
		Message:    message,
		Resolution: resolution,
		Since:      since,
	})
}

func (m *UpgradeOutcomeMonitor) clearConnectivityError() {
	m.connectivityErr.Store(nil)
}

// Run implements daemonkit.MonitorRunner. It loops: List → Watch → backoff on
// error, reconnect on clean channel close.
func (m *UpgradeOutcomeMonitor) Run(ctx context.Context) error {
	logx.As().Info().Str("reason", ReasonUpgradeOutcomeMonitorStarted.String()).
		Str("namespace", m.cfg.Namespace).Msg("upgrade-outcome monitor started")

	backoff := outcomeBackoffInitial
	for {
		executeRV, abortRV, listErr := m.listAndSeed(ctx)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", ReasonUpgradeOutcomeMonitorStopped.String()).Msg("upgrade-outcome monitor stopped")
			return nil
		}
		if listErr != nil {
			m.setConnectivityError(ReasonProvisionerProxyListError.String(), listErr.Error(),
				fmt.Sprintf("Verify RBAC for networkupgradeexecutes in namespace %s", m.cfg.Namespace))
			logx.As().Warn().Err(listErr).Str("reason", ReasonProvisionerProxyListError.String()).
				Dur("retry_in", backoff).Msg("list error — retrying")
			if !sleepCtx(ctx, backoff) {
				return nil
			}
			backoff = minDuration(time.Duration(float64(backoff)*outcomeBackoffFactor), outcomeBackoffMax)
			continue
		}
		m.clearConnectivityError()

		err := m.runWatch(ctx, executeRV, abortRV)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", ReasonUpgradeOutcomeMonitorStopped.String()).Msg("upgrade-outcome monitor stopped")
			return nil
		}
		if err == nil {
			backoff = outcomeBackoffInitial
			if !sleepCtx(ctx, outcomeBackoffInitial) {
				return nil
			}
			continue
		}
		if isAuthError(err) {
			m.setConnectivityError(ReasonProvisionerProxyAuthError.String(), err.Error(),
				"Restore RBAC for the UC ServiceAccount on networkupgradeexecutes")
			logx.As().Warn().Err(err).Str("reason", ReasonProvisionerProxyAuthError.String()).
				Dur("retry_in", backoff).Msg("auth error watching NetworkUpgradeExecute")
		} else {
			m.setConnectivityError(ReasonProvisionerProxyWatchError.String(), err.Error(),
				"Check connectivity to the Kubernetes API server")
			logx.As().Warn().Err(err).Str("reason", ReasonProvisionerProxyWatchError.String()).
				Dur("retry_in", backoff).Msg("watch error — retrying")
		}
		if !sleepCtx(ctx, backoff) {
			return nil
		}
		backoff = minDuration(time.Duration(float64(backoff)*outcomeBackoffFactor), outcomeBackoffMax)
	}
}

// listAndSeed lists all Execute and FreezeAbort CRs. It:
//  1. Seeds the ProvisionerProxy's completedOpIDs (if wired) from terminal Execute CRs.
//  2. Rotates/discards upgrade packages for already-terminal CRs (idempotent).
//  3. Dispatches any Execute CR already at ReadyForProvisionerDaemon via the
//     onReadyForProvisionerDaemon callback (recovery for CRs that arrived while the
//     UC was offline).
//
// Returns the Execute and FreezeAbort List ResourceVersions for a gapless watch.
func (m *UpgradeOutcomeMonitor) listAndSeed(ctx context.Context) (executeRV, abortRV string, _ error) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	execList, err := m.client.Resource(networkUpgradeExecuteGVR).Namespace(m.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradeExecute"), ReasonExecuteListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradeexecutes."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	abortList, err := m.client.Resource(networkUpgradeFreezeAbortGVR).Namespace(m.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradeFreezeAbort"), ReasonFreezeAbortListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradefreezeaborts."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	// Let ProvisionerProxy seed its completedOpIDs from the Execute list so it
	// doesn't re-patch operations that are already past ReadyForProvisionerDaemon.
	m.mu.Lock()
	proxy := m.proxy
	m.mu.Unlock()
	if proxy != nil {
		proxy.Seed(execList.Items)
	}

	// Rotate on Execute.Succeeded so current is clear before a new
	// ReadyForProvisionerDaemon transition acquires the slot.
	for i := range execList.Items {
		phase, _, _ := unstructured.NestedString(execList.Items[i].Object, "status", "phase")
		if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseSucceeded {
			m.handleTerminalExecute(&execList.Items[i])
		}
	}

	// Discard on FreezeAbort.Succeeded (freeze cancelled, package unused).
	for i := range abortList.Items {
		phase, _, _ := unstructured.NestedString(abortList.Items[i].Object, "status", "phase")
		if operatorv1alpha1.Phase(phase) == operatorv1alpha1.PhaseSucceeded {
			m.handleTerminalAbort(&abortList.Items[i])
		}
	}

	// Recover any Execute CR already at ReadyForProvisionerDaemon (arrived while UC was offline).
	var pending []*unstructured.Unstructured
	for i := range execList.Items {
		phase, _, _ := unstructured.NestedString(execList.Items[i].Object, "status", "phase")
		if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
			pending = append(pending, &execList.Items[i])
		}
	}
	// Oldest-first so the longest-waiting operation acquires the slot first.
	for _, cr := range pending {
		if proxy != nil {
			proxy.Handle(ctx, cr)
		}
	}

	return execList.GetResourceVersion(), abortList.GetResourceVersion(), nil
}

func (m *UpgradeOutcomeMonitor) runWatch(ctx context.Context, executeRV, abortRV string) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", ReasonProvisionerProxyWatchPanic.String()).Interface("panic", r).
				Msg("panic in watch loop — recovering")
			retErr = errx.WithReason(errK8sAPI.New("recovered panic in watch loop: %v", r), ReasonProvisionerProxyWatchPanic)
		}
	}()
	timeoutSec := outcomeWatchTimeoutSeconds

	execWatcher, err := m.client.Resource(networkUpgradeExecuteGVR).Namespace(m.cfg.Namespace).
		Watch(ctx, metav1.ListOptions{ResourceVersion: executeRV, TimeoutSeconds: &timeoutSec})
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "watch NetworkUpgradeExecute"), ReasonWatchFailed),
			"Confirm the UC ServiceAccount has watch permission on networkupgradeexecutes."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	defer execWatcher.Stop()

	abortWatcher, err := m.client.Resource(networkUpgradeFreezeAbortGVR).Namespace(m.cfg.Namespace).
		Watch(ctx, metav1.ListOptions{ResourceVersion: abortRV, TimeoutSeconds: &timeoutSec})
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "watch NetworkUpgradeFreezeAbort"), ReasonWatchFailed),
			"Confirm the UC ServiceAccount has watch permission on networkupgradefreezeaborts."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	defer abortWatcher.Stop()

	m.mu.Lock()
	proxy := m.proxy
	m.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-execWatcher.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Error {
				return errx.WithReason(
					errK8sAPI.Wrap(k8serrors.FromObject(event.Object), "watch error event"), ReasonWatchErrorEvent)
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			cr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			m.handleExecuteEvent(ctx, cr, proxy)
		case event, ok := <-abortWatcher.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Error {
				return errx.WithReason(
					errK8sAPI.Wrap(k8serrors.FromObject(event.Object), "abort watch error event"), ReasonWatchErrorEvent)
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			cr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
			if m.onPhaseUpdate != nil {
				if opID := operationIDOf(cr); opID != "" {
					m.onPhaseUpdate(opID, cr.GetLabels()[operatorv1alpha1.LabelStep], phase)
				}
			}
			if operatorv1alpha1.Phase(phase) == operatorv1alpha1.PhaseSucceeded {
				m.handleTerminalAbort(cr)
			}
		}
	}
}

// handleExecuteEvent dispatches on the Execute CR's current phase:
//   - All phases: updates onPhaseUpdate cache callback.
//   - Succeeded: rotates the upgrade package.
//   - ReadyForProvisionerDaemon: validates disk opId then delegates to
//     ProvisionerProxy.Handle (nil-safe; no-op in mainnet mode).
func (m *UpgradeOutcomeMonitor) handleExecuteEvent(ctx context.Context, cr *unstructured.Unstructured, proxy *ProvisionerProxy) {
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if m.onPhaseUpdate != nil {
		if opID := operationIDOf(cr); opID != "" {
			m.onPhaseUpdate(opID, cr.GetLabels()[operatorv1alpha1.LabelStep], phase)
		}
	}
	switch ExecutePhaseOf(phase) {
	case operatorv1alpha1.ExecutePhaseSucceeded:
		m.handleTerminalExecute(cr)
	case operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon:
		if proxy == nil {
			return
		}
		operationID := operationIDOf(cr)
		if operationID == "" {
			logx.As().Warn().Str("reason", ReasonProvisionerProxyMissingOperationID.String()).
				Str("cr_name", cr.GetName()).Msg("Execute CR has no spec.operationId — ignoring")
			return
		}
		// Disk opId guard lives here (not in ProvisionerProxy) because UpgradePath
		// belongs to the monitor's config. A stale ReadyForProvisionerDaemon CR from
		// a prior cycle must not trigger a new transition for a different upgrade.
		if !m.matchesDiskOperationID(operationID, cr.GetName(),
			map[string]string{"phase": string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon)}) {
			return
		}
		proxy.Handle(ctx, cr)
	}
}

// handleTerminalExecute rotates the upgrade package when an Execute CR reaches
// Succeeded. Failed is excluded — current/ is preserved for human diagnosis.
// The disk-operationId guard prevents a stale CR from a prior cycle from
// rotating a new upgrade package.
func (m *UpgradeOutcomeMonitor) handleTerminalExecute(cr *unstructured.Unstructured) {
	operationID := operationIDOf(cr)
	if operationID == "" {
		return
	}
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if !m.matchesDiskOperationID(operationID, cr.GetName(), map[string]string{"phase": phase}) {
		return
	}
	rotateUpgradePackage(m.cfg.UpgradePath, m.cfg.PreviousUpgradePath, operationID)
}

// handleTerminalAbort discards the staged upgrade package when a FreezeAbort CR
// reaches Succeeded. Abort means the freeze was cancelled before the package was
// applied — the staged contents of current/ are garbage, not a rollback candidate.
// Discarding (wipe current/*, leave previous/ untouched) is the correct action:
// rotating into previous would clobber the genuine last-good rollback copy with
// a package that was never installed. Failed is excluded — a Failed FreezeAbort
// means the abort reconciler itself failed; leave current/ intact for diagnosis.
func (m *UpgradeOutcomeMonitor) handleTerminalAbort(cr *unstructured.Unstructured) {
	operationID := operationIDOf(cr)
	if operationID == "" {
		return
	}
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if !m.matchesDiskOperationID(operationID, cr.GetName(), map[string]string{"phase": phase, "cr_type": "FreezeAbort"}) {
		return
	}
	discardUpgradePackage(m.cfg.UpgradePath, operationID)
}

// matchesDiskOperationID returns true only when operationID matches the one
// derived from execute_immediate.mf on disk. Returns false (with a log) on IO
// error, missing marker, or mismatch.
func (m *UpgradeOutcomeMonitor) matchesDiskOperationID(operationID, crName string, extra map[string]string) bool {
	addExtra := func(ev *zerolog.Event) *zerolog.Event {
		for k, v := range extra {
			ev = ev.Str(k, v)
		}
		return ev.Str("operation_id", operationID).Str("cr_name", crName)
	}

	diskOpID, _, _, found, err := deriveOperationIDPrimary(m.cfg.UpgradePath)
	if err != nil {
		addExtra(logx.As().Warn().Err(err).Str("reason", ReasonUpgradePackageRotateFailed.String())).
			Msg("could not read disk operationId — skipping")
		return false
	}
	if !found {
		addExtra(logx.As().Debug().Str("reason", ReasonProvisionerProxyMissingOperationID.String())).
			Msg("no execute_immediate.mf on disk — skipping (current may be empty after rotation)")
		return false
	}
	if diskOpID != operationID {
		addExtra(logx.As().Info().Str("disk_operation_id", diskOpID)).
			Msg("CR operationId does not match disk — skipping (stale CR)")
		return false
	}
	return true
}
