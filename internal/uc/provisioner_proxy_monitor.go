// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/logx"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

const (
	proxyBackoffInitial = 2 * time.Second
	proxyBackoffMax     = 5 * time.Minute
	proxyBackoffFactor  = 2.0

	// proxyWatchTimeoutSeconds is the server-side watch timeout. The server
	// closes the channel cleanly after this window; Run reconnects.
	proxyWatchTimeoutSeconds = int64(5 * 60)
)

// ProvisionerProxyMonitorConfig configures the provisioner-proxy monitor.
type ProvisionerProxyMonitorConfig struct {
	// Namespace is the orbit namespace to watch for NetworkUpgradeExecute CRs.
	Namespace string
	// NodeID is the Hedera node identifier (a non-negative integer), used in log context.
	NodeID int
	// UpgradePath is the current upgrade-package directory (data/upgrade/current).
	UpgradePath string
	// PreviousUpgradePath is the previous upgrade-package directory (data/upgrade/previous),
	// used as the rotation target after a successful install.
	PreviousUpgradePath string
}

// ProvisionerProxyMonitor stands in for the host-level solo-provisioner daemon
// in cluster-only / hashsphere deployments. It watches NetworkUpgradeExecute CRs
// and, when one reaches ReadyForProvisionerDaemon, runs the execute-phase
// workflow — deploying the config CRs the upgrade requires — and only then
// advances status.phase to PendingNodeUpgrade. It is self-healing: transient and
// auth errors retry with exponential backoff.
//
// Ported from hashgraph/solo-weaver internal/daemon/consensus/upgrade_monitor.go.
// The cluster-only profile drops the host infra upgrade (unsupported there) but
// retains config-CR deployment, so the workflow runs asynchronously (single slot
// guarded by activeOpID) rather than as a bare status patch — the phase advance
// is the final step once the config CRs are in place.
type ProvisionerProxyMonitor struct {
	cfg    ProvisionerProxyMonitorConfig
	client dynamic.Interface

	mu sync.Mutex
	// activeOpID is non-empty while a transition is in-flight; guards the single
	// execution slot so two operations cannot transition concurrently.
	activeOpID string
	// completedOpIDs holds operationIds for which this monitor has already patched
	// (or observed) the Execute CR past ReadyForProvisionerDaemon. It prevents a
	// second patch attempt during the window between the status write and the watch
	// loop observing the new phase. Seeded at every Run entry from the cluster List
	// — no disk persistence needed because the CR's phase is the durable source of
	// truth. Mirrors solo-weaver UpgradeMonitor.
	//
	// Note: this is NOT the Factory op-id dedup cache. The Factory cache deduplicates
	// CR creation (marker → Create). This set deduplicates the provisioner-proxy
	// status patch (ReadyForProvisionerDaemon → PendingNodeUpgrade). The UC never
	// sets ReadyForProvisionerDaemon; the operator does. The monitor's only write
	// action is the status patch.
	completedOpIDs map[string]struct{}

	connectivityErr atomic.Pointer[daemonkit.StatusError]

	// onTransition is called at the start of each handleExecute; nil in production,
	// set in tests to observe invocations.
	onTransition func(operationID string)

	// onPhaseUpdate is called whenever a watch event delivers a phase transition for
	// a known CR. Used to keep the Factory dedup cache current so deliverCR can log
	// the correct severity (ERROR on Failed) without a re-query. Nil is safe.
	onPhaseUpdate func(operationID, step, phase string)
}

// NewProvisionerProxyMonitor builds a ProvisionerProxyMonitor with a dynamic
// client from kubeconfigPath (empty = in-cluster).
func NewProvisionerProxyMonitor(cfg ProvisionerProxyMonitorConfig, kubeconfigPath string) (*ProvisionerProxyMonitor, error) {
	client, err := buildDynamicClient(kubeconfigPath)
	if err != nil {
		// Already an errx-decorated client error.
		return nil, err
	}
	return NewProvisionerProxyMonitorWithClient(cfg, client), nil
}

// NewProvisionerProxyMonitorWithClient builds the monitor with an injected client
// (used in unit tests).
func NewProvisionerProxyMonitorWithClient(cfg ProvisionerProxyMonitorConfig, client dynamic.Interface) *ProvisionerProxyMonitor {
	return &ProvisionerProxyMonitor{
		cfg:            cfg,
		client:         client,
		completedOpIDs: make(map[string]struct{}),
	}
}

// Name implements daemonkit.MonitorRunner.
func (pm *ProvisionerProxyMonitor) Name() string { return "provisioner-proxy-monitor" }

