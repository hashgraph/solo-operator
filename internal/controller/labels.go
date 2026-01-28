// SPDX-License-Identifier: Apache-2.0

package controller

// Standard Kubernetes label keys
const (
	// LabelManagedBy indicates the resource is managed by solo-operator
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelPartOf indicates the component the resource is part of
	LabelPartOf = "app.kubernetes.io/part-of"
	// LabelOrbit indicates the orbit name
	LabelOrbit = "operator.solo.hedera.com/orbit"
)

// Standard label values
const (
	// ManagedByValue is the value for the managed-by label
	ManagedByValue = "solo-operator"
)
