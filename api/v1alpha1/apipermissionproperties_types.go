// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApiPermissionPropertiesSpec struct {
	// Scope determines the ConfigMap name prefix. The reconciler builds the
	// ConfigMap name as scope + "-" + <kind-suffix>. Must match "node" followed
	// by one or more digits (e.g. node0, node1).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^node\d+$`
	Scope string `json:"scope"`

	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Orbit is the owning Orbit's name.
	// +kubebuilder:validation:MinLength=1
	Orbit string `json:"orbit"`
}

type ApiPermissionPropertiesStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

const ApiPermissionPropertiesConditionTypeValid = "Valid"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=apipermprops
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type ApiPermissionProperties struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApiPermissionPropertiesSpec   `json:"spec,omitempty"`
	Status ApiPermissionPropertiesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ApiPermissionPropertiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiPermissionProperties `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApiPermissionProperties{}, &ApiPermissionPropertiesList{})
}
