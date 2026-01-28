// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvoyProxySpec defines the desired state of EnvoyProxy
type EnvoyProxySpec struct {
	// Image is the Envoy proxy image to use
	// +kubebuilder:default="envoyproxy/envoy:v1.21.6"
	// +optional
	Image string `json:"image,omitempty"`

	// Env contains environment variables for the container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Replicas is the number of Envoy proxy replicas
	// +kubebuilder:default=2
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ProxyPort is the port for incoming traffic
	// +kubebuilder:default=8081
	// +optional
	ProxyPort int32 `json:"proxyPort,omitempty"`

	// AdminPort is the admin interface port
	// +kubebuilder:default=9901
	// +optional
	AdminPort int32 `json:"adminPort,omitempty"`

	// Files is a map of file names to file contents to mount in the container
	// +optional
	Files map[string]string `json:"files,omitempty"`
}

// EnvoyProxyStatus defines the observed state of EnvoyProxy
type EnvoyProxyStatus struct {
	// ObservedGeneration is the generation observed by the operator
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the total number of replicas
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// AvailableReplicas is the number of available replicas
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ReadyReplicas is the number of ready replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// TerminatingReplicas is the number of terminating replicas
	// +optional
	TerminatingReplicas int32 `json:"terminatingReplicas,omitempty"`

	// UnavailableReplicas is the number of unavailable replicas
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// UpdatedReplicas is the number of updated replicas
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Conditions are the deployment conditions
	// +optional
	Conditions []appsv1.DeploymentCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=envoy
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EnvoyProxy is the Schema for the envoyproxies API
type EnvoyProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvoyProxySpec   `json:"spec,omitempty"`
	Status EnvoyProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvoyProxyList contains a list of EnvoyProxy
type EnvoyProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvoyProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EnvoyProxy{}, &EnvoyProxyList{})
}