// ConnectivityError implements daemonkit.ConnectivityMonitor.
func (pm *ProvisionerProxyMonitor) ConnectivityError() *daemonkit.StatusError {
	return pm.connectivityErr.Load()
}

func (pm *ProvisionerProxyMonitor) setConnectivityError(reason, message, resolution string) {
	since := time.Now().UTC().Format(time.RFC3339)
	if prev := pm.connectivityErr.Load(); prev != nil {
		since = prev.Since
	}
	pm.connectivityErr.Store(&daemonkit.StatusError{
		Reason:     reason,
		Message:    message,
		Resolution: resolution,
		Since:      since,
	})
}

func (pm *ProvisionerProxyMonitor) clearConnectivityError() {
	pm.connectivityErr.Store(nil)
}

// Run blocks until ctx is cancelled. Each iteration lists all
// NetworkUpgradeExecute and NetworkUpgradeFreezeAbort CRs (seeding completedOpIDs,
// rotating for terminal states, and dispatching any already at
// ReadyForProvisionerDaemon), then watches both CR types from their List ResourceVersions.
func (pm *ProvisionerProxyMonitor) Run(ctx context.Context) error {
	logx.As().Info().Str("reason", ReasonProvisionerProxyMonitorStarted.String()).
		Str("namespace", pm.cfg.Namespace).Msg("provisioner-proxy monitor started")

	backoff := proxyBackoffInitial
	for {
		executeRV, abortRV, listErr := pm.listAndSeed(ctx)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", ReasonProvisionerProxyMonitorStopped.String()).Msg("provisioner-proxy monitor stopped")
			return nil
		}
		if listErr != nil {
			pm.setConnectivityError(ReasonProvisionerProxyListError.String(), listErr.Error(),
				fmt.Sprintf("Verify RBAC for networkupgradeexecutes in namespace %s", pm.cfg.Namespace))
			logx.As().Warn().Err(listErr).Str("reason", ReasonProvisionerProxyListError.String()).
				Dur("retry_in", backoff).Msg("list error — retrying")
			if !sleepCtx(ctx, backoff) {
				return nil
			}
			backoff = minDuration(time.Duration(float64(backoff)*proxyBackoffFactor), proxyBackoffMax)
			continue
		}
		pm.clearConnectivityError()

		err := pm.runWatch(ctx, executeRV, abortRV)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", ReasonProvisionerProxyMonitorStopped.String()).Msg("provisioner-proxy monitor stopped")
			return nil
		}
		if err == nil {
			// Clean channel close (server expiry). Sleep before reconnect to avoid hot-loop.
			backoff = proxyBackoffInitial
			if !sleepCtx(ctx, proxyBackoffInitial) {
				return nil
			}
			continue
		}
		if isAuthError(err) {
			pm.setConnectivityError(ReasonProvisionerProxyAuthError.String(), err.Error(),
				"Restore RBAC for the UC ServiceAccount on networkupgradeexecutes")
			logx.As().Warn().Err(err).Str("reason", ReasonProvisionerProxyAuthError.String()).
				Dur("retry_in", backoff).Msg("auth error watching NetworkUpgradeExecute")
		} else {
			pm.setConnectivityError(ReasonProvisionerProxyWatchError.String(), err.Error(),
				"Check connectivity to the Kubernetes API server")
			logx.As().Warn().Err(err).Str("reason", ReasonProvisionerProxyWatchError.String()).
				Dur("retry_in", backoff).Msg("watch error — retrying")
		}
		if !sleepCtx(ctx, backoff) {
			return nil
		}
		backoff = minDuration(time.Duration(float64(backoff)*proxyBackoffFactor), proxyBackoffMax)
	}
}

