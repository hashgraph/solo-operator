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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
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
}

// ProvisionerProxyMonitor stands in for the host-level solo-provisioner daemon
// in cluster-only / hashsphere deployments. It watches NetworkUpgradeExecute CRs
// and, when one reaches ReadyForProvisionerDaemon, advances its status.phase to
// PendingNodeUpgrade — the single transition the real daemon would otherwise own
// (HIP XXXX2 §1318). It is self-healing: transient and auth errors retry with
// exponential backoff.
//
// Ported from hashgraph/solo-weaver internal/daemon/consensus/upgrade_monitor.go,
// reduced to the phase-advance behaviour (no infra upgrade / config placement,
// which are unsupported in the cluster-only profile).
type ProvisionerProxyMonitor struct {
	cfg    ProvisionerProxyMonitorConfig
	client dynamic.Interface

	mu sync.Mutex
	// activeOpID is non-empty while a transition is in-flight; guards the single
	// execution slot so two operations cannot transition concurrently.
	activeOpID string
	// completedOpIDs holds operationIds already advanced past
	// ReadyForProvisionerDaemon, stopping a rerun of the same operation. It guards
	// the patch round-trip window (the gap before the CR's new phase is observed by
	// the watch loop). Seeded at every Run entry from the cluster List, so surviving
	// a restart needs no disk persistence — the CR's phase is the durable source of
	// truth. Mirrors solo-weaver UpgradeMonitor.
	completedOpIDs map[string]struct{}

	connectivityErr atomic.Pointer[daemonkit.StatusError]

	// onTransition is called at the start of each handleExecute; nil in production,
	// set in tests to observe invocations.
	onTransition func(operationID string)
}

// NewProvisionerProxyMonitor builds a ProvisionerProxyMonitor with a dynamic
// client from kubeconfigPath (empty = in-cluster).
func NewProvisionerProxyMonitor(cfg ProvisionerProxyMonitorConfig, kubeconfigPath string) (*ProvisionerProxyMonitor, error) {
	client, err := buildDynamicClient(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("provisioner-proxy monitor: build k8s client: %w", err)
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
// NetworkUpgradeExecute CRs (seeding completedOpIDs and dispatching any already at
// ReadyForProvisionerDaemon), then watches from the List's ResourceVersion.
func (pm *ProvisionerProxyMonitor) Run(ctx context.Context) error {
	logx.As().Info().Str("reason", "ProxyMonitorStarted").
		Str("namespace", pm.cfg.Namespace).Msg("provisioner-proxy monitor started")

	backoff := proxyBackoffInitial
	for {
		listRV, listErr := pm.listAndSeed(ctx)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", "ProxyMonitorStopped").Msg("provisioner-proxy monitor stopped")
			return nil
		}
		if listErr != nil {
			pm.setConnectivityError("ProxyListError", listErr.Error(),
				fmt.Sprintf("Verify RBAC for networkupgradeexecutes in namespace %s", pm.cfg.Namespace))
			logx.As().Warn().Err(listErr).Str("reason", "ProxyListError").
				Dur("retry_in", backoff).Msg("list error — retrying")
			if !sleepCtx(ctx, backoff) {
				return nil
			}
			backoff = minDuration(time.Duration(float64(backoff)*proxyBackoffFactor), proxyBackoffMax)
			continue
		}
		pm.clearConnectivityError()

		err := pm.runWatch(ctx, listRV)
		if ctx.Err() != nil {
			logx.As().Info().Str("reason", "ProxyMonitorStopped").Msg("provisioner-proxy monitor stopped")
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
			pm.setConnectivityError("ProxyAuthError", err.Error(),
				"Restore RBAC for the UC ServiceAccount on networkupgradeexecutes")
			logx.As().Warn().Err(err).Str("reason", "ProxyAuthError").
				Dur("retry_in", backoff).Msg("auth error watching NetworkUpgradeExecute")
		} else {
			pm.setConnectivityError("ProxyWatchError", err.Error(),
				"Check connectivity to the Kubernetes API server")
			logx.As().Warn().Err(err).Str("reason", "ProxyWatchError").
				Dur("retry_in", backoff).Msg("watch error — retrying")
		}
		if !sleepCtx(ctx, backoff) {
			return nil
		}
		backoff = minDuration(time.Duration(float64(backoff)*proxyBackoffFactor), proxyBackoffMax)
	}
}

// listAndSeed lists all Execute CRs, seeds completedOpIDs from terminal-phase CRs,
// and dispatches any CR already at ReadyForProvisionerDaemon (recovery for CRs that
// arrived while the UC was offline). Returns the List ResourceVersion for a gapless
// watch.
func (pm *ProvisionerProxyMonitor) listAndSeed(ctx context.Context) (string, error) {
	resource := pm.client.Resource(networkUpgradeExecuteGVR).Namespace(pm.cfg.Namespace)
	list, err := resource.List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list NetworkUpgradeExecute: %w", err)
	}

	pm.mu.Lock()
	for i := range list.Items {
		phase, _, _ := unstructured.NestedString(list.Items[i].Object, "status", "phase")
		opID := operationIDOf(&list.Items[i])
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

	// Oldest-first so the longest-waiting operation acquires the slot first.
	var pending []*unstructured.Unstructured
	for i := range list.Items {
		phase, _, _ := unstructured.NestedString(list.Items[i].Object, "status", "phase")
		if ExecutePhaseOf(phase) == operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
			pending = append(pending, &list.Items[i])
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
	return list.GetResourceVersion(), nil
}

// runWatch performs a single watch cycle from resourceVersion. Returns nil on a
// clean channel close, or an error to back off and retry.
func (pm *ProvisionerProxyMonitor) runWatch(ctx context.Context, resourceVersion string) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", "ProxyWatchPanic").Interface("panic", r).
				Msg("panic in watch loop — recovering")
			retErr = fmt.Errorf("recovered panic: %v", r)
		}
	}()
	resource := pm.client.Resource(networkUpgradeExecuteGVR).Namespace(pm.cfg.Namespace)
	timeoutSec := proxyWatchTimeoutSeconds
	watcher, err := resource.Watch(ctx, metav1.ListOptions{
		ResourceVersion: resourceVersion,
		TimeoutSeconds:  &timeoutSec,
	})
	if err != nil {
		return fmt.Errorf("watch NetworkUpgradeExecute: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Error {
				return fmt.Errorf("watch error event: %w", k8serrors.FromObject(event.Object))
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			cr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			pm.handleEvent(ctx, cr)
		}
	}
}

