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

// VerifyStreamUploaderChangesetStep verifies a stream uploader changeset by checking image manifests
// from each software version source and selecting one for deployment.
type VerifyStreamUploaderChangesetStep struct {
	client       client.Client
	uploaderType StreamUploaderType
}

// NewVerifyStreamUploaderChangesetStep creates a new VerifyStreamUploaderChangesetStep
func NewVerifyStreamUploaderChangesetStep(c client.Client, uploaderType StreamUploaderType) *VerifyStreamUploaderChangesetStep {
	return &VerifyStreamUploaderChangesetStep{
		client:       c,
		uploaderType: uploaderType,
	}
}

// Name returns the step name
func (s *VerifyStreamUploaderChangesetStep) Name() string {
	return fmt.Sprintf("VerifyStreamUploaderChangeset-%s", s.uploaderType)
}

// Run executes the stream uploader changeset verification
func (s *VerifyStreamUploaderChangesetStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) *UpgradeStatus {
	logger := log.FromContext(ctx)

	if !hasStreamUploaderChangeset(resource, s.uploaderType) {
		logger.Info("No changeset specified for stream uploader, skipping", "uploaderType", s.uploaderType)
		return s.markSucceeded(status, resource.Generation, "No changeset specified")
	}

	changeset := s.getChangeset(resource)
	if changeset == nil {
		return s.markSucceeded(status, resource.Generation, "No changeset specified")
	}

	// Verify the software version source
	if changeset.SoftwareVersionSource == nil {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("%s changeset requires a softwareVersionSource", s.uploaderType))
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
				"uploaderType", s.uploaderType,
				"repository", repo.Repository,
				"imageName", repo.ImageName)
			validationErrors = append(validationErrors, fmt.Sprintf("%s/%s: %v", repo.Repository, repo.ImageName, err))
			continue
		}

		// Use the first valid repository
		if selectedRepo == nil {
			selectedRepo = repo
			logger.Info("Selected image repository for stream uploader",
				"uploaderType", s.uploaderType,
				"repository", repo.Repository,
				"imageName", repo.ImageName,
				"tag", repo.ImageTag)
		}
	}

	if selectedRepo == nil {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No valid image repositories found for %s: %v", s.uploaderType, validationErrors))
	}

	// Update status with selected repository
	if status.SoftwareSourceStatus == nil {
		status.SoftwareSourceStatus = &operatorv1alpha1.SoftwareSourceStatus{}
	}

	repoStatus := []operatorv1alpha1.ImageRepositoryStatus{{
		Repository:            selectedRepo.Repository,
		ImageName:             selectedRepo.ImageName,
		ImageTag:              selectedRepo.ImageTag,
		Validity:              operatorv1alpha1.ImageValidityValid,
		SelectedForDeployment: true,
	}}

	switch s.uploaderType {
	case StreamUploaderTypeRecordStream:
		status.SoftwareSourceStatus.RecordStreamUploader = repoStatus
	case StreamUploaderTypeSidecarFiles:
		status.SoftwareSourceStatus.RecordStreamSidecarUploader = repoStatus
	case StreamUploaderTypeEventStream:
		status.SoftwareSourceStatus.EventsStreamUploader = repoStatus
	case StreamUploaderTypeBackup:
		status.SoftwareSourceStatus.BackupUploader = repoStatus
	}

	logger.Info("Successfully verified stream uploader changeset",
		"uploaderType", s.uploaderType,
		"selectedRepository", selectedRepo.Repository,
		"selectedImageName", selectedRepo.ImageName)

	return s.markSucceeded(status, resource.Generation, fmt.Sprintf("Selected %s/%s:%s", selectedRepo.Repository, selectedRepo.ImageName, selectedRepo.ImageTag))
}

// getChangeset returns the stream uploader changeset for this step's uploader type
func (s *VerifyStreamUploaderChangesetStep) getChangeset(resource *operatorv1alpha1.NetworkOperation) *operatorv1alpha1.StreamUploaderChangeset {
	if resource.Spec.NetworkUpgrade == nil {
		return nil
	}

	changesets := &resource.Spec.NetworkUpgrade.Changesets
	switch s.uploaderType {
	case StreamUploaderTypeRecordStream:
		return changesets.RecordStreamUploaderChangeset
	case StreamUploaderTypeSidecarFiles:
		return changesets.RecordStreamSidecarUploaderChangeset
	case StreamUploaderTypeEventStream:
		return changesets.EventsStreamUploaderChangeset
	case StreamUploaderTypeBackup:
		return changesets.BackupUploaderChangeset
	}
	return nil
}

// validateImageRepository validates an image repository
func (s *VerifyStreamUploaderChangesetStep) validateImageRepository(ctx context.Context, repo *operatorv1alpha1.ImageRepository) error {
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
func (s *VerifyStreamUploaderChangesetStep) markSucceeded(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             getStreamUploaderConditionReason(s.uploaderType),
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *VerifyStreamUploaderChangesetStep) markFailed(status *UpgradeStatus, generation int64, message string) *UpgradeStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             getStreamUploaderConditionReason(s.uploaderType),
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *VerifyStreamUploaderChangesetStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, getStreamUploaderConditionReason(s.uploaderType))
}

// ShouldSkip determines if this step should be skipped
func (s *VerifyStreamUploaderChangesetStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	// Skip if no changeset or already successful
	if !hasStreamUploaderChangeset(resource, s.uploaderType) {
		return true
	}
	return s.IsSuccessful(resource, status)
}
