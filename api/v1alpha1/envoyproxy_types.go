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

// EnvoyUpstreamProtocol is the upstream protocol.
// +kubebuilder:validation:Enum=HTTP;HTTPS
type EnvoyUpstreamProtocol string

const (
	EnvoyUpstreamProtocolHTTP  EnvoyUpstreamProtocol = "HTTP"
	EnvoyUpstreamProtocolHTTPS EnvoyUpstreamProtocol = "HTTPS"
)

// EnvoyHealthCheckType is the upstream health check.
// +kubebuilder:validation:Enum=TCP;GRPC
type EnvoyHealthCheckType string

const (
	EnvoyHealthCheckTCP  EnvoyHealthCheckType = "TCP"
	EnvoyHealthCheckGRPC EnvoyHealthCheckType = "GRPC"
)

type EnvoyUpstreamSpec struct {
	// +optional
	Protocol EnvoyUpstreamProtocol `json:"protocol,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// +kubebuilder:default=TCP
	// +optional
	HealthCheck EnvoyHealthCheckType `json:"healthCheck,omitempty"`
}

type EnvoyListenerSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// +optional
	Upstream *EnvoyUpstreamSpec `json:"upstream,omitempty"`
}

type EnvoyGRPCWebListenerSpec struct {
	EnvoyListenerSpec `json:",inline"`

	// +optional
	CORS *CORSConfig `json:"cors,omitempty"`

	// +kubebuilder:validation:MaxItems=64
	// +optional
	TrustedProxyCIDRs []EnvoyCIDR `json:"trustedProxyCIDRs,omitempty"`
}

// EnvoyListenersSpec defaults ports to 8081, 50211 and 50212; upstreams to HTTP 50211, except grpcs to HTTPS 50212.
// +kubebuilder:validation:XValidation:rule="!has(self.grpcs) || has(self.grpcs.tlsSecretName)",message="grpcs requires tlsSecretName"
type EnvoyListenersSpec struct {
	// +optional
	GRPCWeb *EnvoyGRPCWebListenerSpec `json:"grpcWeb,omitempty"`

	// +optional
	GRPC *EnvoyListenerSpec `json:"grpc,omitempty"`

	// +optional
	GRPCS *EnvoyListenerSpec `json:"grpcs,omitempty"`
}

type EnvoyConfigMapKeyRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// EnvoyCIDR is an IPv4 or IPv6 CIDR.
// +kubebuilder:validation:MaxLength=43
// +kubebuilder:validation:XValidation:rule="isCIDR(self)",message="must be a CIDR"
type EnvoyCIDR string

type EnvoyTimeoutsSpec struct {
	// +kubebuilder:default="10s"
	// +optional
	Connect *metav1.Duration `json:"connect,omitempty"`

	// +kubebuilder:default="120s"
	// +optional
	Client *metav1.Duration `json:"client,omitempty"`

	// +kubebuilder:default="120s"
	// +optional
	Server *metav1.Duration `json:"server,omitempty"`

	// +kubebuilder:default="3600s"
	// +optional
	Stream *metav1.Duration `json:"stream,omitempty"`

	// +kubebuilder:default="30m"
	// +optional
	Drain *metav1.Duration `json:"drain,omitempty"`
}

// EnvoyProxySpec defines the desired state of EnvoyProxy
// +kubebuilder:validation:XValidation:rule="self.serviceType != 'Ingress' || has(self.ingressSpec)",message="ingressSpec is required when serviceType is Ingress"
// +kubebuilder:validation:XValidation:rule="!has(self.aclFiles) || size(self.aclFiles) == 0 || self.serviceType != 'LoadBalancer' || self.externalTrafficPolicy == 'Local'",message="aclFiles with a LoadBalancer Service require externalTrafficPolicy Local"
type EnvoyProxySpec struct {
	// Orbit is the owning Orbit's name.
	// +kubebuilder:validation:MinLength=1
	Orbit string `json:"orbit"`

	// Image is the Envoy container image.
	// +kubebuilder:default="envoyproxy/envoy:distroless-v1.40.2"
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="self.matches('@sha256:[a-f0-9]{64}$') || self.matches(':[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$')",message="image must be pinned by a tag or a sha256 digest"
	// +kubebuilder:validation:XValidation:rule="!self.lowerAscii().matches(':latest(@sha256:[a-f0-9]{64})?$')",message="image must not use the mutable 'latest' tag"
	// +optional
	Image string `json:"image,omitempty"`

	// Env are environment variables for the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Replicas is the number of proxy replicas.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// +kubebuilder:default={grpcWeb: {}}
	// +optional
	Listeners *EnvoyListenersSpec `json:"listeners,omitempty"`

	// AdminPort is the Envoy admin interface port.
	// +kubebuilder:default=9901
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	AdminPort int32 `json:"adminPort,omitempty"`

	// MetricsPort is the Prometheus metrics port.
	// +kubebuilder:default=9090
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// Files overlays the generated envoy.yaml, lds.yaml, cds.yaml and sds-*.yaml by key.
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
	// +kubebuilder:validation:Minimum=0
	// +optional
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// UpstreamService is the consensus node Service to route traffic to.
	// Defaults to the CR name with "envoy-" stripped (envoy-node1 → node1). Accepts any DNS name or FQDN.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`
	// +optional
	UpstreamService string `json:"upstreamService,omitempty"`

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

	// ExposeAdmin binds the Envoy admin interface to 0.0.0.0 and adds the admin port to the container ports.
	// By default admin is bound to 127.0.0.1 only (not reachable outside the pod).
	// +optional
	ExposeAdmin bool `json:"exposeAdmin,omitempty"`

	// +listType=map
	// +listMapKey=name
	// +listMapKey=key
	// +kubebuilder:validation:MaxItems=16
	// +optional
	ACLFiles []EnvoyConfigMapKeyRef `json:"aclFiles,omitempty"`

	// +optional
	Timeouts *EnvoyTimeoutsSpec `json:"timeouts,omitempty"`

	// +kubebuilder:default=10000
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
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

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// +optional
	DeploymentConditions []appsv1.DeploymentCondition `json:"deploymentConditions,omitempty"`
}

const EnvoyProxyConditionTypeReconciled = "Reconciled"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=envoy
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Reconciled",type="string",JSONPath=".status.conditions[?(@.type=='Reconciled')].status"
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
