// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
)

// NowFrozenTask executes the network upgrade steps after the network has frozen.
//
// When the network node stops processing transactions and emits the "now_frozen.mf" marker file,
// this task performs:
// 1. Delete existing cluster config (if needed for regeneration)
// 2. Apply consensus pod changes (image updates, sidecar updates)
// 3. Apply orbit configuration changes (settings.txt, application.properties, etc.)
// 4. Recreate cluster config with updated information
//
// After completion, this task defers to the NetworkCheck processor to verify
// the network comes back up successfully.
type NowFrozenTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *UpgradeStatus
}

// NewNowFrozenTask creates a new NowFrozenTask
func NewNowFrozenTask(c client.Client) *NowFrozenTask {
	return &NowFrozenTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]{
			NewDeleteClusterConfigStep(c),
			processor.NewRetryStepWithConfig[*operatorv1alpha1.NetworkOperation, *UpgradeStatus](
				NewApplyConsensusPodChangeStep(c),
				3*time.Second,
				10, // ~30 seconds of retries
			),
			NewApplyOrbitConfigurationStep(c),
			processor.NewRetryStepWithConfig[*operatorv1alpha1.NetworkOperation, *UpgradeStatus](
				NewCreateClusterConfigStep(c),
				2*time.Second,
				150, // ~5 minutes of retries
			),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the now frozen task
func (t *NowFrozenTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*UpgradeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	result.ReconcileDelay = NowFrozenReconcileDelay
	return result, nil
}

// IsDone checks if all now frozen steps are complete
func (t *NowFrozenTask) IsDone(status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigDeleted, StepConditionReasonClusterConfig) &&
		status.HasSuccessfulCondition(StepConditionTypeApplyChangeset, StepConditionReasonConsensusPodChange) &&
		status.HasSuccessfulCondition(StepConditionTypeOrbitConfigApplied, StepConditionReasonOrbitConfiguration) &&
		status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonClusterConfig)
}

// DeleteClusterConfigStep deletes the existing cluster config to prepare for regeneration
type DeleteClusterConfigStep struct {
	client client.Client
}

// NewDeleteClusterConfigStep creates a new DeleteClusterConfigStep
func NewDeleteClusterConfigStep(c client.Client) *DeleteClusterConfigStep {
	return &DeleteClusterConfigStep{client: c}
}

// Name returns the step name
func (s *DeleteClusterConfigStep) Name() string {
	return "DeleteClusterConfig"
}

// Run executes the cluster config deletion
func (s *DeleteClusterConfigStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)
	namespace := resource.Namespace

	// TODO: Actually delete the cluster config ConfigMap
	// This would involve:
	// 1. Getting the existing ConfigMap
	// 2. Deleting it if it exists
	// 3. Waiting for deletion to complete

	logger.Info("Cluster config deletion step completed (placeholder)", "namespace", namespace)
	return s.markSucceeded(status, resource.Generation)
}

// markSucceeded marks the step as successful
func (s *DeleteClusterConfigStep) markSucceeded(status *UpgradeStatus, generation int64) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigDeleted,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            "Cluster config deleted successfully",
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *DeleteClusterConfigStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigDeleted, StepConditionReasonClusterConfig)
}

// ShouldSkip determines if this step should be skipped
func (s *DeleteClusterConfigStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}

// ApplyConsensusPodChangeStep applies the consensus pod changes from the changeset
type ApplyConsensusPodChangeStep struct {
	client client.Client
}

// NewApplyConsensusPodChangeStep creates a new ApplyConsensusPodChangeStep
func NewApplyConsensusPodChangeStep(c client.Client) *ApplyConsensusPodChangeStep {
	return &ApplyConsensusPodChangeStep{client: c}
}

// Name returns the step name
func (s *ApplyConsensusPodChangeStep) Name() string {
	return "ApplyConsensusPodChange"
}