// listAndSeed lists all Execute and FreezeAbort CRs on startup. It does three things:
//
//  1. Seeds completedOpIDs with any Execute CR whose phase is already past the
//     provisioner-proxy action point (PendingNodeUpgrade, Succeeded, Failed).
//     These are operations the monitor must not re-patch — either it already did
//     so in a prior run, or the operation resolved independently. The three phases
//     cover every state where a second patch would be wrong: PendingNodeUpgrade
//     means the patch already landed; Succeeded/Failed mean the upgrade reconciler
//     finished, so patching again would be meaningless or harmful.
//
//  2. Rotates/discards upgrade packages for terminal CRs (Execute.Succeeded →
//     rotate into previous/; FreezeAbort.Succeeded → discard current/).
//     Execute.Failed is intentionally excluded: current/ is preserved for
//     diagnostics, same as Prepare/Freeze failures.
//
//  3. Dispatches any Execute CR already at ReadyForProvisionerDaemon so the
//     monitor recovers operations that arrived while the UC was offline.
//
// Returns the Execute and FreezeAbort List ResourceVersions for a gapless watch.
func (pm *ProvisionerProxyMonitor) listAndSeed(ctx context.Context) (executeRV, abortRV string, _ error) {
	// Bound the List with a per-call deadline. The dynamic client has no
	// client-wide request timeout (see buildDynamicClient), so without this a
	// stalled List would block the Run loop indefinitely. Scoped to the List only —
	// handleEvent below keeps the parent ctx so a triggered transition is not
	// cancelled after listTimeout.
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	execList, err := pm.client.Resource(networkUpgradeExecuteGVR).Namespace(pm.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradeExecute"), ReasonExecuteListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradeexecutes."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	abortList, err := pm.client.Resource(networkUpgradeFreezeAbortGVR).Namespace(pm.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradeFreezeAbort"), ReasonFreezeAbortListFailed),
			"Confirm the UC ServiceAccount has list permission on networkupgradefreezeaborts."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	pm.mu.Lock()
	for i := range execList.Items {
		phase, _, _ := unstructured.NestedString(execList.Items[i].Object, "status", "phase")
		opID := operationIDOf(&execList.Items[i])
		if opID == "" {
			continue
		}
		switch ExecutePhaseOf(phase) {
		case operatorv1alpha1.ExecutePhasePendingNodeUpgrade,
			operatorv1alpha1.ExecutePhaseSucceeded,
			operatorv1alpha1.ExecutePhaseFailed:
			pm.completedOpIDs[opID] = struct{}{}
		}
	}
	pm.mu.Unlock()

	// Rotate on Execute.Succeeded so current is cleared before any new
	// ReadyForProvisionerDaemon transition acquires the slot. rotateUpgradePackage
	// is idempotent — a no-op if current is already empty. Execute.Failed is
	// intentionally excluded: like Prepare/Freeze failures it requires human
	// intervention; current is preserved for diagnostics.
	for i := range execList.Items {
		phase, _, _ := unstructured.NestedString(execList.Items[i].Object, "status", "phase")
		if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseSucceeded {
			pm.handleTerminalExecute(&execList.Items[i])
		}
	}

	// Rotate on FreezeAbort.Succeeded (abort cycle complete). Failed means the
	// abort reconciler itself failed — the abort was not processed, so no rotation.
	for i := range abortList.Items {
		phase, _, _ := unstructured.NestedString(abortList.Items[i].Object, "status", "phase")
		if operatorv1alpha1.Phase(phase) == operatorv1alpha1.PhaseSucceeded {
			pm.handleTerminalAbort(&abortList.Items[i])
		}
	}

	// Oldest-first so the longest-waiting operation acquires the slot first.
	var pending []*unstructured.Unstructured
	for i := range execList.Items {
		phase, _, _ := unstructured.NestedString(execList.Items[i].Object, "status", "phase")
		if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
			pending = append(pending, &execList.Items[i])
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		ti := pending[i].GetCreationTimestamp()
		tj := pending[j].GetCreationTimestamp()
		return ti.Before(&tj)
	})
	for _, cr := range pending {
		pm.handleEvent(ctx, cr)
	}
	return execList.GetResourceVersion(), abortList.GetResourceVersion(), nil
}

// runWatch performs a single watch cycle from executeRV and abortRV. Returns nil
// on a clean channel close, or an error to back off and retry.
func (pm *ProvisionerProxyMonitor) runWatch(ctx context.Context, executeRV, abortRV string) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", ReasonProvisionerProxyWatchPanic.String()).Interface("panic", r).
				Msg("panic in watch loop — recovering")
			retErr = errx.WithReason(errorx.InternalError.New("recovered panic in watch loop: %v", r), ReasonProvisionerProxyWatchPanic)
		}
	}()
	timeoutSec := proxyWatchTimeoutSeconds

	execWatcher, err := pm.client.Resource(networkUpgradeExecuteGVR).Namespace(pm.cfg.Namespace).
		Watch(ctx, metav1.ListOptions{ResourceVersion: executeRV, TimeoutSeconds: &timeoutSec})
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "watch NetworkUpgradeExecute"), ReasonWatchFailed),
			"Confirm the UC ServiceAccount has watch permission on networkupgradeexecutes."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	defer execWatcher.Stop()

	abortWatcher, err := pm.client.Resource(networkUpgradeFreezeAbortGVR).Namespace(pm.cfg.Namespace).
		Watch(ctx, metav1.ListOptions{ResourceVersion: abortRV, TimeoutSeconds: &timeoutSec})
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "watch NetworkUpgradeFreezeAbort"), ReasonWatchFailed),
			"Confirm the UC ServiceAccount has watch permission on networkupgradefreezeaborts."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	defer abortWatcher.Stop()

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
			pm.handleEvent(ctx, cr)
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
			if pm.onPhaseUpdate != nil {
				if opID := operationIDOf(cr); opID != "" {
					pm.onPhaseUpdate(opID, cr.GetLabels()[operatorv1alpha1.LabelStep], phase)
				}
			}
			if operatorv1alpha1.Phase(phase) == operatorv1alpha1.PhaseSucceeded {
				pm.handleTerminalAbort(cr)
			}
		}
	}
}

