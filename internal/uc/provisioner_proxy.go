// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ProvisionerProxy stands in for the host-level solo-provisioner daemon in
// cluster-only / hashsphere deployments. It is NOT a daemonkit.MonitorRunner —
// it is pure logic wired as a callback into UpgradeOutcomeMonitor, which owns
// the List+Watch loop.
//
// Its only job: when an Execute CR reaches ReadyForProvisionerDaemon, deploy
// the config CRs the upgrade requires, then patch status.phase to
// PendingNodeUpgrade. A single execution slot (activeOpID) prevents concurrent
// transitions; completedOpIDs prevents re-patching an already-advanced operation.
//
// Ported from hashgraph/solo-weaver internal/daemon/consensus/upgrade_monitor.go.
type ProvisionerProxy struct {
	cfg    ProvisionerProxyConfig
	client dynamic.Interface

	mu sync.Mutex
	// activeOpID is non-empty while a transition is in-flight.
	activeOpID string
	// completedOpIDs holds operationIds already patched past ReadyForProvisionerDaemon.
	// Seeded at every Run entry via Seed(); no disk persistence needed — the CR's
	// phase is the durable source of truth.
	//
	// Note: NOT the Factory op-id dedup cache. The Factory cache deduplicates
	// CR creation (marker → Create). This set deduplicates the provisioner-proxy
	// status patch (ReadyForProvisionerDaemon → PendingNodeUpgrade). The UC never
	// sets ReadyForProvisionerDaemon; the operator does.
	completedOpIDs map[string]struct{}

	// onTransition is called at the start of each handleExecute; nil in production,
	// set in tests to observe invocations.
	onTransition func(operationID string)
}

// ProvisionerProxyConfig configures the ProvisionerProxy.
type ProvisionerProxyConfig struct {
	// Namespace is the orbit namespace to patch NetworkUpgradeExecute CRs in.
	Namespace string
	// NodeID is the Hedera node identifier, used in log context and to select the
	// per-node block-node config file (block-nodes/config/block-nodes-<NodeID>.json).
	NodeID int
	// CapsuleName is the owning ConsensusCapsule name (derived from the pod name).
	// When empty, this UC instance is not bound to a node and per-node
	// BlockNodesConfig deployment is skipped.
	CapsuleName string
	// UpgradePath is the root of the extracted upgrade package
	// (data/upgrade/current). The proxy scans <UpgradePath>/data/config/ and
	// <UpgradePath>/ (for log4j2.xml and settings.txt) during the execute phase.
	// Required; deployConfigCRs returns an error when empty. If data/config/ is
	// absent or contains no recognised files the deploy step is a no-op.
	UpgradePath string
	// Orbit is the owning Orbit name, written into spec.orbit on each config CR
	// so the Orbit controller can cascade-delete them.
	Orbit string
	// ConfigCRPollInterval overrides the default poll cadence when waiting for
	// config CRs to reach Valid=True. Zero falls back to DefaultConfigCRPollInterval.
	ConfigCRPollInterval time.Duration
	// ConfigCRDeployTimeout overrides the default total budget for the config-CR
	// deployment step. Zero falls back to DefaultConfigCRDeployTimeout.
	ConfigCRDeployTimeout time.Duration
	// HandoffDeadline overrides the total budget for the provisioner handoff,
	// measured from status.startTime across all retries. Zero falls back to
	// DefaultHandoffDeadline. When exceeded, a transient failure is reported
	// as DaemonResult=False (reason DeadlineExceeded) instead of retried.
	HandoffDeadline time.Duration
}

