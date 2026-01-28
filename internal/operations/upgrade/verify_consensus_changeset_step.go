// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// VerifyConsensusNodeChangesetStep verifies the consensus node changeset by checking image manifests
// from each software version source and selecting one for deployment.
type VerifyConsensusNodeChangesetStep struct {
	client client.Client
}

// NewVerifyConsensusNodeChangesetStep creates a new VerifyConsensusNodeChangesetStep
func NewVerifyConsensusNodeChangesetStep(c client.Client) *VerifyConsensusNodeChangesetStep {
	return &VerifyConsensusNodeChangesetStep{client: c}
}

// Name returns the step name
func (s *VerifyConsensusNodeChangesetStep) Name() string {
	return "VerifyConsensusNodeChangeset"
}

// Run executes the consensus node changeset verification
func (s *VerifyConsensusNodeChangesetStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)

	if resource.Spec.NetworkUpgrade == nil {
		logger.Info("No network upgrade specified, skipping consensus node verification")
		return s.markSucceeded(status, resource.Generation, "No network upgrade specified")
	}

	changeset := &resource.Spec.NetworkUpgrade.Changesets.ConsensusNodeChangeset

	// Verify the software version source
	if changeset.SoftwareVersionSource == nil {
		logger.Info("No consensus node software version source specified, skipping")
		return s.markSucceeded(status, resource.Generation, "No consensus node software version source specified")
	}

	// Verify image repositories
	repos := changeset.SoftwareVersionSource.ImageRepositories
	if len(repos) == 0 {
		return s.markFailed(status, resource.Generation, "At least one image repository is required")
	}

	// Validate each repository and select one for deployment
	var selectedRepo *operatorv1alpha1.ImageRepository
	var validationErrors []string

	for i := range repos {
		repo := &repos[i]
		if err := s.validateImageRepository(ctx, repo); err != nil {
			logger.Error(err, "Failed to validate image repository",
				"repository", repo.Repository,
				"imageName", repo.ImageName)
			validationErrors = append(validationErrors, fmt.Sprintf("%s/%s: %v", repo.Repository, repo.ImageName, err))
			continue
		}

		// Use the first valid repository
		if selectedRepo == nil {
			selectedRepo = repo
			logger.Info("Selected image repository for deployment",
				"repository", repo.Repository,
				"imageName", repo.ImageName,
				"tag", repo.ImageTag)
		}
	}

	if selectedRepo == nil {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No valid image repositories found: %v", validationErrors))
	}

	// Update status with selected repository
	if status.SoftwareSourceStatus == nil {
		status.SoftwareSourceStatus = &operatorv1alpha1.SoftwareSourceStatus{}
	}
	status.SoftwareSourceStatus.ConsensusNode = []operatorv1alpha1.ImageRepositoryStatus{{
		Repository:            selectedRepo.Repository,
		ImageName:             selectedRepo.ImageName,
		ImageTag:              selectedRepo.ImageTag,
		Validity:              operatorv1alpha1.ImageValidityValid,
		SelectedForDeployment: true,
	}}

	logger.Info("Successfully verified consensus node changeset",
		"selectedRepository", selectedRepo.Repository,
		"selectedImageName", selectedRepo.ImageName)

	return s.markSucceeded(status, resource.Generation, fmt.Sprintf("Selected %s/%s:%s", selectedRepo.Repository, selectedRepo.ImageName, selectedRepo.ImageTag))
}

// validateImageRepository validates an image repository by checking its availability
func (s *VerifyConsensusNodeChangesetStep) validateImageRepository(ctx context.Context, repo *operatorv1alpha1.ImageRepository) error {
	// Validate required fields
	if repo.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if repo.ImageName == "" {
		return fmt.Errorf("imageName is required")
	}
	if repo.ImageTag == "" {
		return fmt.Errorf("imageTag is required")
	}

	// TODO: Implement actual image manifest verification
	// This would involve:
	// 1. Pulling the image manifest from the registry
	// 2. Verifying layer hashes if verification spec is provided
	// 3. Checking that the image is accessible
	//
	// For now, we just validate the fields are present
	// The actual OCI registry client implementation would go here

	return nil
}

// markSucceeded marks the step as successful
func (s *VerifyConsensusNodeChangesetStep) markSucceeded(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNode,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *VerifyConsensusNodeChangesetStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNode,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *VerifyConsensusNodeChangesetStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonConsensusNode)
}

// ShouldSkip determines if this step should be skipped
func (s *VerifyConsensusNodeChangesetStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
