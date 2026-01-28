// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"encoding/base64"
	"fmt"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreatePublicKeysStep creates the public keys ConfigMap
type CreatePublicKeysStep struct {
	client client.Client
}

// NewCreatePublicKeysStep creates a new CreatePublicKeysStep
func NewCreatePublicKeysStep(c client.Client) *CreatePublicKeysStep {
	return &CreatePublicKeysStep{client: c}
}

// Name returns the step name
func (s *CreatePublicKeysStep) Name() string {
	return "CreatePublicKeysConfig"
}

// Run creates/updates the public keys ConfigMap
func (s *CreatePublicKeysStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
	logger := log.FromContext(ctx)

	namespace := resource.Namespace
	orbitName := resource.Spec.Orbit

	// Get the orbit
	orbit := &operatorv1alpha1.Orbit{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		logger.Error(err, "Failed to get orbit", "orbit", orbitName)
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Orbit not found: %s", orbitName))
	}

	// List consensus capsules
	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Failed to list consensus capsules")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list consensus capsules: %v", err))
	}

	if len(capsuleList.Items) == 0 {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No consensus capsules found in namespace: %s", namespace))
	}

	// Build public keys data
	data, err := s.buildPublicKeysData(ctx, namespace, capsuleList.Items, orbit)
	if err != nil {
		logger.Error(err, "Failed to build public keys data")
		return s.markFailed(status, resource.Generation, err.Error())
	}

	// Create or update the ConfigMap
	if err := s.createOrUpdateConfigMap(ctx, namespace, orbitName, data); err != nil {
		logger.Error(err, "Failed to create/update public keys ConfigMap")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to create public keys ConfigMap: %v", err))
	}

	logger.Info("Successfully created/updated public keys ConfigMap", "namespace", namespace)
	return s.markSucceeded(status, resource.Generation, namespace)
}

// buildPublicKeysData collects public keys from all capsules
func (s *CreatePublicKeysStep) buildPublicKeysData(ctx context.Context, namespace string, capsules []operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) (map[string]string, error) {
	data := make(map[string]string)

	// Collect from capsules
	for _, capsule := range capsules {
		if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
			continue
		}

		secrets := capsule.Spec.Secrets.SecretResources
		if secrets.SigningPublicCertificate.SecretName == "" {
			continue
		}

		keyRef := &secrets.SigningPublicCertificate
		secret := &corev1.Secret{}
		if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: keyRef.SecretName}, secret); err != nil {
			return nil, fmt.Errorf("secret not found: %s", keyRef.SecretName)
		}

		keyName := keyRef.KeyName
		if keyName == "" {
			keyName = "value"
		}

		encoded, ok := secret.Data[keyName]
		if !ok {
			return nil, fmt.Errorf("key %s not found in secret %s", keyName, keyRef.SecretName)
		}

		// Decode from base64 if needed (secret data is already base64 decoded by K8s)
		nodeName := prepareNodeName(capsule.Spec.NodeId)
		key := fmt.Sprintf("s-public-%s.pem", nodeName)
		data[key] = string(encoded)
	}

	// Add external addresses if present
	if orbit.Spec.Consensus.Genesis != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook != nil {

		for _, entry := range orbit.Spec.Consensus.Genesis.AddressBook.ExternalAddresses {
			if entry.PublicSigningCertificate != "" {
				key := fmt.Sprintf("s-public-%s.pem", entry.SelfName)
				// Decode if it's base64 encoded
				decoded, err := base64.StdEncoding.DecodeString(entry.PublicSigningCertificate)
				if err != nil {
					// Not base64 encoded, use as-is
					data[key] = entry.PublicSigningCertificate
				} else {
					data[key] = string(decoded)
				}
			}
		}
	}

	return data, nil
}

// createOrUpdateConfigMap creates or updates the public keys ConfigMap
func (s *CreatePublicKeysStep) createOrUpdateConfigMap(ctx context.Context, namespace, orbitName string, data map[string]string) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: PublicKeysName}, cm)

	if apierrors.IsNotFound(err) {
		// Create new ConfigMap
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PublicKeysName,
				Namespace: namespace,
				Labels:    standardLabels(PublicKeysName, orbitName),
			},
			Data: data,
		}
		logger.Info("Creating public keys ConfigMap", "namespace", namespace, "name", PublicKeysName)
		return s.client.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	// Update existing ConfigMap
	cm.Data = data
	cm.Labels = standardLabels(PublicKeysName, orbitName)
	logger.Info("Updating public keys ConfigMap", "namespace", namespace, "name", PublicKeysName)
	return s.client.Update(ctx, cm)
}

// markSucceeded marks the step as successful
func (s *CreatePublicKeysStep) markSucceeded(status *GenesisStatus, generation int64, namespace string) *GenesisStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonPublicKeyConfig,
		Message:            fmt.Sprintf("Successfully created public key config: %s/%s", namespace, PublicKeysName),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *CreatePublicKeysStep) markFailed(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonPublicKeyConfig,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *CreatePublicKeysStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonPublicKeyConfig)
}

// ShouldSkip determines if this step should be skipped
func (s *CreatePublicKeysStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}

// prepareNodeName creates a node name from a node ID
func prepareNodeName(nodeId int64) string {
	return fmt.Sprintf("node%d", nodeId)
}

// standardLabels returns standard labels for the ConfigMap
func standardLabels(name, orbitName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   orbitName,
		"app.kubernetes.io/managed-by": "solo-operator",
		"app.kubernetes.io/part-of":    "consensus-node",
		"app.kubernetes.io/component":  "config-map",
	}
}
