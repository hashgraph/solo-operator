// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationOverridePropertiesSpec struct {
	// Scope determines the ConfigMap name prefix. The reconciler builds the
	// ConfigMap name as scope + "-" + <kind-suffix>. Must match "node" followed
	// by one or more digits (e.g. node0, node1).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^node\d+$`
	Scope string `json:"scope"`

	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Set so the Orbit can cascade-delete this resource.
	// +optional
	Orbit string `json:"orbit,omitempty"`
}

type ApplicationOverridePropertiesStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

const ApplicationOverridePropertiesConditionTypeValid = "Valid"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=appoverprops
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type ApplicationOverrideProperties struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationOverridePropertiesSpec   `json:"spec,omitempty"`
	Status ApplicationOverridePropertiesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ApplicationOverridePropertiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationOverrideProperties `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationOverrideProperties{}, &ApplicationOverridePropertiesList{})
}
