// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateClusterConfigStep creates the cluster configuration ConfigMap
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

// Run creates/updates the cluster configuration ConfigMap
func (s *CreateClusterConfigStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
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

	// List pods to get IPs
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/managed-by": "solo-operator",
			"app.kubernetes.io/part-of":    "consensus-node",
			"app.kubernetes.io/component":  "deployment",
		},
	}
	if err := s.client.List(ctx, podList, listOpts...); err != nil {
		logger.Error(err, "Failed to list pods")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list pods: %v", err))
	}

	// Collect ready pod IPs
	podIPs := s.collectPodIPs(podList)
	if len(podIPs) != len(capsuleList.Items) {
		logger.Info("Not all pods are ready",
			"expected", len(capsuleList.Items),
			"found", len(podIPs))
		// Don't mark as failed - this is a retry condition
		return status
	}

	// Build config.txt content
	configTxt := s.buildConfigTxt(capsuleList.Items, orbit, namespace)

	// Build data map
	data := map[string]string{
		ConfigKeyConfigTxt: configTxt,
		ConfigKeyPodIPs:    strings.Join(podIPs, "\n"),
	}

	// Create or update the ConfigMap
	if err := s.createOrUpdateConfigMap(ctx, namespace, orbitName, data); err != nil {
		logger.Error(err, "Failed to create/update cluster config ConfigMap")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to create cluster config ConfigMap: %v", err))
	}

	logger.Info("Successfully created/updated cluster config ConfigMap", "namespace", namespace)
	return s.markSucceeded(status, resource.Generation, namespace)
}

// collectPodIPs collects IPs from ready pods
func (s *CreateClusterConfigStep) collectPodIPs(podList *corev1.PodList) []string {
	var ips []string
	for _, pod := range podList.Items {
		// Skip pods being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Check if pod is running
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check if pod is ready but not fully ready (transitioning state we need)
		// The Java implementation checks for Ready=False while Running
		isReady := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionFalse {
				isReady = true
				break
			}
		}

		if !isReady {
			continue
		}

		if pod.Status.PodIP != "" {
			ips = append(ips, pod.Status.PodIP)
		}
	}
	return ips
}

// buildConfigTxt builds the config.txt content for the Hedera network
func (s *CreateClusterConfigStep) buildConfigTxt(capsules []operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit, namespace string) string {
	var sb strings.Builder

	// Get ledger ID from orbit
	ledgerId := "123" // default
	if orbit.Spec.Consensus.Genesis != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook.LedgerId != "" {
		ledgerId = orbit.Spec.Consensus.Genesis.AddressBook.LedgerId
	}

	sb.WriteString(fmt.Sprintf("swirld, %s\n", ledgerId))
	sb.WriteString("app, HederaNode.jar\n")

	// Build address entries from capsules (sorted by node ID)
	addressMap := make(map[int64]string)

	for _, capsule := range capsules {
		nodeId := capsule.Spec.NodeId
		nodeName := prepareNodeName(nodeId)

		internalAddr := s.computeNodeInternalAddress(&capsule, namespace)
		externalAddr := s.computeNodeExternalAddress(&capsule, nodeName, namespace)

		weight := int64(capsule.Spec.Weight)
		if weight == 0 {
			weight = 1
		}

		accountId := capsule.Spec.AccountId
		if accountId == "" {
			accountId = fmt.Sprintf("0.0.%d", 3+nodeId) // default account
		}

		// Format: address, nodeId, name, name, weight, internalHost, 50111, externalHost, 50111, accountId
		addressMap[nodeId] = fmt.Sprintf("address, %d, %s, %s, %d, %s, 50111, %s, 50111, %s\n",
			nodeId, nodeName, nodeName, weight, internalAddr, externalAddr, accountId)
	}

	// Add external addresses from orbit if present
	if orbit.Spec.Consensus.Genesis != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook != nil {

		for _, entry := range orbit.Spec.Consensus.Genesis.AddressBook.ExternalAddresses {
			nodeId := entry.NodeId
			addressMap[nodeId] = fmt.Sprintf("address, %d, %s, %s, %d, %s, %d, %s, %d, %s\n",
				nodeId,
				entry.NickName,
				entry.SelfName,
				entry.Weight,
				entry.InternalHostname,
				entry.InternalPort,
				entry.ExternalHostname,
				entry.ExternalPort,
				entry.NodeAccount)
		}
	}

	// Sort by node ID and append
	var nodeIds []int64
	for id := range addressMap {
		nodeIds = append(nodeIds, id)
	}
	sort.Slice(nodeIds, func(i, j int) bool { return nodeIds[i] < nodeIds[j] })

	for _, id := range nodeIds {
		sb.WriteString(addressMap[id])
	}

	return sb.String()
}

// computeNodeInternalAddress computes the internal address for a consensus node
func (s *CreateClusterConfigStep) computeNodeInternalAddress(capsule *operatorv1alpha1.ConsensusCapsule, namespace string) string {
	if capsule.Spec.InternalAddress != "" {
		return capsule.Spec.InternalAddress
	}
	// Default: pod-name.headless-svc.namespace.svc.cluster.local
	return fmt.Sprintf("%s.consensus.%s.svc.cluster.local", capsule.Name, namespace)
}

// computeNodeExternalAddress computes the external address for a consensus node
func (s *CreateClusterConfigStep) computeNodeExternalAddress(capsule *operatorv1alpha1.ConsensusCapsule, nodeName, namespace string) string {
	if capsule.Spec.ExternalAddress != "" {
		return capsule.Spec.ExternalAddress
	}
	// Default: node-svc.namespace
	return fmt.Sprintf("%s-svc.%s", nodeName, namespace)
}

// createOrUpdateConfigMap creates or updates the cluster config ConfigMap
func (s *CreateClusterConfigStep) createOrUpdateConfigMap(ctx context.Context, namespace, orbitName string, data map[string]string) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ClusterConfigName}, cm)

	if apierrors.IsNotFound(err) {
		// Create new ConfigMap
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterConfigName,
				Namespace: namespace,
				Labels:    standardLabels(ClusterConfigName, orbitName),
			},
			Data: data,
		}
		logger.Info("Creating cluster config ConfigMap", "namespace", namespace, "name", ClusterConfigName)
		return s.client.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	// Update existing ConfigMap
	cm.Data = data
	cm.Labels = standardLabels(ClusterConfigName, orbitName)
	logger.Info("Updating cluster config ConfigMap", "namespace", namespace, "name", ClusterConfigName)
	return s.client.Update(ctx, cm)
}

// markSucceeded marks the step as successful
func (s *CreateClusterConfigStep) markSucceeded(status *GenesisStatus, generation int64, namespace string) *GenesisStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            fmt.Sprintf("Successfully created cluster config: %s/%s", namespace, ClusterConfigName),
		ObservedGeneration: generation,
	})
	return status
}

// markFailed marks the step as failed
func (s *CreateClusterConfigStep) markFailed(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *CreateClusterConfigStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonClusterConfig)
}

// ShouldSkip determines if this step should be skipped
func (s *CreateClusterConfigStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