// NewProvisionerProxy constructs the proxy with an in-cluster or
// kubeconfig-based dynamic client (empty kubeconfigPath = in-cluster).
func NewProvisionerProxy(cfg ProvisionerProxyConfig, kubeconfigPath string) (*ProvisionerProxy, error) {
	client, err := buildDynamicClient(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return NewProvisionerProxyWithClient(cfg, client), nil
}

// NewProvisionerProxyWithClient constructs the proxy with an injected client
// (used in unit tests).
func NewProvisionerProxyWithClient(cfg ProvisionerProxyConfig, client dynamic.Interface) *ProvisionerProxy {
	return &ProvisionerProxy{
		cfg:            cfg,
		client:         client,
		completedOpIDs: make(map[string]struct{}),
	}
}

// Seed populates completedOpIDs from a fresh Execute CR list so the proxy does
// not re-patch operations that are already past ReadyForProvisionerDaemon.
// Called by UpgradeOutcomeMonitor.listAndSeed before the watch loop starts.
func (p *ProvisionerProxy) Seed(items []unstructured.Unstructured) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range items {
		phase, _, _ := unstructured.NestedString(items[i].Object, "status", "phase")
		opID := operationIDOf(&items[i])
		if opID == "" {
			continue
		}
		switch ExecutePhaseOf(phase) {
		case operatorv1alpha1.ExecutePhasePendingNodeUpgrade,
			operatorv1alpha1.ExecutePhaseSucceeded,
			operatorv1alpha1.ExecutePhaseFailed:
			p.completedOpIDs[opID] = struct{}{}
		}
	}
}

// EvictOperation removes an operationId from completedOpIDs so a recreated CR
// with the same operationId is not silently skipped. Called by the watch loop
// when a Deleted event is observed.
func (p *ProvisionerProxy) EvictOperation(operationID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.completedOpIDs, operationID)
}

// Handle is called by UpgradeOutcomeMonitor when an Execute CR reaches
// ReadyForProvisionerDaemon. The monitor has already validated operationID and
// the disk opId — Handle only needs to acquire the slot and dispatch.
func (p *ProvisionerProxy) Handle(ctx context.Context, cr *unstructured.Unstructured) {
	operationID := operationIDOf(cr)
	if operationID == "" {
		return
	}

	p.mu.Lock()
	if _, done := p.completedOpIDs[operationID]; done {
		p.mu.Unlock()
		return
	}
	if p.activeOpID != "" {
		active := p.activeOpID
		p.mu.Unlock()
		if active != operationID {
			logx.As().Warn().Str("reason", ReasonProvisionerProxyBusy.String()).
				Str("active", active).Str("incoming", operationID).
				Msg("Rejecting: another transition is in-flight")
		}
		return
	}
	p.activeOpID = operationID
	p.mu.Unlock()

	logx.As().Info().Str("reason", ReasonReadyForProvisionerDaemon.String()).
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("Execute entered ReadyForProvisionerDaemon — advancing to PendingNodeUpgrade")

	// Run the transition on daemonkit.Go so a panic is recovered on this
	// goroutine's own barrier — recover is per-goroutine, and the monitor
	// supervisor only guards a monitor's Run goroutine, not one it spawns itself —
	// and logged via logx (slog.Default() is the logx handler) instead of crashing
	// the UC. operationId rides on the logger as a structured field via slog's
	// With, so it lands on the recovery log line without baking it into the name.
	daemonkit.Go(ctx, slog.Default().With("operation_id", operationID), "provisioner-transition", func(ctx context.Context) {
		// completedOpIDs is set ONLY on a positive success, so a transition that
		// returns an error OR panics stays out of the set and the Execute CR
		// remains eligible for retry: a panic unwinds through this defer with
		// succeeded still false, then daemonkit.Go recovers it.
		succeeded := false
		defer func() {
			p.mu.Lock()
			if p.activeOpID == operationID {
				p.activeOpID = ""
			}
			if succeeded {
				p.completedOpIDs[operationID] = struct{}{}
			}
			p.mu.Unlock()
		}()
		if err := p.handleExecute(ctx, cr); err != nil {
			logx.As().Error().Err(err).Str("reason", ReasonProvisionerProxyTransitionFailed.String()).
				Str("operation_id", operationID).Msg("Failed to transition to PendingNodeUpgrade")
			// HIP-1496 termination guarantee: fatal failures are reported as
			// DaemonResult=False immediately so the operator marks the CR Failed.
			// Transient failures are retried on the next watch tick (op-id left out
			// of completedOpIDs) — but only until the handoff deadline, after which
			// they too are reported so a stuck operation terminates rather than hangs.
			switch {
			case !errorx.IsTemporary(err):
				reason := operatorv1alpha1.ReasonDaemonInfraUpgradeFailed.String()
				if r, ok := errx.ReasonOf(err); ok && r != "" {
					reason = r.String()
				}
				p.reportFailure(ctx, cr, reason, err.Error())
			case p.handoffDeadlineExceeded(cr):
				p.reportFailure(ctx, cr, operatorv1alpha1.ReasonDaemonDeadlineExceeded.String(),
					fmt.Sprintf("provisioner retry budget exceeded before handoff: %v", err))
			}
			return
		}
		succeeded = true
	})
}

