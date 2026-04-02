// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CORSConfig customizes CORS in the operator-generated envoy.yaml. All fields are optional.
type CORSConfig struct {
	// AllowOrigins is a list of allowed origins (prefix match). Defaults to wildcard.
	// +optional
	AllowOrigins []string `json:"allowOrigins,omitempty"`

	// AllowMethods overrides the allowed HTTP methods.
	// +optional
	AllowMethods string `json:"allowMethods,omitempty"`

	// AllowHeaders overrides the list of allowed request headers.
	// +optional
	AllowHeaders []string `json:"allowHeaders,omitempty"`

	// ExposeHeaders overrides the list of headers exposed to the browser.
	// +optional
	ExposeHeaders []string `json:"exposeHeaders,omitempty"`

	// MaxAge overrides the preflight cache duration.
	// +optional
	MaxAge string `json:"maxAge,omitempty"`
}

// EnvoyProxySpec defines the desired state of EnvoyProxy
type EnvoyProxySpec struct {
	// Image is the Envoy container image.
	// +kubebuilder:default="envoyproxy/envoy:v1.21.6"
	// +optional
	Image string `json:"image,omitempty"`

	// Env are environment variables for the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Replicas is the number of proxy replicas.
	// +kubebuilder:default=2
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ProxyPort is the gRPC-Web listener port.
	// +kubebuilder:default=8080
	// +optional
	ProxyPort int32 `json:"proxyPort,omitempty"`

	// AdminPort is the Envoy admin interface port.
	// +kubebuilder:default=9901
	// +optional
	AdminPort int32 `json:"adminPort,omitempty"`

	// MetricsPort is the Prometheus metrics port.
	// +kubebuilder:default=9090
	// +optional
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// Files overrides the operator-generated envoy.yaml. Map of filename → content.
	// +optional
	Files map[string]string `json:"files,omitempty"`

	// ServiceType is the Kubernetes Service type.
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType ProxyServiceType `json:"serviceType,omitempty"`

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
	// +optional
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// UpstreamService is the consensus node Service to route traffic to.
	// Defaults to the CR name with "envoy-" stripped (envoy-node1 → node1). Accepts any DNS name or FQDN.
	// +optional
	UpstreamService string `json:"upstreamService,omitempty"`

	// UpstreamPort is the upstream gRPC port.
	// +kubebuilder:default=50211
	// +optional
	UpstreamPort int32 `json:"upstreamPort,omitempty"`

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

	// CORSConfig customizes CORS in the operator-generated config. Ignored if Files is set.
	// +optional
	CORSConfig *CORSConfig `json:"corsConfig,omitempty"`

	// TLSSecretName is a kubernetes.io/tls secret (tls.crt + tls.key). Enables TLS termination on proxyPort.
	// Works directly with cert-manager Certificate secrets.
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// ExposeAdmin binds the Envoy admin interface to 0.0.0.0 and adds the admin port to the container ports.
	// By default admin is bound to 127.0.0.1 only (not reachable outside the pod).
	// +optional
	ExposeAdmin bool `json:"exposeAdmin,omitempty"`
}

// EnvoyProxyStatus defines the observed state of EnvoyProxy
type EnvoyProxyStatus struct {
	// ObservedGeneration is the last generation the controller acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the total replica count.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// AvailableReplicas is the number of available replicas.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ReadyReplicas is the number of ready replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// TerminatingReplicas is the number of terminating replicas.
	// +optional
	TerminatingReplicas int32 `json:"terminatingReplicas,omitempty"`

	// UnavailableReplicas is the number of unavailable replicas.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// UpdatedReplicas is the number of updated replicas.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Conditions are the deployment conditions.
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