// handleEvent dispatches on the CR's current phase:
//   - Succeeded: rotates the upgrade package out of the current directory.
//   - ReadyForProvisionerDaemon: deploys config CRs and advances to PendingNodeUpgrade.
//   - Failed: no action — human intervention required; current is preserved for diagnostics.
//
// The ReadyForProvisionerDaemon path is deduplicated by operationId at two levels
// (mirroring solo-provisioner-daemon): completedOpIDs stops a rerun of an
// already-advanced operation, and activeOpID stops concurrent transitions.
func (pm *ProvisionerProxyMonitor) handleEvent(ctx context.Context, cr *unstructured.Unstructured) {
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if pm.onPhaseUpdate != nil {
		if opID := operationIDOf(cr); opID != "" {
			pm.onPhaseUpdate(opID, cr.GetLabels()[operatorv1alpha1.LabelStep], phase)
		}
	}
	if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseSucceeded {
		pm.handleTerminalExecute(cr)
		return
	}
	if ExecutePhaseOf(phase) != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		return
	}
	operationID := operationIDOf(cr)
	if operationID == "" {
		logx.As().Warn().Str("reason", ReasonProvisionerProxyMissingOperationID.String()).
			Str("cr_name", cr.GetName()).Msg("Execute CR has no spec.operationId — ignoring")
		return
	}

	// Validate that this CR's operationId matches the current upgrade on disk
	// before acquiring the slot. Without this check a stale ReadyForProvisionerDaemon
	// CR from a prior cycle (whose phase patch was lost) would incorrectly trigger a
	// new transition when the UC restarts with a different upgrade in current.
	if !pm.matchesDiskOperationID(operationID, cr.GetName(), map[string]string{"phase": string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon)}) {
		return
	}

	pm.mu.Lock()
	if _, done := pm.completedOpIDs[operationID]; done {
		pm.mu.Unlock()
		return
	}
	if pm.activeOpID != "" {
		active := pm.activeOpID
		pm.mu.Unlock()
		if active != operationID {
			logx.As().Warn().Str("reason", ReasonProvisionerProxyBusy.String()).
				Str("active", active).Str("incoming", operationID).
				Msg("another transition is in-flight — rejecting")
		}
		return
	}
	pm.activeOpID = operationID
	pm.mu.Unlock()

	logx.As().Info().Str("reason", ReasonReadyForProvisionerDaemon.String()).
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("Execute entered ReadyForProvisionerDaemon — advancing to PendingNodeUpgrade")

	go func() {
		var transErr error
		defer func() {
			if r := recover(); r != nil {
				logx.As().Error().Str("reason", ReasonProvisionerProxyTransitionPanic.String()).
					Str("operation_id", operationID).Interface("panic", r).
					Msg("transition panicked — UC continues")
			}
			pm.mu.Lock()
			if pm.activeOpID == operationID {
				pm.activeOpID = ""
			}
			if transErr == nil {
				pm.completedOpIDs[operationID] = struct{}{}
			}
			pm.mu.Unlock()
		}()
		transErr = pm.handleExecute(ctx, cr)
		if transErr != nil {
			logx.As().Error().Err(transErr).Str("reason", ReasonProvisionerProxyTransitionFailed.String()).
				Str("operation_id", operationID).Msg("transition to PendingNodeUpgrade failed")
		}
	}()
}

