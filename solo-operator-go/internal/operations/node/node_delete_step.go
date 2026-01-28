// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NodeDeleteStep performs the consensus node delete operation
type NodeDeleteStep struct {
	client client.Client
}

// NewNodeDeleteStep creates a new NodeDeleteStep
func NewNodeDeleteStep(c client.Client) *NodeDeleteStep {
	return &NodeDeleteStep{client: c}
}

// Name returns the step name
func (s *NodeDeleteStep) Name() string {
	return "NodeDeleteStep"
}

// Run executes the node delete operation
func (s *NodeDeleteStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) *NodeStatus {
	logger := log.FromContext(ctx)

	nodeOp := resource.Spec.NodeOperation
	if nodeOp == nil {
		return s.markFailed(status, resource.Generation, "No node operation specified")
	}

	nodeDelete := nodeOp.NodeDelete
	if nodeDelete == nil {
		return s.markFailed(status, resource.Generation, "No node delete configuration specified")
	}

	logger.Info("Executing node delete operation",
		"nodeName", nodeDelete.NodeName,
		"namespace", resource.Namespace)

	// TODO: Implement actual node delete logic
	// This would involve:
	// 1. Finding the existing ConsensusCapsule CR
	// 2. Removing the node from network configuration
	// 3. Deleting the ConsensusCapsule CR
	// 4. Updating Orbit CR
	// 5. Cleaning up associated resources

	return s.markFailed(status, resource.Generation, "Node delete operation not yet implemented")
}

// markSucceeded marks the step as successful
func (s *NodeDeleteStep) markSucceeded(status *NodeStatus, generation int64, nodeName string) *NodeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeDelete,
		Message:            fmt.Sprintf("Successfully deleted consensus node: %s", nodeName),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *NodeDeleteStep) markFailed(status *NodeStatus, generation int64, message string) *NodeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.Message = message
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeDelete,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *NodeDeleteStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeNodeOperation, StepConditionReasonNodeDelete)
}

// ShouldSkip determines if this step should be skipped
func (s *NodeDeleteStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