// handleEvent advances a CR at ReadyForProvisionerDaemon to PendingNodeUpgrade,
// deduplicated by operationId at two levels (mirroring solo-provisioner-daemon):
// completedOpIDs stops a rerun of an already-advanced operation, and activeOpID
// stops a second transition running simultaneously.
func (pm *ProvisionerProxyMonitor) handleEvent(ctx context.Context, cr *unstructured.Unstructured) {
	phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
	if ExecutePhaseOf(phase) != operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon {
		return
	}
	operationID := operationIDOf(cr)
	if operationID == "" {
		logx.As().Warn().Str("reason", "ProxyMissingOperationID").
			Str("cr_name", cr.GetName()).Msg("Execute CR has no spec.operationId — ignoring")
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
			logx.As().Warn().Str("reason", "ProxyBusy").
				Str("active", active).Str("incoming", operationID).
				Msg("another transition is in-flight — rejecting")
		}
		return
	}
	pm.activeOpID = operationID
	pm.mu.Unlock()

	logx.As().Info().Str("reason", "ReadyForProvisionerDaemon").
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("Execute entered ReadyForProvisionerDaemon — advancing to PendingNodeUpgrade")

	go func() {
		var transErr error
		defer func() {
			if r := recover(); r != nil {
				logx.As().Error().Str("reason", "ProxyTransitionPanic").
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
			logx.As().Error().Err(transErr).Str("reason", "ProxyTransitionFailed").
				Str("operation_id", operationID).Msg("transition to PendingNodeUpgrade failed")
		}
	}()
}

// handleExecute is the single action this proxy performs: patch the Execute CR's
// status.phase from ReadyForProvisionerDaemon to PendingNodeUpgrade. In the
// Mainnet profile the host solo-provisioner daemon does this (plus infra upgrades
// and config placement); in cluster-only mode the UC owns only this transition.
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
		return fmt.Errorf("patch %s status to PendingNodeUpgrade: %w", cr.GetName(), err)
	}
	logx.As().Info().Str("reason", "PendingNodeUpgrade").
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("advanced Execute CR to PendingNodeUpgrade")
	return nil
}

// operationIDOf returns the authoritative operation id from spec.operationId.
// This mirrors the operator's indexed field and solo-weaver's upgrade monitor:
// spec.operationId is identity, while the operator.solo.hedera.com/operation-id
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
