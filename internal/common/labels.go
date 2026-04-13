// SPDX-License-Identifier: Apache-2.0

package common

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
	LabelOrbit = "operator.solo.hedera.com/orbit"
)

// Standard label values.
const (
	ManagedByValue = "solo-operator"
)

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