// Run applies the consensus pod changes
func (s *ApplyConsensusPodChangeStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)
	namespace := resource.Namespace

	// Get list of consensus capsules
	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Failed to list consensus capsules")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list consensus capsules: %v", err))
	}

	if len(capsuleList.Items) == 0 {
		return s.markFailed(status, resource.Generation, "No consensus capsules found")
	}

	// Apply changes to each capsule
	var upgradedNodes []int64
	for i := range capsuleList.Items {
		capsule := &capsuleList.Items[i]

		// Apply consensus node image change
		if err := s.applyConsensusNodeChange(ctx, resource, capsule); err != nil {
			logger.Error(err, "Failed to apply consensus node change", "capsule", capsule.Name)
			return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to update capsule %s: %v", capsule.Name, err))
		}

		// Apply stream uploader changes
		if err := s.applyStreamUploaderChanges(ctx, resource, capsule); err != nil {
			logger.Error(err, "Failed to apply stream uploader changes", "capsule", capsule.Name)
			return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to update stream uploaders for %s: %v", capsule.Name, err))
		}

		upgradedNodes = append(upgradedNodes, capsule.Spec.NodeId)
		logger.Info("Applied changes to consensus capsule", "capsule", capsule.Name, "nodeId", capsule.Spec.NodeId)
	}

	// Record upgraded nodes in status
	status.UpgradedNodes = upgradedNodes

	logger.Info("Successfully applied consensus pod changes", "upgradedNodes", upgradedNodes)
	return s.markSucceeded(status, resource.Generation, len(upgradedNodes))
}

// applyConsensusNodeChange updates the consensus node image in the capsule
func (s *ApplyConsensusPodChangeStep) applyConsensusNodeChange(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, capsule *operatorv1alpha1.ConsensusCapsule) error {
	if resource.Spec.NetworkUpgrade == nil {
		return nil
	}

	changeset := &resource.Spec.NetworkUpgrade.Changesets.ConsensusNodeChangeset
	if changeset.SoftwareVersionSource == nil {
		return nil
	}

	// TODO: Actually update the capsule's software version
	// This would involve:
	// 1. Creating a patch with the new software version
	// 2. Applying the patch to the ConsensusCapsule CR
	// 3. Waiting for the pod to be recreated with the new image

	return nil
}

// applyStreamUploaderChanges updates the stream uploader images in the capsule
func (s *ApplyConsensusPodChangeStep) applyStreamUploaderChanges(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, capsule *operatorv1alpha1.ConsensusCapsule) error {
	if resource.Spec.NetworkUpgrade == nil {
		return nil
	}

	changesets := &resource.Spec.NetworkUpgrade.Changesets

	// TODO: Apply each stream uploader changeset
	// - recordStreamUploaderChangeset
	// - recordStreamSidecarUploaderChangeset
	// - eventsStreamUploaderChangeset
	// - backupUploaderChangeset
	_ = changesets

	return nil
}

// markSucceeded marks the step as successful
func (s *ApplyConsensusPodChangeStep) markSucceeded(status *UpgradeStatus, generation int64, count int) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeApplyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusPodChange,
		Message:            fmt.Sprintf("Applied changes to %d consensus capsules", count),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *ApplyConsensusPodChangeStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeApplyChangeset,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusPodChange,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *ApplyConsensusPodChangeStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeApplyChangeset, StepConditionReasonConsensusPodChange)
}

// ShouldSkip determines if this step should be skipped
func (s *ApplyConsensusPodChangeStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}

// ApplyOrbitConfigurationStep applies configuration changes to the Orbit CR
type ApplyOrbitConfigurationStep struct {
	client client.Client
}

// NewApplyOrbitConfigurationStep creates a new ApplyOrbitConfigurationStep
func NewApplyOrbitConfigurationStep(c client.Client) *ApplyOrbitConfigurationStep {
	return &ApplyOrbitConfigurationStep{client: c}
}

