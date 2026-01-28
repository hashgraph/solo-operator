// SPDX-License-Identifier: Apache-2.0

package node

import (
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeStatus_AddCondition(t *testing.T) {
	status := &NodeStatus{
		Phase:      operatorv1alpha1.PhasePending,
		Conditions: []metav1.Condition{},
	}

	now := metav1.Now()
	condition := metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeAdd,
		Message:            "Test message",
	}

	status.AddCondition(condition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(status.Conditions))
	}

	// Add another condition with same type and reason - should replace
	newCondition := metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeAdd,
		Message:            "New message",
	}

	status.AddCondition(newCondition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after replacement, got %d", len(status.Conditions))
	}

	// Add condition with same type but different reason - should add
	anotherCondition := metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeDelete,
		Message:            "Different reason",
	}

	status.AddCondition(anotherCondition)

	if len(status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(status.Conditions))
	}
}

func TestNodeStatus_HasSuccessfulCondition(t *testing.T) {
	status := &NodeStatus{
		Phase: operatorv1alpha1.PhaseRunning,
		Conditions: []metav1.Condition{
			{
				Type:   StepConditionTypeNodeOperation,
				Status: metav1.ConditionTrue,
				Reason: StepConditionReasonNodeAdd,
			},
		},
	}

	if !status.HasSuccessfulCondition(StepConditionTypeNodeOperation, StepConditionReasonNodeAdd) {
		t.Error("Expected to find successful condition")
	}

	if status.HasSuccessfulCondition(StepConditionTypeNodeOperation, StepConditionReasonNodeDelete) {
		t.Error("Expected not to find non-existent condition")
	}

	// Test with ConditionFalse
	status.Conditions[0].Status = metav1.ConditionFalse
	if status.HasSuccessfulCondition(StepConditionTypeNodeOperation, StepConditionReasonNodeAdd) {
		t.Error("Expected not to find failed condition as successful")
	}
}

func TestNewProcessor(t *testing.T) {
	p := NewProcessor(nil)

	if p == nil {
		t.Fatal("Expected processor to be created")
	}

	if p.nodeAddTask == nil {
		t.Error("Expected nodeAddTask to be initialized")
	}

	if p.nodeUpdateTask == nil {
		t.Error("Expected nodeUpdateTask to be initialized")
	}

	if p.nodeDeleteTask == nil {
		t.Error("Expected nodeDeleteTask to be initialized")
	}
}

func TestNewNodeAddTask(t *testing.T) {
	task := NewNodeAddTask(nil)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if len(task.steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(task.steps))
	}

	if task.steps[0].Name() != "NodeAddStep" {
		t.Errorf("Expected step name 'NodeAddStep', got %q", task.steps[0].Name())
	}
}

func TestNewNodeUpdateTask(t *testing.T) {
	task := NewNodeUpdateTask(nil)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if len(task.steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(task.steps))
	}

	if task.steps[0].Name() != "NodeUpdateStep" {
		t.Errorf("Expected step name 'NodeUpdateStep', got %q", task.steps[0].Name())
	}
}

func TestNewNodeDeleteTask(t *testing.T) {
	task := NewNodeDeleteTask(nil)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	if len(task.steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(task.steps))
	}

	if task.steps[0].Name() != "NodeDeleteStep" {
		t.Errorf("Expected step name 'NodeDeleteStep', got %q", task.steps[0].Name())
	}
}

func TestNodeAddStep_ShouldSkip(t *testing.T) {
	step := NewNodeAddStep(nil)

	tests := []struct {
		name     string
		status   *NodeStatus
		expected bool
	}{
		{
			name: "skip when failed",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseFailed,
			},
			expected: true,
		},
		{
			name: "skip when already successful",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseRunning,
				Conditions: []metav1.Condition{
					{
						Type:   StepConditionTypeNodeOperation,
						Status: metav1.ConditionTrue,
						Reason: StepConditionReasonNodeAdd,
					},
				},
			},
			expected: true,
		},
		{
			name: "don't skip when pending",
			status: &NodeStatus{
				Phase:      operatorv1alpha1.PhasePending,
				Conditions: []metav1.Condition{},
			},
			expected: false,
		},
	}

	op := &operatorv1alpha1.NetworkOperation{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := step.ShouldSkip(op, tc.status)
			if result != tc.expected {
				t.Errorf("ShouldSkip() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestNodeUpdateStep_ShouldSkip(t *testing.T) {
	step := NewNodeUpdateStep(nil)

	tests := []struct {
		name     string
		status   *NodeStatus
		expected bool
	}{
		{
			name: "skip when failed",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseFailed,
			},
			expected: true,
		},
		{
			name: "skip when already successful",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseRunning,
				Conditions: []metav1.Condition{
					{
						Type:   StepConditionTypeNodeOperation,
						Status: metav1.ConditionTrue,
						Reason: StepConditionReasonNodeUpdate,
					},
				},
			},
			expected: true,
		},
		{
			name: "don't skip when pending",
			status: &NodeStatus{
				Phase:      operatorv1alpha1.PhasePending,
				Conditions: []metav1.Condition{},
			},
			expected: false,
		},
	}

	op := &operatorv1alpha1.NetworkOperation{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := step.ShouldSkip(op, tc.status)
			if result != tc.expected {
				t.Errorf("ShouldSkip() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestNodeDeleteStep_ShouldSkip(t *testing.T) {
	step := NewNodeDeleteStep(nil)

	tests := []struct {
		name     string
		status   *NodeStatus
		expected bool
	}{
		{
			name: "skip when failed",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseFailed,
			},
			expected: true,
		},
		{
			name: "skip when already successful",
			status: &NodeStatus{
				Phase: operatorv1alpha1.PhaseRunning,
				Conditions: []metav1.Condition{
					{
						Type:   StepConditionTypeNodeOperation,
						Status: metav1.ConditionTrue,
						Reason: StepConditionReasonNodeDelete,
					},
				},
			},
			expected: true,
		},
		{
			name: "don't skip when pending",
			status: &NodeStatus{
				Phase:      operatorv1alpha1.PhasePending,
				Conditions: []metav1.Condition{},
			},
			expected: false,
		},
	}

	op := &operatorv1alpha1.NetworkOperation{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := step.ShouldSkip(op, tc.status)
			if result != tc.expected {
				t.Errorf("ShouldSkip() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
