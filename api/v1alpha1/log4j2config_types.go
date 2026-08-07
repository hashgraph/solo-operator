// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Log4j2ConfigSpec struct {
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

type Log4j2ConfigStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// Conditions contains the latest observations of the Log4j2Config state.
	// The "Valid" condition reflects whether spec.content is valid log4j2 XML.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty"`
}

const (
	// ConditionTypeValid is True when spec.content is valid log4j2 XML; False otherwise.
	ConditionTypeValid = "Valid"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=log4j2
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type Log4j2Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Log4j2ConfigSpec   `json:"spec,omitempty"`
	Status Log4j2ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type Log4j2ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Log4j2Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Log4j2Config{}, &Log4j2ConfigList{})
}
