// SPDX-License-Identifier: Apache-2.0

package check

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// StepConditionTypeNetworkCheck is the condition type for network check
	StepConditionTypeNetworkCheck = "NetworkCheck"
	// StepConditionReasonVerifyConsensus is the condition reason for verifying consensus
	StepConditionReasonVerifyConsensus = "VerifyConsensus"
)

// CheckStatus represents the status during check processing
type CheckStatus struct {
	Phase      operatorv1alpha1.Phase
	Conditions []metav1.Condition
}

// GetPhase returns the current phase
func (s *CheckStatus) GetPhase() operatorv1alpha1.Phase {
	return s.Phase
}

// SetPhase sets the phase
func (s *CheckStatus) SetPhase(phase operatorv1alpha1.Phase) {
	s.Phase = phase
}

// GetConditions returns the conditions
func (s *CheckStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

// AddCondition adds a condition to the status
func (s *CheckStatus) AddCondition(condition metav1.Condition) {
	filtered := make([]metav1.Condition, 0, len(s.Conditions))
	for _, c := range s.Conditions {
		if c.Type != condition.Type {
			filtered = append(filtered, c)
		}
	}
	s.Conditions = append(filtered, condition)
}

// HasSuccessfulCondition checks if a condition with the given type and reason is successful
func (s *CheckStatus) HasSuccessfulCondition(conditionType, reason string) bool {
	for _, c := range s.Conditions {
		if c.Type == conditionType && c.Reason == reason && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Task represents the network check task
type Task struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *CheckStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *CheckStatus
}

// NewTask creates a new check task
func NewTask(c client.Client) *Task {
	return &Task{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *CheckStatus]{
			NewVerifyConsensusCapsuleStep(c),
		},
		statusGetter: func(op *operatorv1alpha1.NetworkOperation) *CheckStatus {
			return &CheckStatus{
				Phase:      op.Status.Phase,
				Conditions: op.Status.Conditions,
			}
		},
	}
}

// Steps returns the steps for this task
func (t *Task) Steps() []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *CheckStatus] {
	return t.steps
}

// StatusGetter returns the status getter function
func (t *Task) StatusGetter() func(*operatorv1alpha1.NetworkOperation) *CheckStatus {
	return t.statusGetter
}

// Execute runs the check task
func (t *Task) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*CheckStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	return baseTask.Execute(ctx, op)
}
