// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// Label keys — alias common.
const (
	LabelManagedBy = common.LabelManagedBy
	LabelPartOf    = common.LabelPartOf
	LabelOrbit     = common.LabelOrbit
)

// Label values — alias common; orbit-specific values defined here.
const (
	ManagedByValue      = common.ManagedByValue
	PartOfConsensusNode = "consensus-node"
)

// Resource names
const (
	// ConsensusConfigMapName is the name of the consensus config map
	ConsensusConfigMapName = "consensus-config"
	// ConsensusCapsuleRoleName is the name of the consensus capsule role
	ConsensusCapsuleRoleName = "consensus-capsule-role"
)

// Port numbers — alias common.
const (
	GossipPort = common.GossipPort
	GrpcPort   = common.GrpcPort
	GrpcsPort  = common.GrpcsPort
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
