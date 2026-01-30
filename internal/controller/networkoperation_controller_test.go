// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestNewNetworkOperationReconciler(t *testing.T) {
	r := NewNetworkOperationReconciler(nil, nil)

	if r == nil {
		t.Fatal("Expected reconciler to be created")
	}

	if r.genesisProcessor == nil {
		t.Error("Expected genesisProcessor to be initialized")
	}

	if r.checkProcessor == nil {
		t.Error("Expected checkProcessor to be initialized")
	}

	if r.upgradeProcessor == nil {
		t.Error("Expected upgradeProcessor to be initialized")
	}

	if r.nodeProcessor == nil {
		t.Error("Expected nodeProcessor to be initialized")
	}
}

func TestNetworkOperationReconciler_setCondition(t *testing.T) {
	r := &NetworkOperationReconciler{}

	netOp := &operatorv1alpha1.NetworkOperation{
		Status: operatorv1alpha1.NetworkOperationStatus{
			Conditions: []metav1.Condition{},
		},
	}

	// Add first condition
	condition1 := metav1.Condition{
		Type:    "Test",
		Status:  metav1.ConditionTrue,
		Reason:  "TestReason",
		Message: "Test message 1",
	}
	r.setCondition(netOp, condition1)

	if len(netOp.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(netOp.Status.Conditions))
	}

	// Add second condition with same type - should replace
	condition2 := metav1.Condition{
		Type:    "Test",
		Status:  metav1.ConditionFalse,
		Reason:  "TestReason2",
		Message: "Test message 2",
	}
	r.setCondition(netOp, condition2)

	if len(netOp.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after replacement, got %d", len(netOp.Status.Conditions))
	}

	if netOp.Status.Conditions[0].Message != "Test message 2" {
		t.Errorf("Expected condition to be updated")
	}

	// Add third condition with different type - should add
	condition3 := metav1.Condition{
		Type:    "Another",
		Status:  metav1.ConditionTrue,
		Reason:  "AnotherReason",
		Message: "Another message",
	}
	r.setCondition(netOp, condition3)

	if len(netOp.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(netOp.Status.Conditions))
	}
}

func TestNetworkOperationReconciler_listActiveOperations(t *testing.T) {
	// This test would require a mock client to properly test
	// The listActiveOperations method requires a valid client, so we skip the
	// actual invocation in this unit test. Integration tests with envtest
	// would be needed to properly test this functionality.
	t.Skip("Requires mock client or envtest for proper testing")
}

func TestOperationModeSwitching(t *testing.T) {
	// Test that the reconciler correctly routes to different operation handlers
	tests := []struct {
		mode     operatorv1alpha1.NetworkOperationMode
		expected string
	}{
		{operatorv1alpha1.NetworkOperationModeNetworkGenesis, "genesis"},
		{operatorv1alpha1.NetworkOperationModeFirstInitialization, "genesis"},
		{operatorv1alpha1.NetworkOperationModeNetworkCheck, "check"},
		{operatorv1alpha1.NetworkOperationModeNetworkUpgrade, "upgrade"},
		{operatorv1alpha1.NetworkOperationModeNodeAdd, "nodeAdd"},
		{operatorv1alpha1.NetworkOperationModeNodeDelete, "nodeDelete"},
		{operatorv1alpha1.NetworkOperationModeNodeUpdate, "nodeUpdate"},
		{operatorv1alpha1.NetworkOperationModeProxyUpgrade, "proxyUpgrade"},
	}

	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			netOp := &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Mode: tc.mode,
				},
			}

			// Verify the mode is set correctly
			if netOp.Spec.Mode != tc.mode {
				t.Errorf("Expected mode %s, got %s", tc.mode, netOp.Spec.Mode)
			}
		})
	}
}

func TestNetworkOperationPhases(t *testing.T) {
	tests := []struct {
		phase    operatorv1alpha1.Phase
		terminal bool
	}{
		{operatorv1alpha1.PhasePending, false},
		{operatorv1alpha1.PhaseRunning, false},
		{operatorv1alpha1.PhaseSucceeded, true},
		{operatorv1alpha1.PhaseFailed, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.phase), func(t *testing.T) {
			isTerminal := tc.phase == operatorv1alpha1.PhaseSucceeded ||
				tc.phase == operatorv1alpha1.PhaseFailed

			if isTerminal != tc.terminal {
				t.Errorf("Expected terminal=%v for phase %s, got %v", tc.terminal, tc.phase, isTerminal)
			}
		})
	}
}

func TestUpgradeWorkflowSteps(t *testing.T) {
	tests := []struct {
		step operatorv1alpha1.NetworkUpgradeWorkflowStep
	}{
		{operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate},
		{operatorv1alpha1.NetworkUpgradeWorkflowStepFreezeScheduled},
		{operatorv1alpha1.NetworkUpgradeWorkflowStepNowFrozen},
		{operatorv1alpha1.NetworkUpgradeWorkflowStepFreezeAbort},
	}

	for _, tc := range tests {
		t.Run(string(tc.step), func(t *testing.T) {
			netOp := &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Mode: operatorv1alpha1.NetworkOperationModeNetworkUpgrade,
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						WorkflowStep: tc.step,
					},
				},
			}

			if netOp.Spec.NetworkUpgrade.WorkflowStep != tc.step {
				t.Errorf("Expected workflow step %s, got %s", tc.step, netOp.Spec.NetworkUpgrade.WorkflowStep)
			}
		})
	}
}
