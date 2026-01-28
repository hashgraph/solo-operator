// SPDX-License-Identifier: Apache-2.0

package check

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// VerifyConsensusCapsuleStep verifies that all consensus capsules are running
type VerifyConsensusCapsuleStep struct {
	client client.Client
}

// NewVerifyConsensusCapsuleStep creates a new VerifyConsensusCapsuleStep
func NewVerifyConsensusCapsuleStep(c client.Client) *VerifyConsensusCapsuleStep {
	return &VerifyConsensusCapsuleStep{client: c}
}

// Name returns the step name
func (s *VerifyConsensusCapsuleStep) Name() string {
	return "VerifyConsensusCapsules"
}

// Run executes the consensus capsule verification
func (s *VerifyConsensusCapsuleStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *CheckStatus) *CheckStatus {
	logger := log.FromContext(ctx)

	namespace := resource.Namespace

	// List consensus capsules
	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Failed to list consensus capsules")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list consensus capsules: %v", err))
	}

	if len(capsuleList.Items) == 0 {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No consensus capsules found in namespace: %s", namespace))
	}

	// Check that all capsules are running
	allRunning := true
	var notRunning []string
	for _, capsule := range capsuleList.Items {
		if capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
			allRunning = false
			notRunning = append(notRunning, fmt.Sprintf("%s (phase: %s)", capsule.Name, capsule.Status.Phase))
		}
	}

	if !allRunning {
		logger.Info("Not all consensus capsules are running", "notRunning", notRunning)
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Not all consensus capsules are running: %v", notRunning))
	}

	logger.Info("All consensus capsules are running", "count", len(capsuleList.Items))
	return s.markSucceeded(status, resource.Generation, len(capsuleList.Items))
}

// markSucceeded marks the step as successful
func (s *VerifyConsensusCapsuleStep) markSucceeded(status *CheckStatus, generation int64, count int) *CheckStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNetworkCheck,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonVerifyConsensus,
		Message:            fmt.Sprintf("All %d consensus capsules are running", count),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *VerifyConsensusCapsuleStep) markFailed(status *CheckStatus, generation int64, message string) *CheckStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeNetworkCheck,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonVerifyConsensus,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *VerifyConsensusCapsuleStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *CheckStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeNetworkCheck, StepConditionReasonVerifyConsensus)
}

// ShouldSkip determines if this step should be skipped
func (s *VerifyConsensusCapsuleStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *CheckStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