// handleExecute runs the execute-phase workflow for the given CR. In the Mainnet
// profile the host solo-provisioner daemon owns this (infra upgrade + config
// placement + phase advance); in cluster-only mode the UC owns the config-CR
// deployment and the phase advance (infra upgrade is unsupported).
//
// TODO: deploy the config CRs the upgrade requires before the phase advance. Each
// step must use a context derived from ctx with an explicit timeout — if a step
// hangs, activeOpID stays set and the proxy rejects all later upgrades silently.
//
// The final step (below) patches status.phase ReadyForProvisionerDaemon ->
// PendingNodeUpgrade once the config CRs are in place.
func (pm *ProvisionerProxyMonitor) handleExecute(ctx context.Context, cr *unstructured.Unstructured) error {
	operationID := operationIDOf(cr)
	if pm.onTransition != nil {
		pm.onTransition(operationID)
	}

	patch := []byte(fmt.Sprintf(
		`{"status":{"phase":%q}}`, operatorv1alpha1.ExecutePhasePendingNodeUpgrade))

	resource := pm.client.Resource(networkUpgradeExecuteGVR).Namespace(pm.cfg.Namespace)
	_, err := resource.Patch(ctx, cr.GetName(), types.MergePatchType, patch,
		metav1.PatchOptions{}, "status")
	if err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "patch %s status to PendingNodeUpgrade", cr.GetName()), ReasonExecutePatchFailed),
			"Confirm the UC ServiceAccount has patch permission on networkupgradeexecutes/status."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	logx.As().Info().Str("reason", ReasonPendingNodeUpgrade.String()).
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("advanced Execute CR to PendingNodeUpgrade")
	return nil
}

// handleTerminalExecute rotates the upgrade package when the Execute CR reaches
// Succeeded. Failed is intentionally excluded — like Prepare/Freeze failures it
// requires human intervention; current is preserved for diagnostics. Before
// rotating it confirms the CR's operationId matches the one on disk so that a
// stale CR from a previous cycle cannot clear a new upgrade package.
func (pm *ProvisionerProxyMonitor) handleTerminalExecute(cr *unstructured.Unstructured) {
	operationID := operationIDOf(cr)
	if operationID == "" {
		return
	}
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if !pm.matchesDiskOperationID(operationID, cr.GetName(), map[string]string{"phase": phase}) {
		return
	}
	rotateUpgradePackage(pm.cfg.UpgradePath, pm.cfg.PreviousUpgradePath, operationID)
}

// handleTerminalAbort discards the staged upgrade package when a FreezeAbort CR
// reaches Succeeded. Abort means the freeze was cancelled before the package was
// applied — the staged contents of current/ are garbage, not a rollback candidate.
// Discarding (wipe current/*, leave previous/ untouched) is the correct action:
// rotating into previous would clobber the genuine last-good rollback copy with
// a package that was never installed. Failed is excluded — a Failed FreezeAbort
// means the abort reconciler itself failed; leave current/ intact for diagnosis.
// The same disk-operationId guard applies to prevent stale CRs from prior cycles
// from wiping a new upgrade package.
func (pm *ProvisionerProxyMonitor) handleTerminalAbort(cr *unstructured.Unstructured) {
	operationID := operationIDOf(cr)
	if operationID == "" {
		return
	}
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if !pm.matchesDiskOperationID(operationID, cr.GetName(), map[string]string{"phase": phase, "cr_type": "FreezeAbort"}) {
		return
	}
	discardUpgradePackage(pm.cfg.UpgradePath, operationID)
}

// matchesDiskOperationID returns true only when operationID matches the one
// derived from execute_immediate.mf on disk. extra is merged into every log
// line so callers can inject context (e.g. {"phase": "Succeeded"}) without
// coupling this helper to a specific call site. Returns false (with a log) on
// IO error, missing marker, or mismatch.
func (pm *ProvisionerProxyMonitor) matchesDiskOperationID(operationID, crName string, extra map[string]string) bool {
	addExtra := func(ev *zerolog.Event) *zerolog.Event {
		for k, v := range extra {
			ev = ev.Str(k, v)
		}
		return ev.Str("operation_id", operationID).Str("cr_name", crName)
	}

	diskOpID, _, _, found, err := deriveOperationIDPrimary(pm.cfg.UpgradePath)
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

// operationIDOf returns the authoritative operation id from spec.operationId.
// This mirrors the operator's indexed field and solo-weaver's upgrade monitor:
// spec.operationId is identity, while the operator.solo.hiero.org/operation-id
// label is only for List/Watch selection.
func operationIDOf(cr *unstructured.Unstructured) string {
	id, _, _ := unstructured.NestedString(cr.Object, "spec", "operationId")
	return id
}

// ExecutePhaseOf converts a raw status.phase string into the typed enum.
func ExecutePhaseOf(s string) operatorv1alpha1.ExecutePhase {
	return operatorv1alpha1.ExecutePhase(s)
}

// sleepCtx sleeps for d or until ctx is cancelled. Returns false if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
