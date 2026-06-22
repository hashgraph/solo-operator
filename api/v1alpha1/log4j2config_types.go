// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Log4j2ConfigSpec defines the desired state of Log4j2Config
type Log4j2ConfigSpec struct {
	// Content is the raw log4j2.xml content. Must be a valid Log4j2 XML configuration.
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Orbit is the owning Orbit's name. Set so the Orbit can cascade-delete this resource.
	// +optional
	Orbit string `json:"orbit,omitempty"`
}

// Log4j2ConfigStatus defines the observed state of Log4j2Config
type Log4j2ConfigStatus struct {
	// ObservedGeneration is the last generation the controller acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigMapName is the name of the managed ConfigMap.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=log4j2
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Log4j2Config is the Schema for the log4j2configs API
type Log4j2Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Log4j2ConfigSpec   `json:"spec,omitempty"`
	Status Log4j2ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Log4j2ConfigList contains a list of Log4j2Config
type Log4j2ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Log4j2Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Log4j2Config{}, &Log4j2ConfigList{})
}
