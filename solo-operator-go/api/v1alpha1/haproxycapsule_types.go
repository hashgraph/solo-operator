// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HAProxyServiceType represents the service type for HAProxy
// +kubebuilder:validation:Enum=ClusterIP;Ingress;LoadBalancer
type HAProxyServiceType string

const (
	HAProxyServiceTypeClusterIP    HAProxyServiceType = "ClusterIP"
	HAProxyServiceTypeIngress      HAProxyServiceType = "Ingress"
	HAProxyServiceTypeLoadBalancer HAProxyServiceType = "LoadBalancer"
)

// HAProxyCapsuleSpec defines the desired state of HAProxyCapsule
type HAProxyCapsuleSpec struct {
	// Image is the HAProxy image to use
	// +kubebuilder:default="haproxy:2.8.10-alpine3.20"
	// +optional
	Image string `json:"image,omitempty"`

	// Env contains environment variables for the container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// SecretName is the name of the secret containing "hedera.pem"
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// ServiceType specifies the service type
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType HAProxyServiceType `json:"serviceType,omitempty"`

	// ServiceSpec is the full service specification
	// +optional
	ServiceSpec *corev1.ServiceSpec `json:"serviceSpec,omitempty"`

	// IngressSpec is the ingress specification when using Ingress service type
	// +optional
	IngressSpec *networkingv1.IngressSpec `json:"ingressSpec,omitempty"`
}

// HAProxyCapsuleStatus defines the observed state of HAProxyCapsule
type HAProxyCapsuleStatus struct {
	// Phase is the current phase
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// ObservedGeneration is the generation observed by the operator
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=haproxy
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HAProxyCapsule is the Schema for the haproxycapsules API
type HAProxyCapsule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HAProxyCapsuleSpec   `json:"spec,omitempty"`
	Status HAProxyCapsuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HAProxyCapsuleList contains a list of HAProxyCapsule
type HAProxyCapsuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HAProxyCapsule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HAProxyCapsule{}, &HAProxyCapsuleList{})
}