// handleExecute runs the execute-phase workflow for the cluster-only profile.
// Host infra upgrade is not supported; the UC owns config-CR deployment and the
// phase advance.
//
// Steps (each uses a context with an explicit timeout so a hung step cannot
// silently stall the slot and block all later upgrades):
//
//  1. Deploy config CRs from the upgrade package (data/config/ + package root).
//  2. Wait for all config CRs to reach Valid=True.
//  3. Write ConfigCRsApplied=True + DaemonResult=True and advance status.phase to
//     PendingNodeUpgrade — the same handoff the host-side provisioner daemon makes
//     (see the daemon's reportOutcome), so both provisioner paths present an
//     identical contract to the operator.
func (p *ProvisionerProxy) handleExecute(ctx context.Context, cr *unstructured.Unstructured) error {
	operationID := operationIDOf(cr)
	if p.onTransition != nil {
		p.onTransition(operationID)
	}

	// Step 1 + 2: deploy config CRs and wait for reconciliation.
	deployCtx, deployCancel := context.WithTimeout(ctx, p.cfg.deployTimeout())
	defer deployCancel()
	if err := p.deployConfigCRs(deployCtx, operationID); err != nil {
		return err
	}

	// Step 3: write the success conditions and advance the phase in one status
	// patch. The provisioner (daemon or this proxy) owns the ConfigCRsApplied →
	// DaemonResult → PendingNodeUpgrade handoff; the operator only reads it.
	patch, err := buildProvisionerHandoffPatch(cr)
	if err != nil {
		return errx.WithReason(errManifestInvalid.Wrap(err, "build handoff patch for %s", cr.GetName()), ReasonExecutePatchFailed)
	}

	patchCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	resource := p.client.Resource(networkUpgradeExecuteGVR).Namespace(p.cfg.Namespace)
	if _, err := resource.Patch(patchCtx, cr.GetName(), types.MergePatchType, patch,
		metav1.PatchOptions{}, "status"); err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "patch %s status to PendingNodeUpgrade", cr.GetName()), ReasonExecutePatchFailed),
			"Confirm the UC ServiceAccount has patch permission on networkupgradeexecutes/status."+operatorv1alpha1.GroupVersion.Group,
		)
	}
	logx.As().Info().Str("reason", ReasonPendingNodeUpgrade.String()).
		Str("operation_id", operationID).Str("cr_name", cr.GetName()).
		Msg("Advanced Execute CR to PendingNodeUpgrade")
	return nil
}

// upsertCondition returns conds with a condition of condType set to the given
// status/reason/message (replacing any existing entry of that type, else
// appending). A JSON merge patch replaces the whole conditions array, so callers
// must merge onto the CR's existing conditions rather than patch a bare list.
func upsertCondition(conds []interface{}, condType string, status metav1.ConditionStatus, reason, message, now string) []interface{} {
	next := map[string]interface{}{
		"type":               condType,
		"status":             string(status),
		"reason":             reason,
		"message":            message,
		"lastTransitionTime": now,
	}
	for i, raw := range conds {
		c, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _, _ := unstructured.NestedString(c, "type"); t == condType {
			// Per Kubernetes convention lastTransitionTime marks the last status
			// flip, not the last write: preserve the prior timestamp when the status
			// value is unchanged (mirrors the operator-side setCondition).
			if s, _, _ := unstructured.NestedString(c, "status"); s == string(status) {
				if prev, _, _ := unstructured.NestedString(c, "lastTransitionTime"); prev != "" {
					next["lastTransitionTime"] = prev
				}
			}
			conds[i] = next
			return conds
		}
	}
	return append(conds, next)
}

