// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// VerifyAlloyStep verifies the Grafana Alloy changeset
type VerifyAlloyStep struct {
	client client.Client
}

// NewVerifyAlloyStep creates a new VerifyAlloyStep
func NewVerifyAlloyStep(c client.Client) *VerifyAlloyStep {
	return &VerifyAlloyStep{client: c}
}

// Name returns the step name
func (s *VerifyAlloyStep) Name() string {
	return "VerifyAlloyChangeset"
}

// Run executes the Alloy changeset verification
func (s *VerifyAlloyStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)

	if !hasAlloyChangeset(resource) {
		logger.Info("No Alloy changeset specified, skipping")
		return s.markSucceeded(status, resource.Generation, "No changeset specified")
	}

	changeset := resource.Spec.NetworkUpgrade.Changesets.Alloy
	if changeset == nil {
		return s.markSucceeded(status, resource.Generation, "No changeset specified")
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
			logger.Info("Selected image repository for Alloy",
				"repository", repo.Repository,
				"imageName", repo.ImageName,
				"tag", repo.ImageTag)
		}
	}

	if selectedRepo == nil {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No valid image repositories found for Alloy: %v", validationErrors))
	}

	// Update status with selected repository
	if status.SoftwareSourceStatus == nil {
		status.SoftwareSourceStatus = &operatorv1alpha1.SoftwareSourceStatus{}
	}
	status.SoftwareSourceStatus.Alloy = []operatorv1alpha1.ImageRepositoryStatus{{
		Repository:            selectedRepo.Repository,
		ImageName:             selectedRepo.ImageName,
		ImageTag:              selectedRepo.ImageTag,
		Validity:              operatorv1alpha1.ImageValidityValid,
		SelectedForDeployment: true,
	}}

	logger.Info("Successfully verified Alloy changeset",
		"selectedRepository", selectedRepo.Repository,
		"selectedImageName", selectedRepo.ImageName)

	return s.markSucceeded(status, resource.Generation, fmt.Sprintf("Selected %s/%s:%s", selectedRepo.Repository, selectedRepo.ImageName, selectedRepo.ImageTag))
}

// validateImageRepository validates an image repository
func (s *VerifyAlloyStep) validateImageRepository(ctx context.Context, repo *operatorv1alpha1.ImageRepository) error {
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
	return nil
}

// markSucceeded marks the step as successful
func (s *VerifyAlloyStep) markSucceeded(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonAlloy,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *VerifyAlloyStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonAlloy,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *VerifyAlloyStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonAlloy)
}

// ShouldSkip determines if this step should be skipped
func (s *VerifyAlloyStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	// Skip if no changeset or already successful
	if !hasAlloyChangeset(resource) {
		return true
	}
	return s.IsSuccessful(resource, status)
}
