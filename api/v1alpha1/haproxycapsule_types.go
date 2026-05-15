// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HAProxyServiceType is an alias for ProxyServiceType for backwards compatibility.
type HAProxyServiceType = ProxyServiceType

const (
	HAProxyServiceTypeClusterIP    = ProxyServiceTypeClusterIP
	HAProxyServiceTypeIngress      = ProxyServiceTypeIngress
	HAProxyServiceTypeLoadBalancer = ProxyServiceTypeLoadBalancer
)

// HAProxyHealthCheckSpec configures startup, liveness, and readiness probes using HTTP GET /stats on the metrics port.
type HAProxyHealthCheckSpec struct {
	// Enabled turns on startup, liveness, and readiness probes.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// StartupInitialDelaySeconds is the delay before the startup probe first runs.
	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=0
	// +optional
	StartupInitialDelaySeconds int32 `json:"startupInitialDelaySeconds,omitempty"`

	// StartupPeriodSeconds is the startup probe interval.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +optional
	StartupPeriodSeconds int32 `json:"startupPeriodSeconds,omitempty"`

	// StartupFailureThreshold is the number of startup probe failures before restart. 720 × 5s = 60 min window.
	// +kubebuilder:default=720
	// +kubebuilder:validation:Minimum=1
	// +optional
	StartupFailureThreshold int32 `json:"startupFailureThreshold,omitempty"`

	// PeriodSeconds is the liveness/readiness probe interval.
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// FailureThreshold is the number of consecutive liveness/readiness failures before the pod is unhealthy.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// TimeoutSeconds is the probe timeout.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// HAProxyCapsuleSpec defines the desired state of HAProxyCapsule
// +kubebuilder:validation:XValidation:rule="self.serviceType != 'Ingress' || has(self.ingressSpec)",message="ingressSpec is required when serviceType is Ingress"
type HAProxyCapsuleSpec struct {
	// Orbit is the owning Orbit's name. Set so the Orbit can cascade-delete this proxy.
	// +optional
	Orbit string `json:"orbit,omitempty"`

	// Image is the HAProxy container image.
	// +kubebuilder:default="haproxy:2.8.10-alpine3.20"
	// +optional
	Image string `json:"image,omitempty"`

	// Env are environment variables for the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// TLSSecretName is a kubernetes.io/tls secret (tls.crt + tls.key). Enables TLS termination on grpcsPort.
	// Works directly with cert-manager Certificate secrets.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// ServiceType is the Kubernetes Service type.
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType HAProxyServiceType `json:"serviceType,omitempty"`

	// ServiceSpec overrides the generated Service spec.
	// +optional
	ServiceSpec *corev1.ServiceSpec `json:"serviceSpec,omitempty"`

	// IngressSpec is required when serviceType is Ingress.
	// +optional
	IngressSpec *networkingv1.IngressSpec `json:"ingressSpec,omitempty"`

	// NodeSelector pins pods to specific nodes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity configures pod scheduling affinity.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations allow pods to schedule on tainted nodes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TerminationGracePeriodSeconds is the pod shutdown grace period.
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=0
	// +optional
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// ExternalTrafficPolicy controls routing for external traffic.
	// +kubebuilder:validation:Enum=Cluster;Local
	// +kubebuilder:default=Local
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`

	// ServiceAnnotations are extra Service annotations (e.g. cloud.google.com/neg for GCP NEG).
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// AdditionalLabels are merged into all managed resources. Don't affect the pod selector.
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// GrpcPort is the plain gRPC frontend port.
	// +kubebuilder:default=50211
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	GrpcPort int32 `json:"grpcPort,omitempty"`

	// GrpcsPort is the TLS gRPC frontend port.
	// +kubebuilder:default=50212
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	GrpcsPort int32 `json:"grpcsPort,omitempty"`

	// MetricsPort is the Prometheus metrics port.
	// +kubebuilder:default=9090
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// UpstreamService is the consensus node Service to route traffic to.
	// Defaults to the CR name with "haproxy-" stripped (haproxy-node1 → node1). Accepts any DNS name or FQDN.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`
	// +optional
	UpstreamService string `json:"upstreamService,omitempty"`

	// HealthCheck configures probes. Defaults match the GKE reference deployment.
	// +optional
	HealthCheck *HAProxyHealthCheckSpec `json:"healthCheck,omitempty"`
}

// HAProxyCapsuleStatus defines the observed state of HAProxyCapsule
type HAProxyCapsuleStatus struct {
	// Phase is the current reconciliation phase.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// ObservedGeneration is the last generation the controller acted on.
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
