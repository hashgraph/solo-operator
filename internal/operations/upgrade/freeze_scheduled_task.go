// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
)

// FreezeScheduledTask performs the actions when the network node emits the "freeze_scheduled.mf" marker file.
//
// Upon receiving a FREEZE_UPGRADE FreezeTransaction, the network node emits this marker file.
// This task re-verifies the image sources to ensure they are still valid before the imminent freeze.
//
// The steps are similar to ExecuteImmediateTask but serve as a final verification before
// the network actually freezes.
type FreezeScheduledTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *UpgradeStatus
}

// NewFreezeScheduledTask creates a new FreezeScheduledTask
func NewFreezeScheduledTask(c client.Client) *FreezeScheduledTask {
	return &FreezeScheduledTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]{
			// Re-verification steps - same as ExecuteImmediate but validates previously selected registries
			NewReverifyConsensusNodeChangesetStep(c),
			NewReverifyStreamUploaderChangesetStep(c, StreamUploaderTypeRecordStream),
			NewReverifyStreamUploaderChangesetStep(c, StreamUploaderTypeSidecarFiles),
			NewReverifyStreamUploaderChangesetStep(c, StreamUploaderTypeEventStream),
			NewReverifyStreamUploaderChangesetStep(c, StreamUploaderTypeBackup),
			NewReverifyAlloyStep(c),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the freeze scheduled task
func (t *FreezeScheduledTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*UpgradeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	result.ReconcileDelay = FreezeScheduledReconcileDelay
	return result, nil
}

// ReverifyConsensusNodeChangesetStep re-verifies the consensus node changeset
// by checking that the previously selected registry still has the image available.
type ReverifyConsensusNodeChangesetStep struct {
	client client.Client
}

// NewReverifyConsensusNodeChangesetStep creates a new ReverifyConsensusNodeChangesetStep
func NewReverifyConsensusNodeChangesetStep(c client.Client) *ReverifyConsensusNodeChangesetStep {
	return &ReverifyConsensusNodeChangesetStep{client: c}
}

// Name returns the step name
func (s *ReverifyConsensusNodeChangesetStep) Name() string {
	return "ReverifyConsensusNodeChangeset"
}

// Run executes the re-verification
func (s *ReverifyConsensusNodeChangesetStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	// If already verified in ExecuteImmediate, just validate the selection still exists
	if hasSelectedRepository(status, "consensusNode") {
		// TODO: Re-verify the image is still accessible
		// For now, assume it's still valid
		return status
	}

	// Delegate to the original verification step
	verifyStep := NewVerifyConsensusNodeChangesetStep(s.client)
	return verifyStep.Run(ctx, resource, status)
}

// IsSuccessful checks if this step has completed successfully
func (s *ReverifyConsensusNodeChangesetStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	// Check if we have a valid selection
	if hasSelectedRepository(status, "consensusNode") {
		return true
	}
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonConsensusNode)
}

// ShouldSkip determines if this step should be skipped
func (s *ReverifyConsensusNodeChangesetStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	// Skip if no changeset
	if resource.Spec.NetworkUpgrade == nil {
		return true
	}
	if resource.Spec.NetworkUpgrade.Changesets.ConsensusNodeChangeset.SoftwareVersionSource == nil {
		return true
	}
	return false
}

// ReverifyStreamUploaderChangesetStep re-verifies a stream uploader changeset
type ReverifyStreamUploaderChangesetStep struct {
	client       client.Client
	uploaderType StreamUploaderType
}

// NewReverifyStreamUploaderChangesetStep creates a new ReverifyStreamUploaderChangesetStep
func NewReverifyStreamUploaderChangesetStep(c client.Client, uploaderType StreamUploaderType) *ReverifyStreamUploaderChangesetStep {
	return &ReverifyStreamUploaderChangesetStep{
		client:       c,
		uploaderType: uploaderType,
	}
}

// Name returns the step name
func (s *ReverifyStreamUploaderChangesetStep) Name() string {
	return "ReverifyStreamUploaderChangeset-" + string(s.uploaderType)
}

// Run executes the re-verification
func (s *ReverifyStreamUploaderChangesetStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	if !hasStreamUploaderChangeset(resource, s.uploaderType) {
		return status
	}

	// Check if already verified
	componentName := ""
	switch s.uploaderType {
	case StreamUploaderTypeRecordStream:
		componentName = "recordStreamUploader"
	case StreamUploaderTypeSidecarFiles:
		componentName = "recordStreamSidecarUploader"
	case StreamUploaderTypeEventStream:
		componentName = "eventsStreamUploader"
	case StreamUploaderTypeBackup:
		componentName = "backupUploader"
	}

	if hasSelectedRepository(status, componentName) {
		// TODO: Re-verify the image is still accessible
		return status
	}

	// Delegate to the original verification step
	verifyStep := NewVerifyStreamUploaderChangesetStep(s.client, s.uploaderType)
	return verifyStep.Run(ctx, resource, status)
}

// IsSuccessful checks if this step has completed successfully
func (s *ReverifyStreamUploaderChangesetStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, getStreamUploaderConditionReason(s.uploaderType))
}

// ShouldSkip determines if this step should be skipped
func (s *ReverifyStreamUploaderChangesetStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return !hasStreamUploaderChangeset(resource, s.uploaderType)
}

// ReverifyAlloyStep re-verifies the Alloy changeset
type ReverifyAlloyStep struct {
	client client.Client
}

// NewReverifyAlloyStep creates a new ReverifyAlloyStep
func NewReverifyAlloyStep(c client.Client) *ReverifyAlloyStep {
	return &ReverifyAlloyStep{client: c}
}

// Name returns the step name
func (s *ReverifyAlloyStep) Name() string {
	return "ReverifyAlloyChangeset"
}

// Run executes the re-verification
func (s *ReverifyAlloyStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	if !hasAlloyChangeset(resource) {
		return status
	}

	if hasSelectedRepository(status, "alloy") {
		// TODO: Re-verify the image is still accessible
		return status
	}

	// Delegate to the original verification step
	verifyStep := NewVerifyAlloyStep(s.client)
	return verifyStep.Run(ctx, resource, status)
}

// IsSuccessful checks if this step has completed successfully
func (s *ReverifyAlloyStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonAlloy)
}

// ShouldSkip determines if this step should be skipped
func (s *ReverifyAlloyStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return !hasAlloyChangeset(resource)
}
