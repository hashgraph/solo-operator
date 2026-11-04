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

// HAProxyReloadStrategy is HotReload or Restart.
// +kubebuilder:validation:Enum=HotReload;Restart
type HAProxyReloadStrategy string

const (
	HAProxyReloadStrategyHotReload HAProxyReloadStrategy = "HotReload"
	HAProxyReloadStrategyRestart   HAProxyReloadStrategy = "Restart"
)

type HAProxyReloaderSpec struct {
	// Image defaults to spec.image; needs sh, readlink and nc or bash.
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="self.matches('@sha256:[a-f0-9]{64}$') || self.matches(':[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$')",message="image must be pinned by a tag or a sha256 digest"
	// +kubebuilder:validation:XValidation:rule="!self.lowerAscii().matches(':latest(@sha256:[a-f0-9]{64})?$')",message="image must not use the mutable 'latest' tag"
	// +optional
	Image string `json:"image,omitempty"`

	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +optional
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
}

// HAProxyBackendProtocol is the protocol spoken to the upstream.
// +kubebuilder:validation:Enum=HTTP;HTTPS
type HAProxyBackendProtocol string

const (
	HAProxyBackendProtocolHTTP  HAProxyBackendProtocol = "HTTP"
	HAProxyBackendProtocolHTTPS HAProxyBackendProtocol = "HTTPS"
)

// HAProxyBackendHealthCheck is the upstream server check type.
// +kubebuilder:validation:Enum=TCP;GRPC
type HAProxyBackendHealthCheck string

const (
	HAProxyBackendHealthCheckTCP  HAProxyBackendHealthCheck = "TCP"
	HAProxyBackendHealthCheckGRPC HAProxyBackendHealthCheck = "GRPC"
)

// HAProxyBackendTLSSpec configures upstream TLS. Certificates are not verified (verify none).
// TODO: add verify Required with a CA certificate reference.
type HAProxyBackendTLSSpec struct {
	// ServerName is sent as SNI.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`
	// +optional
	ServerName string `json:"serverName,omitempty"`
}

type HAProxyBackendSpec struct {
	// Protocol defaults to HTTP for grpc and HTTPS for grpcs.
	// +optional
	Protocol HAProxyBackendProtocol `json:"protocol,omitempty"`

	// Port defaults to 50211 for HTTP and 50212 for HTTPS.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// HealthCheck GRPC POSTs /grpc.health.v1.Health/Check and expects HTTP 200.
	// +kubebuilder:default=TCP
	// +optional
	HealthCheck HAProxyBackendHealthCheck `json:"healthCheck,omitempty"`

	// +optional
	TLS *HAProxyBackendTLSSpec `json:"tls,omitempty"`
}

// HAProxyBackendsSpec configures the upstream per frontend.
// +kubebuilder:validation:XValidation:rule="!has(self.grpc) || !has(self.grpc.tls) || (has(self.grpc.protocol) && self.grpc.protocol == 'HTTPS')",message="backends.grpc.tls requires protocol HTTPS"
// +kubebuilder:validation:XValidation:rule="!has(self.grpcs) || !has(self.grpcs.tls) || !has(self.grpcs.protocol) || self.grpcs.protocol == 'HTTPS'",message="backends.grpcs.tls requires protocol HTTPS"
type HAProxyBackendsSpec struct {
	// Grpc also serves grpcsPort when tlsSecretName is unset.
	// +optional
	Grpc *HAProxyBackendSpec `json:"grpc,omitempty"`

	// +optional
	Grpcs *HAProxyBackendSpec `json:"grpcs,omitempty"`
}

type HAProxyConfigMapKeyRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

type HAProxyACLFile struct {
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	ConfigMapRef HAProxyConfigMapKeyRef `json:"configMapRef"`
}

type HAProxyTimeoutsSpec struct {
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
	Tunnel *metav1.Duration `json:"tunnel,omitempty"`

	// +optional
	HTTPKeepAlive *metav1.Duration `json:"httpKeepAlive,omitempty"`

	// HardStopAfter caps how long old workers drain after a reload.
	// +kubebuilder:default="30m"
	// +optional
	HardStopAfter *metav1.Duration `json:"hardStopAfter,omitempty"`
}

// HAProxyCapsuleSpec defines the desired state of HAProxyCapsule
// +kubebuilder:validation:XValidation:rule="self.serviceType != 'Ingress' || has(self.ingressSpec)",message="ingressSpec is required when serviceType is Ingress"
// +kubebuilder:validation:XValidation:rule="!has(self.backends) || !has(self.backends.grpcs) || (has(self.tlsSecretName) && size(self.tlsSecretName) > 0)",message="backends.grpcs requires tlsSecretName"
// +kubebuilder:validation:XValidation:rule="!(9999 in [self.grpcPort, self.grpcsPort, self.metricsPort])",message="port 9999 is reserved for the HAProxy master CLI"
type HAProxyCapsuleSpec struct {
	// Orbit is the owning Orbit's name.
	// +kubebuilder:validation:MinLength=1
	Orbit string `json:"orbit"`

	// Image is the HAProxy container image.
	// +kubebuilder:default="haproxy:2.8.10-alpine3.20"
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="self.matches('@sha256:[a-f0-9]{64}$') || self.matches(':[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$')",message="image must be pinned by a tag or a sha256 digest"
	// +kubebuilder:validation:XValidation:rule="!self.lowerAscii().matches(':latest(@sha256:[a-f0-9]{64})?$')",message="image must not use the mutable 'latest' tag"
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

	// +optional
	Backends *HAProxyBackendsSpec `json:"backends,omitempty"`

	// ACLFiles are IP/CIDR blocklists matched on source address and x-forwarded-for.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=16
	// +optional
	ACLFiles []HAProxyACLFile `json:"aclFiles,omitempty"`

	// +optional
	Timeouts *HAProxyTimeoutsSpec `json:"timeouts,omitempty"`

	// +kubebuilder:default=10000
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`

	// ReloadStrategy HotReload reloads in place via a sidecar; Restart rolls the pod.
	// +kubebuilder:default=HotReload
	// +optional
	ReloadStrategy HAProxyReloadStrategy `json:"reloadStrategy,omitempty"`

	// +optional
	Reloader *HAProxyReloaderSpec `json:"reloader,omitempty"`
}

// HAProxyCapsuleStatus defines the observed state of HAProxyCapsule
type HAProxyCapsuleStatus struct {
	// Phase is the current reconciliation phase.
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Unknown
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