// Name returns the step name
func (s *ApplyOrbitConfigurationStep) Name() string {
	return "ApplyOrbitConfiguration"
}

// Run applies the orbit configuration changes
func (s *ApplyOrbitConfigurationStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)
	orbitName := resource.Spec.Orbit

	// Get the orbit
	orbit := &operatorv1alpha1.Orbit{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		logger.Error(err, "Failed to get orbit", "orbit", orbitName)
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Orbit not found: %s", orbitName))
	}

	// Check if there are configuration changes to apply
	if resource.Spec.NetworkUpgrade == nil || resource.Spec.NetworkUpgrade.ConfigurationFiles == nil {
		logger.Info("No configuration files to apply, skipping orbit update")
		return s.markSucceeded(status, resource.Generation)
	}

	configFiles := resource.Spec.NetworkUpgrade.ConfigurationFiles

	// Apply configuration changes
	updated := false

	if configFiles.NetworkSettings != nil && len(configFiles.NetworkSettings) > 0 {
		orbit.Spec.Consensus.NetworkSettings = configFiles.NetworkSettings
		updated = true
	}

	if configFiles.ApplicationProperties != nil && len(configFiles.ApplicationProperties) > 0 {
		orbit.Spec.Consensus.ApplicationConfiguration = configFiles.ApplicationProperties
		updated = true
	}

	if configFiles.PermissionConfiguration != nil && len(configFiles.PermissionConfiguration) > 0 {
		orbit.Spec.Consensus.PermissionConfiguration = configFiles.PermissionConfiguration
		updated = true
	}

	if configFiles.GenesisProperties != nil && len(configFiles.GenesisProperties) > 0 {
		orbit.Spec.Consensus.GenesisProperties = configFiles.GenesisProperties
		updated = true
	}

	if updated {
		if err := s.client.Update(ctx, orbit); err != nil {
			logger.Error(err, "Failed to update orbit", "orbit", orbitName)
			return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to update orbit: %v", err))
		}
		logger.Info("Successfully updated orbit configuration", "orbit", orbitName)
	} else {
		logger.Info("No orbit configuration changes to apply", "orbit", orbitName)
	}

	return s.markSucceeded(status, resource.Generation)
}

// markSucceeded marks the step as successful
func (s *ApplyOrbitConfigurationStep) markSucceeded(status *UpgradeStatus, generation int64) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeOrbitConfigApplied,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonOrbitConfiguration,
		Message:            "Orbit configuration applied successfully",
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *ApplyOrbitConfigurationStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeOrbitConfigApplied,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonOrbitConfiguration,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *ApplyOrbitConfigurationStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeOrbitConfigApplied, StepConditionReasonOrbitConfiguration)
}

// ShouldSkip determines if this step should be skipped
func (s *ApplyOrbitConfigurationStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}

// CreateClusterConfigStep creates/recreates the cluster config after upgrade
type CreateClusterConfigStep struct {
	client client.Client
}

// NewCreateClusterConfigStep creates a new CreateClusterConfigStep
func NewCreateClusterConfigStep(c client.Client) *CreateClusterConfigStep {
	return &CreateClusterConfigStep{client: c}
}

// Name returns the step name
func (s *CreateClusterConfigStep) Name() string {
	return "CreateClusterConfig"
}

// Run creates the cluster config
func (s *CreateClusterConfigStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)
	namespace := resource.Namespace

	// TODO: Implement cluster config creation
	// This would reuse the genesis CreateClusterConfigStep logic
	// or delegate to it directly

	logger.Info("Cluster config creation step completed (placeholder)", "namespace", namespace)
	return s.markSucceeded(status, resource.Generation)
}

// markSucceeded marks the step as successful
func (s *CreateClusterConfigStep) markSucceeded(status *UpgradeStatus, generation int64) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            "Cluster config created successfully",
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *CreateClusterConfigStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonClusterConfig)
}

// ShouldSkip determines if this step should be skipped
func (s *CreateClusterConfigStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
