// SPDX-License-Identifier: Apache-2.0

package common

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Standard app.kubernetes.io label keys.
const (
	LabelName      = "app.kubernetes.io/name"
	LabelInstance  = "app.kubernetes.io/instance"
	LabelComponent = "app.kubernetes.io/component"
	LabelPartOf    = "app.kubernetes.io/part-of"
	LabelManagedBy = "app.kubernetes.io/managed-by"
)

// Hedera / solo-operator specific label keys.
const (
	LabelOrbit       = "operator.solo.hiero.org/orbit"
	LabelOperationID = "operator.solo.hiero.org/operation-id"
)

// Standard label values.
const (
	ManagedByValue = "solo-operator"

	// ComponentConsensusNode is the app.kubernetes.io/component value for consensus node pods.
	ComponentConsensusNode = "consensus-node"
)

// PropagateOperationID copies the operation-id label from src to dst if present.
func PropagateOperationID(dst map[string]string, src metav1.Object) {
	if v, ok := src.GetLabels()[LabelOperationID]; ok {
		dst[LabelOperationID] = v
	}
}

// StandardLabels returns the standard labels used on operator-managed ConfigMaps.
func StandardLabels(name, orbitName string) map[string]string {
	return map[string]string{
		LabelName:      name,
		LabelInstance:  orbitName,
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    "consensus-node",
		LabelComponent: "config-map",
	}
}
