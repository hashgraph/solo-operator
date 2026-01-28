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

// NodeAddStep performs the consensus node add operation
type NodeAddStep struct {
	client client.Client
}

// NewNodeAddStep creates a new NodeAddStep
func NewNodeAddStep(c client.Client) *NodeAddStep {
	return &NodeAddStep{client: c}
}

// Name returns the step name
func (s *NodeAddStep) Name() string {
	return "NodeAddStep"
}

// Run executes the node add operation
func (s *NodeAddStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) *NodeStatus {
	logger := log.FromContext(ctx)

	nodeOp := resource.Spec.NodeOperation
	if nodeOp == nil {
		return s.markFailed(status, resource.Generation, "No node operation specified")
	}

	nodeAdd := nodeOp.NodeAdd
	if nodeAdd == nil {
		return s.markFailed(status, resource.Generation, "No node add configuration specified")
	}

	logger.Info("Executing node add operation",
		"nodeName", nodeAdd.NodeName,
		"namespace", resource.Namespace)

	// TODO: Implement actual node add logic
	// This would involve:
	// 1. Creating the ConsensusCapsule CR for the new node
	// 2. Updating the Orbit CR with the new node
	// 3. Waiting for the node to be ready
	// 4. Updating network configuration

	return s.markFailed(status, resource.Generation, "Node add operation not yet implemented")
}

// markSucceeded marks the step as successful
func (s *NodeAddStep) markSucceeded(status *NodeStatus, generation int64, nodeName string) *NodeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeAdd,
		Message:            fmt.Sprintf("Successfully added consensus node: %s", nodeName),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *NodeAddStep) markFailed(status *NodeStatus, generation int64, message string) *NodeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.Message = message
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNodeOperation,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonNodeAdd,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *NodeAddStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeNodeOperation, StepConditionReasonNodeAdd)
}

// ShouldSkip determines if this step should be skipped
func (s *NodeAddStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *NodeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
