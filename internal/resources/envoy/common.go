// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// Label keys
const (
	// LabelManagedBy indicates the resource is managed by solo-operator
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelPartOf indicates the component the resource is part of
	LabelPartOf = "app.kubernetes.io/part-of"
	// LabelApp indicates the application name
	LabelApp = "app"
)

// Label values
const (
	// ManagedByValue is the value for the managed-by label
	ManagedByValue = "solo-operator"
	// PartOfEnvoyProxy is the value for Envoy proxy part-of label
	PartOfEnvoyProxy = "envoy-proxy"
)

// Resource name suffixes
const (
	ConfigMapSuffix = "-config"
)

// Default values
const (
	// DefaultImage is the default Envoy image
	DefaultImage = "envoyproxy/envoy:v1.21.6"
	// DefaultReplicas is the default number of replicas
	DefaultReplicas = int32(2)
	// DefaultProxyPort is the default proxy port
	DefaultProxyPort = int32(8081)
	// DefaultAdminPort is the default admin port
	DefaultAdminPort = int32(9901)
)

// Volume and mount paths
const (
	ConfigVolumeName = "config"
	ConfigMountPath  = "/etc/envoy"
	AdminVolumeName  = "admin"
	AdminMountPath   = "/admin"
	ContainerName    = "envoy"
	ConfigFileName   = "envoy.yaml"
)

// CommonLabels returns the common labels for Envoy proxy resources
func CommonLabels(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfEnvoyProxy,
		LabelApp:       envoy.Name,
	}
}

// ConfigMapName returns the config map name for an Envoy proxy
func ConfigMapName(envoy *operatorv1alpha1.EnvoyProxy) string {
	return fmt.Sprintf("%s%s", envoy.Name, ConfigMapSuffix)
}

// GetImage returns the image to use, with default fallback
func GetImage(envoy *operatorv1alpha1.EnvoyProxy) string {
	if envoy.Spec.Image != "" {
		return envoy.Spec.Image
	}
	return DefaultImage
}

// GetReplicas returns the number of replicas, with default fallback
func GetReplicas(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.Replicas != nil {
		return *envoy.Spec.Replicas
	}
	return DefaultReplicas
}

// GetProxyPort returns the proxy port, with default fallback
func GetProxyPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.ProxyPort != 0 {
		return envoy.Spec.ProxyPort
	}
	return DefaultProxyPort
}

// GetAdminPort returns the admin port, with default fallback
func GetAdminPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.AdminPort != 0 {
		return envoy.Spec.AdminPort
	}
	return DefaultAdminPort
}

// BuildEnv builds the environment variables for the Envoy container
func BuildEnv(envoy *operatorv1alpha1.EnvoyProxy) []corev1.EnvVar {
	if len(envoy.Spec.Env) > 0 {
		return envoy.Spec.Env
	}

	// Default environment variables
	return []corev1.EnvVar{
		{
			Name: "SERVICE_CLUSTER",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "SERVICE_NODE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}
}

// DefaultEnvoyConfig returns the default Envoy configuration
func DefaultEnvoyConfig(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	proxyPort := GetProxyPort(envoy)

	config := fmt.Sprintf(`admin:
  address:
    pipe:
      path: /admin/admin.sock
static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address:
          address: 0.0.0.0
          port_value: %d
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                codec_type: auto
                stat_prefix: ingress_http
                use_remote_address: true
                skip_xff_append: false
                route_config:
                  name: local_route
                  virtual_hosts:
                    - name: local_service
                      domains:
                        - '*'
                      routes:
                        - match:
                            prefix: /
                          route:
                            cluster: echo_service
                            timeout: 0s
                            max_stream_duration:
                              grpc_timeout_header_max: 0s
                      cors:
                        allow_origin_string_match:
                          - prefix: '*'
                        allow_methods: GET, PUT, DELETE, POST, OPTIONS
                        allow_headers: keep-alive,user-agent,cache-control,content-type,content-transfer-encoding,custom-header-1,x-accept-content-transfer-encoding,x-accept-response-streaming,x-user-agent,x-grpc-web,grpc-timeout
                        max_age: "1728000"
                        expose_headers: custom-header-1,grpc-status,grpc-message
                http_filters:
                  - name: envoy.filters.http.grpc_web
                  - name: envoy.filters.http.cors
                  - name: envoy.filters.http.router
  clusters:
    - name: echo_service
      connect_timeout: 0.25s
      type: logical_dns
      http2_protocol_options: {}
      lb_policy: round_robin
      load_assignment:
        cluster_name: cluster_0
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: hedera-proxy
                      port_value: 50211
`, proxyPort)

	return map[string]string{
		ConfigFileName: config,
	}
}

// NewSubReconcilers creates all Envoy proxy subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.EnvoyProxy] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
	)
}
