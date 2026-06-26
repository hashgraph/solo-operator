// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FeeSchedulesSpec struct {
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Set so the Orbit can cascade-delete this resource.
	// +optional
	Orbit string `json:"orbit,omitempty"`
}

type FeeSchedulesStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

const FeeSchedulesConditionTypeValid = "Valid"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=feesched
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type FeeSchedules struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeeSchedulesSpec   `json:"spec,omitempty"`
	Status FeeSchedulesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type FeeSchedulesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeeSchedules `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FeeSchedules{}, &FeeSchedulesList{})
}
