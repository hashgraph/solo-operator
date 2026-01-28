// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// Label keys
const (
	// LabelManagedBy indicates the resource is managed by solo-operator
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelPartOf indicates the component the resource is part of
	LabelPartOf = "app.kubernetes.io/part-of"
	// LabelOrbit indicates the orbit name
	LabelOrbit = "operator.solo.hedera.com/orbit"
	// LabelNodeId indicates the node ID
	LabelNodeId = "operator.solo.hedera.com/node-id"
	// LabelComponent indicates the component type
	LabelComponent = "app.kubernetes.io/component"
)

// Label values
const (
	// ManagedByValue is the value for the managed-by label
	ManagedByValue = "solo-operator"
	// PartOfConsensusNode is the value for consensus node part-of label
	PartOfConsensusNode = "consensus-node"
	// ComponentDeployment is the component value for deployment
	ComponentDeployment = "deployment"
)

// Resource name suffixes
const (
	ServiceAccountSuffix = "-sa"
	RoleBindingSuffix    = "-rb"
	ConfigMapSuffix      = "-cm"
)

// Resource names
const (
	// ConsensusCapsuleRoleName is the name of the role created by Orbit
	ConsensusCapsuleRoleName = "consensus-capsule-role"
	// ConsensusConfigMapName is the name of the ConfigMap created by Orbit
	ConsensusConfigMapName = "consensus-config"
	// DeploymentName is the name of the deployment
	DeploymentName = "consensus-node"
	// ConsensusSubdomain is the subdomain for consensus nodes
	ConsensusSubdomain = "consensus"
)

// Port numbers
const (
	GossipPort     = 50111
	GrpcPort       = 50211
	GrpcsPort      = 50212
	PrometheusPort = 9999
	DebugPort      = 5005
)

// CommonLabels returns the common labels for consensus capsule resources
func CommonLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfConsensusNode,
		LabelOrbit:     capsule.Spec.Orbit,
		LabelNodeId:    fmt.Sprintf("%d", capsule.Spec.NodeId),
	}
}

// PodLabels returns the labels for pods
func PodLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	labels := CommonLabels(capsule)
	labels[LabelComponent] = ComponentDeployment
	return labels
}

// ServiceAccountName returns the service account name for a capsule
func ServiceAccountName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + ServiceAccountSuffix
}

// RoleBindingName returns the role binding name for a capsule
func RoleBindingName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + RoleBindingSuffix
}

// ConfigMapName returns the config map name for a capsule
func ConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + ConfigMapSuffix
}

// PVCName returns the PVC name for a capsule and volume type
func PVCName(capsule *operatorv1alpha1.ConsensusCapsule, volumeType string) string {
	return fmt.Sprintf("%s-%s", capsule.Name, volumeType)
}

// InternalHostname returns the internal FQDN for a capsule
func InternalHostname(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local", capsule.Name, ConsensusSubdomain, capsule.Namespace)
}

// NewSubReconcilers creates all consensus capsule subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.ConsensusCapsule] {
	return subreconciler.NewChain(
		NewServiceAccountSubReconciler(c, scheme),
		NewRoleBindingSubReconciler(c, scheme),
		NewConfigMapSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewPVCSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
	)
}
