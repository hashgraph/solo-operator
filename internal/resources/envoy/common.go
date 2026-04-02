// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

//go:embed tmpl/envoy.yaml.tmpl
var envoyYamlTmpl string

var envoyTemplate = template.Must(template.New("envoy.yaml").Parse(envoyYamlTmpl))

type envoyConfigData struct {
	AdminAddress    string
	AdminPort       int32
	ProxyPort       int32
	MetricsPort     int32
	AllowOrigins    string
	AllowMethods    string
	AllowHeaders    string
	MaxAge          string
	ExposeHeaders   string
	TLSBlock        bool
	TLSCertPath     string
	TLSKeyPath      string
	UpstreamService string
	UpstreamPort    int32
}

const (
	LabelManagedBy   = "app.kubernetes.io/managed-by"
	LabelPartOf      = "app.kubernetes.io/part-of"
	LabelApp         = "app"
	ManagedByValue   = "solo-operator"
	PartOfEnvoyProxy = "envoy-proxy"
	ConfigMapSuffix  = "-config"
)

const (
	DefaultImage       = "envoyproxy/envoy:v1.21.6"
	DefaultReplicas    = int32(2)
	DefaultProxyPort   = int32(8080)
	DefaultAdminPort   = int32(9901)
	DefaultMetricsPort = int32(9090)
)

const (
	ConfigVolumeName = "config"
	ConfigMountPath  = "/etc/envoy"
	TLSVolumeName    = "tls"
	TLSMountPath     = "/etc/envoy/certs"
	TLSCertPath      = "/etc/envoy/certs/tls.crt"
	TLSKeyPath       = "/etc/envoy/certs/tls.key"
	ContainerName    = "envoy"
	ConfigFileName   = "envoy.yaml"
)

// CommonLabels returns the pod selector labels. These must remain stable — never include AdditionalLabels here.
func CommonLabels(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfEnvoyProxy,
		LabelApp:       envoy.Name,
	}
}

// ResourceLabels merges CommonLabels with AdditionalLabels. Use on ObjectMeta and pod template, not selectors.
func ResourceLabels(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	labels := CommonLabels(envoy)
	for k, v := range envoy.Spec.AdditionalLabels {
		labels[k] = v
	}
	return labels
}

func ConfigMapName(envoy *operatorv1alpha1.EnvoyProxy) string {
	return fmt.Sprintf("%s%s", envoy.Name, ConfigMapSuffix)
}

func GetImage(envoy *operatorv1alpha1.EnvoyProxy) string {
	if envoy.Spec.Image != "" {
		return envoy.Spec.Image
	}
	return DefaultImage
}

func GetReplicas(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.Replicas != nil {
		return *envoy.Spec.Replicas
	}
	return DefaultReplicas
}

func GetProxyPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.ProxyPort != 0 {
		return envoy.Spec.ProxyPort
	}
	return DefaultProxyPort
}

func GetAdminPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.AdminPort != 0 {
		return envoy.Spec.AdminPort
	}
	return DefaultAdminPort
}

func GetMetricsPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.MetricsPort != 0 {
		return envoy.Spec.MetricsPort
	}
	return DefaultMetricsPort
}

func BuildEnv(envoy *operatorv1alpha1.EnvoyProxy) []corev1.EnvVar {
	if len(envoy.Spec.Env) > 0 {
		return envoy.Spec.Env
	}
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

// DefaultEnvoyConfig generates the operator-managed envoy.yaml: a gRPC-Web proxy listener and a Prometheus metrics listener.
func DefaultEnvoyConfig(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	adminAddress := "127.0.0.1"
	if envoy.Spec.ExposeAdmin {
		adminAddress = "0.0.0.0"
	}

	// Build CORS values — use spec overrides or fall back to defaults
	allowOrigins := "- prefix: '*'"
	allowMethods := "GET, PUT, DELETE, POST, OPTIONS"
	allowHeaders := "keep-alive,user-agent,cache-control,content-type,content-transfer-encoding,custom-header-1,x-accept-content-transfer-encoding,x-accept-response-streaming,x-user-agent,x-grpc-web,grpc-timeout"
	exposeHeaders := "custom-header-1,grpc-status,grpc-message"
	maxAge := `"1728000"`

	if c := envoy.Spec.CORSConfig; c != nil {
		if len(c.AllowOrigins) > 0 {
			parts := make([]string, len(c.AllowOrigins))
			for i, o := range c.AllowOrigins {
				parts[i] = "- prefix: '" + o + "'"
			}
			allowOrigins = strings.Join(parts, "\n                          ")
		}
		if c.AllowMethods != "" {
			allowMethods = c.AllowMethods
		}
		if len(c.AllowHeaders) > 0 {
			allowHeaders = strings.Join(c.AllowHeaders, ",")
		}
		if len(c.ExposeHeaders) > 0 {
			exposeHeaders = strings.Join(c.ExposeHeaders, ",")
		}
		if c.MaxAge != "" {
			maxAge = `"` + c.MaxAge + `"`
		}
	}

	data := envoyConfigData{
		AdminAddress:    adminAddress,
		AdminPort:       GetAdminPort(envoy),
		ProxyPort:       GetProxyPort(envoy),
		MetricsPort:     GetMetricsPort(envoy),
		AllowOrigins:    allowOrigins,
		AllowMethods:    allowMethods,
		AllowHeaders:    allowHeaders,
		MaxAge:          maxAge,
		ExposeHeaders:   exposeHeaders,
		TLSBlock:        envoy.Spec.TLSSecretName != "",
		TLSCertPath:     TLSCertPath,
		TLSKeyPath:      TLSKeyPath,
		UpstreamService: GetUpstreamService(envoy),
		UpstreamPort:    GetUpstreamPort(envoy),
	}

	var buf bytes.Buffer
	if err := envoyTemplate.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("envoy config template execution failed: %v", err))
	}

	return map[string]string{
		ConfigFileName: buf.String(),
	}
}

// GetUpstreamService strips the "envoy-" prefix from the CR name when UpstreamService is not set.
func GetUpstreamService(envoy *operatorv1alpha1.EnvoyProxy) string {
	if envoy.Spec.UpstreamService != "" {
		return envoy.Spec.UpstreamService
	}
	name := envoy.Name
	if len(name) > 6 && name[:6] == "envoy-" {
		return name[6:]
	}
	return name
}

func GetUpstreamPort(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.UpstreamPort != 0 {
		return envoy.Spec.UpstreamPort
	}
	return int32(50211)
}

func GetServiceType(envoy *operatorv1alpha1.EnvoyProxy) operatorv1alpha1.ProxyServiceType {
	if envoy.Spec.ServiceType != "" {
		return envoy.Spec.ServiceType
	}
	return operatorv1alpha1.ProxyServiceTypeLoadBalancer
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.EnvoyProxy] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewIngressSubReconciler(c, scheme),
	)
}
