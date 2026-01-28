// SPDX-License-Identifier: Apache-2.0

package orbit

import (
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
)

// Label values
const (
	// ManagedByValue is the value for the managed-by label
	ManagedByValue = "solo-operator"
	// PartOfConsensusNode is the value for consensus node part-of label
	PartOfConsensusNode = "consensus-node"
)

// Resource names
const (
	// ConsensusConfigMapName is the name of the consensus config map
	ConsensusConfigMapName = "consensus-config"
	// ConsensusCapsuleRoleName is the name of the consensus capsule role
	ConsensusCapsuleRoleName = "consensus-capsule-role"
	// ConsensusServiceName is the name of the consensus headless service
	ConsensusServiceName = "consensus"
)

// Port numbers
const (
	GossipPort = 50111
	GrpcPort   = 50211
	GrpcsPort  = 50212
)

// StandardLabels returns the standard labels for orbit-managed resources
func StandardLabels(orbit *operatorv1alpha1.Orbit) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelOrbit:     orbit.Name,
	}
}

// NewSubReconcilers creates all orbit subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.Orbit] {
	return subreconciler.NewChain(
		NewNamespaceSubReconciler(c, scheme),
		NewConfigMapSubReconciler(c, scheme),
		NewRoleSubReconciler(c, scheme),
		NewHeadlessServiceSubReconciler(c, scheme),
	)
}
