// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/automa-saga/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
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
	// NodeID is the Hedera node identifier, used in log context.
	NodeID int
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
	// config CRs to reach Valid=True. Zero falls back to configCRPollInterval.
	ConfigCRPollInterval time.Duration
	// ConfigCRDeployTimeout overrides the default total budget for the config-CR
	// deployment step. Zero falls back to configCRDeployTimeout.
	ConfigCRDeployTimeout time.Duration
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
				Msg("another transition is in-flight — rejecting")
		}
		return
	}
	p.activeOpID = operationID
	p.mu.Unlock()

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
			p.mu.Lock()
			if p.activeOpID == operationID {
				p.activeOpID = ""
			}
			if transErr == nil {
				p.completedOpIDs[operationID] = struct{}{}
			}
			p.mu.Unlock()
		}()
		transErr = p.handleExecute(ctx, cr)
		if transErr != nil {
			logx.As().Error().Err(transErr).Str("reason", ReasonProvisionerProxyTransitionFailed.String()).
				Str("operation_id", operationID).Msg("transition to PendingNodeUpgrade failed")
		}
	}()
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
//  3. Patch Execute CR status.phase to PendingNodeUpgrade.
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

	// Step 3: advance Execute CR to PendingNodeUpgrade.
	patch := []byte(fmt.Sprintf(
		`{"status":{"phase":%q}}`, operatorv1alpha1.ExecutePhasePendingNodeUpgrade))

	patchCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	resource := p.client.Resource(networkUpgradeExecuteGVR).Namespace(p.cfg.Namespace)
	_, err := resource.Patch(patchCtx, cr.GetName(), types.MergePatchType, patch,
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

func (c ProvisionerProxyConfig) pollInterval() time.Duration {
	if c.ConfigCRPollInterval > 0 {
		return c.ConfigCRPollInterval
	}
	return configCRPollInterval
}

func (c ProvisionerProxyConfig) deployTimeout() time.Duration {
	if c.ConfigCRDeployTimeout > 0 {
		return c.ConfigCRDeployTimeout
	}
	return configCRDeployTimeout
}
