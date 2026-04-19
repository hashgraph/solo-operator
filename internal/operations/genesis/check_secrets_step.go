// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	// Secret key names
	GrpcTlsPrivateKey        = "GrpcTlsPrivateKey"
	GrpcTlsPublicCertificate = "GrpcTlsPublicCertificate"
	SigningPrivateKey        = "SigningPrivateKey"
	SigningPublicCertificate = "SigningPublicCertificate"
	LedgerEncryptionKey      = "LedgerEncryptionKey"
	LedgerSigningKey         = "LedgerSigningKey"
)

// CheckSecretsStep validates that all consensus node secrets are present and valid
type CheckSecretsStep struct {
	client client.Client
}

// NewCheckSecretsStep creates a new CheckSecretsStep
func NewCheckSecretsStep(c client.Client) *CheckSecretsStep {
	return &CheckSecretsStep{client: c}
}

// Name returns the step name
func (s *CheckSecretsStep) Name() string {
	return "CheckConsensusNodeSecrets"
}

// Run executes the secret validation
func (s *CheckSecretsStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
	logger := log.FromContext(ctx)

	namespace := resource.Namespace

	// List consensus capsules
	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Failed to list consensus capsules")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list consensus capsules: %v", err))
	}

	if len(capsuleList.Items) == 0 {
		logger.Info("No consensus capsules yet, will retry", "namespace", namespace)
		return status
	}

	// Validate secrets for each capsule
	for _, capsule := range capsuleList.Items {
		if err := s.validateCapsuleSecrets(ctx, namespace, &capsule); err != nil {
			logger.Error(err, "Secret validation failed", "capsule", capsule.Name)
			return s.markFailed(status, resource.Generation, err.Error())
		}
	}

	// All secrets validated successfully
	logger.Info("Successfully validated consensus node secrets", "count", len(capsuleList.Items))
	return s.markSucceeded(status, resource.Generation)
}

// validateCapsuleSecrets validates the secrets for a single consensus capsule
func (s *CheckSecretsStep) validateCapsuleSecrets(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) error {
	if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
		// No secrets configured - that's OK
		return nil
	}

	secrets := capsule.Spec.Secrets.SecretResources

	// Validate each configured secret (non-optional fields)
	if err := s.validateSecretRef(ctx, namespace, &secrets.GrpcTlsPrivateKey, GrpcTlsPrivateKey, capsule.Name); err != nil {
		return err
	}

	if err := s.validateSecretRef(ctx, namespace, &secrets.GrpcTlsPublicCertificate, GrpcTlsPublicCertificate, capsule.Name); err != nil {
		return err
	}

	if err := s.validateSecretRef(ctx, namespace, &secrets.SigningPrivateKey, SigningPrivateKey, capsule.Name); err != nil {
		return err
	}

	if err := s.validateSecretRef(ctx, namespace, &secrets.SigningPublicCertificate, SigningPublicCertificate, capsule.Name); err != nil {
		return err
	}

	// Optional secrets
	if secrets.LedgerEncryptionKey != nil {
		if err := s.validateSecret(ctx, namespace, secrets.LedgerEncryptionKey, LedgerEncryptionKey, capsule.Name); err != nil {
			return err
		}
	}

	if secrets.LedgerSigningKey != nil {
		if err := s.validateSecret(ctx, namespace, secrets.LedgerSigningKey, LedgerSigningKey, capsule.Name); err != nil {
			return err
		}
	}

	return nil
}

// validateSecret checks that a secret exists and contains the required key (for pointer refs)
func (s *CheckSecretsStep) validateSecret(ctx context.Context, namespace string, keyRef *operatorv1alpha1.KeyReference, keyType, capsuleName string) error {
	if keyRef == nil {
		return nil
	}
	return s.validateSecretRef(ctx, namespace, keyRef, keyType, capsuleName)
}

// validateSecretRef checks that a secret exists and contains the required key
func (s *CheckSecretsStep) validateSecretRef(ctx context.Context, namespace string, keyRef *operatorv1alpha1.KeyReference, keyType, capsuleName string) error {
	if keyRef == nil || keyRef.SecretName == "" {
		return nil
	}

	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: keyRef.SecretName}, secret); err != nil {
		return fmt.Errorf("%s secret not found for capsule %s: %s", keyType, capsuleName, keyRef.SecretName)
	}

	keyName := keyRef.KeyName
	if keyName == "" {
		keyName = "value" // default key name
	}

	if _, ok := secret.Data[keyName]; !ok {
		return fmt.Errorf("%s secret %s does not contain key %s for capsule %s", keyType, keyRef.SecretName, keyName, capsuleName)
	}

	return nil
}

// markSucceeded marks the step as successful
func (s *CheckSecretsStep) markSucceeded(status *GenesisStatus, generation int64) *GenesisStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifySecrets,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNodeSecrets,
		Message:            "Successfully validated consensus node secrets",
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *CheckSecretsStep) markFailed(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeVerifySecrets,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNodeSecrets,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *CheckSecretsStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeVerifySecrets, StepConditionReasonConsensusNodeSecrets)
}

// ShouldSkip determines if this step should be skipped
func (s *CheckSecretsStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
