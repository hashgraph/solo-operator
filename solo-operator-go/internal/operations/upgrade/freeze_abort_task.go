// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FreezeAbortTask handles the abort of a network upgrade.
//
// When a FREEZE_ABORT FreezeTransaction is sent, the network node emits the "freeze_aborted.mf"
// marker file. This task performs any necessary cleanup to discard the current upgrade cycle.
//
// In practice, this is essentially a no-op since the network nodes haven't been impacted
// (frozen or upgraded) yet. However, the operator may need to do some housekeeping.
type FreezeAbortTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *UpgradeStatus
}

// NewFreezeAbortTask creates a new FreezeAbortTask
func NewFreezeAbortTask(c client.Client) *FreezeAbortTask {
	return &FreezeAbortTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]{
			NewFreezeAbortStep(c),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the freeze abort task
func (t *FreezeAbortTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*UpgradeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	result.ReconcileDelay = FreezeAbortReconcileDelay
	return result, nil
}

// FreezeAbortStep performs the freeze abort cleanup
type FreezeAbortStep struct {
	client client.Client
}

// NewFreezeAbortStep creates a new FreezeAbortStep
func NewFreezeAbortStep(c client.Client) *FreezeAbortStep {
	return &FreezeAbortStep{client: c}
}

// Name returns the step name
func (s *FreezeAbortStep) Name() string {
	return "FreezeAbort"
}

// Run executes the freeze abort cleanup
func (s *FreezeAbortStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)

	logger.Info("Processing freeze abort",
		"namespace", resource.Namespace,
		"name", resource.Name)

	// TODO: Implement any necessary cleanup
	// This might include:
	// 1. Clearing any cached software source selections
	// 2. Resetting any firewall configurations
	// 3. Removing any temporary resources created for the upgrade
	//
	// For now, this is essentially a no-op since the network hasn't been
	// impacted yet when freeze abort is triggered.

	// Clear software source status if present
	status.SoftwareSourceStatus = nil

	logger.Info("Freeze abort completed successfully",
		"namespace", resource.Namespace,
		"name", resource.Name)

	return s.markSucceeded(status, resource.Generation)
}

// markSucceeded marks the step as successful
func (s *FreezeAbortStep) markSucceeded(status *UpgradeStatus, generation int64) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeFreezeAbort,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonFreezeAborted,
		Message:            "Freeze abort completed successfully - upgrade cycle discarded",
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *FreezeAbortStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeFreezeAbort,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonFreezeAborted,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *FreezeAbortStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeFreezeAbort, StepConditionReasonFreezeAborted)
}

// ShouldSkip determines if this step should be skipped
func (s *FreezeAbortStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