// buildProvisionerHandoffPatch builds the JSON merge patch that mirrors the
// host-side daemon's reportOutcome success path: it upserts ConfigCRsApplied=True
// and DaemonResult=True (reason Succeeded) into the CR's existing
// status.conditions and sets status.phase to PendingNodeUpgrade.
func buildProvisionerHandoffPatch(cr *unstructured.Unstructured) ([]byte, error) {
	now := metav1.Now().Format(time.RFC3339)
	conds, _, _ := unstructured.NestedSlice(cr.Object, "status", "conditions")
	conds = upsertCondition(conds, operatorv1alpha1.CondConfigCRsApplied, metav1.ConditionTrue,
		operatorv1alpha1.ReasonDaemonSucceeded.String(), "config CRs applied and reconciled to Valid", now)
	conds = upsertCondition(conds, operatorv1alpha1.CondDaemonResult, metav1.ConditionTrue,
		operatorv1alpha1.ReasonDaemonSucceeded.String(), "provisioner completed config deployment", now)

	return json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"phase":      string(operatorv1alpha1.ExecutePhasePendingNodeUpgrade),
			"conditions": conds,
		},
	})
}

// buildProvisionerFailurePatch builds the JSON merge patch that mirrors the
// daemon's reportOutcome failure path: it upserts DaemonResult=False (carrying
// the error's reason code) into the CR's existing status.conditions and does NOT
// touch status.phase — the operator reads DaemonResult=False and marks the CR
// Failed, terminal. reason is the error's PascalCase reason code.
func buildProvisionerFailurePatch(cr *unstructured.Unstructured, reason, message string) ([]byte, error) {
	now := metav1.Now().Format(time.RFC3339)
	conds, _, _ := unstructured.NestedSlice(cr.Object, "status", "conditions")
	conds = upsertCondition(conds, operatorv1alpha1.CondDaemonResult, metav1.ConditionFalse, reason, message, now)

	return json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": conds,
		},
	})
}

// reportFailure writes DaemonResult=False (with the given reason and message)
// without advancing status.phase, so the operator marks the CR Failed. Called for
// fatal failures and for transient failures that outlast the handoff deadline.
func (p *ProvisionerProxy) reportFailure(ctx context.Context, cr *unstructured.Unstructured, reason, message string) {
	patch, err := buildProvisionerFailurePatch(cr, reason, message)
	if err != nil {
		logx.As().Error().Err(err).Str("reason", ReasonExecutePatchFailed.String()).
			Str("cr_name", cr.GetName()).Msg("Failed to build DaemonResult=False patch")
		return
	}

	patchCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	resource := p.client.Resource(networkUpgradeExecuteGVR).Namespace(p.cfg.Namespace)
	if _, err := resource.Patch(patchCtx, cr.GetName(), types.MergePatchType, patch,
		metav1.PatchOptions{}, "status"); err != nil {
		logx.As().Error().Err(err).Str("reason", ReasonExecutePatchFailed.String()).
			Str("cr_name", cr.GetName()).Msg("Failed to patch DaemonResult=False")
		return
	}
	logx.As().Warn().Str("reason", reason).Str("cr_name", cr.GetName()).
		Msg("Wrote DaemonResult=False; operator will mark Execute CR Failed")
}

// handoffDeadlineExceeded reports whether the handoff budget, measured from when
// the CR entered ReadyForProvisionerDaemon (status.startTime), has elapsed. If
// status.startTime is absent or unparseable the deadline is treated as NOT
// exceeded — a missing anchor must not force a premature terminal failure; the
// operation stays retryable until the anchor appears.
func (p *ProvisionerProxy) handoffDeadlineExceeded(cr *unstructured.Unstructured) bool {
	startStr, ok, _ := unstructured.NestedString(cr.Object, "status", "startTime")
	if !ok || startStr == "" {
		return false
	}
	started, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return false
	}
	return time.Since(started) > p.cfg.handoffDeadline()
}

func (c ProvisionerProxyConfig) handoffDeadline() time.Duration {
	if c.HandoffDeadline > 0 {
		return c.HandoffDeadline
	}
	return DefaultHandoffDeadline
}

func (c ProvisionerProxyConfig) pollInterval() time.Duration {
	if c.ConfigCRPollInterval > 0 {
		return c.ConfigCRPollInterval
	}
	return DefaultConfigCRPollInterval
}

func (c ProvisionerProxyConfig) deployTimeout() time.Duration {
	if c.ConfigCRDeployTimeout > 0 {
		return c.ConfigCRDeployTimeout
	}
	return DefaultConfigCRDeployTimeout
}
