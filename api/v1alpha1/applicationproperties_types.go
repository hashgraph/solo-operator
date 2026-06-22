// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationPropertiesSpec struct {
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Set so the Orbit can cascade-delete this resource.
	// +optional
	Orbit string `json:"orbit,omitempty"`
}

type ApplicationPropertiesStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=appprops
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type ApplicationProperties struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationPropertiesSpec   `json:"spec,omitempty"`
	Status ApplicationPropertiesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ApplicationPropertiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationProperties `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationProperties{}, &ApplicationPropertiesList{})
}
