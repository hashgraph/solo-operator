// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func TestGenesisStatus_AddCondition(t *testing.T) {
	status := &GenesisStatus{
		Phase:      operatorv1alpha1.PhasePending,
		Conditions: []metav1.Condition{},
	}

	now := metav1.Now()
	condition := metav1.Condition{
		Type:               StepConditionTypeVerifySecrets,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNodeSecrets,
		Message:            "Test message",
	}

	status.AddCondition(condition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(status.Conditions))
	}

	if status.Conditions[0].Type != StepConditionTypeVerifySecrets {
		t.Errorf("Expected condition type %s, got %s", StepConditionTypeVerifySecrets, status.Conditions[0].Type)
	}

	// Add another condition with same type - should replace
	newCondition := metav1.Condition{
		Type:               StepConditionTypeVerifySecrets,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNodeSecrets,
		Message:            "New message",
	}

	status.AddCondition(newCondition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after replacement, got %d", len(status.Conditions))
	}

	if status.Conditions[0].Status != metav1.ConditionFalse {
		t.Errorf("Expected condition status False, got %s", status.Conditions[0].Status)
	}
}

func TestGenesisStatus_HasSuccessfulCondition(t *testing.T) {
	status := &GenesisStatus{
		Phase: operatorv1alpha1.PhaseRunning,
		Conditions: []metav1.Condition{
			{
				Type:   StepConditionTypeVerifySecrets,
				Status: metav1.ConditionTrue,
				Reason: StepConditionReasonConsensusNodeSecrets,
			},
		},
	}

	if !status.HasSuccessfulCondition(StepConditionTypeVerifySecrets, StepConditionReasonConsensusNodeSecrets) {
		t.Error("Expected to find successful condition")
	}

	if status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonClusterConfig) {
		t.Error("Expected not to find non-existent condition")
	}

	// Test with ConditionFalse
	status.Conditions[0].Status = metav1.ConditionFalse
	if status.HasSuccessfulCondition(StepConditionTypeVerifySecrets, StepConditionReasonConsensusNodeSecrets) {
		t.Error("Expected not to find failed condition as successful")
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask(nil)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	steps := task.Steps()
	if len(steps) != 4 {
		t.Errorf("Expected 4 steps, got %d", len(steps))
	}

	// Verify step names
	expectedNames := []string{"CheckConsensusNodeSecrets", "CreatePublicKeysConfig", "CreateGenesisNetwork", "CreateAddressBook"}
	for i, step := range steps {
		if step.Name() != expectedNames[i] {
			t.Errorf("Expected step %d to be %q, got %q", i, expectedNames[i], step.Name())
		}
	}
}

func TestPrepareNodeName(t *testing.T) {
	tests := []struct {
		nodeId   int64
		expected string
	}{
		{0, "node0"},
		{1, "node1"},
		{42, "node42"},
	}

	for _, tc := range tests {
		result := common.NodeAlias(tc.nodeId)
		if result != tc.expected {
			t.Errorf("NodeAlias(%d) = %q, expected %q", tc.nodeId, result, tc.expected)
		}
	}
}
