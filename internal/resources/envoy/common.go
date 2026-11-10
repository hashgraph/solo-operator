// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	LabelManagedBy   = common.LabelManagedBy
	LabelPartOf      = common.LabelPartOf
	LabelApp         = "app"
	ManagedByValue   = common.ManagedByValue
	PartOfEnvoyProxy = "envoy-proxy"
	ConfigMapSuffix  = "-config"
)

const (
	DefaultImage          = "envoyproxy/envoy:distroless-v1.40.2"
	DefaultReplicas       = int32(2)
	DefaultGRPCWebPort    = int32(8081)
	DefaultAdminPort      = int32(9901)
	DefaultMetricsPort    = int32(9090)
	DefaultMaxConnections = int32(10000)
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultClientTimeout  = 120 * time.Second
	defaultServerTimeout  = 120 * time.Second
	defaultStreamTimeout  = 3600 * time.Second
	defaultDrainTimeout   = 30 * time.Minute
)

const (
	ConfigVolumeName         = "config"
	ConfigMountPath          = "/etc/envoy"
	TLSMountRoot             = "/etc/envoy-tls"
	ContainerName            = "envoy"
	ConfigFileName           = "envoy.yaml"
	LDSFileName              = "lds.yaml"
	CDSFileName              = "cds.yaml"
	ConfigChecksumAnnotation = "operator.solo.hiero.org/config-checksum"
	maxRenderedSize          = 900 * 1024
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

func GetDrainSeconds(envoy *operatorv1alpha1.EnvoyProxy) int64 {
	if t := envoy.Spec.Timeouts; t != nil && t.Drain != nil && t.Drain.Duration > 0 {
		return int64(t.Drain.Seconds())
	}
	return int64(defaultDrainTimeout.Seconds())
}

func GetMaxConnections(envoy *operatorv1alpha1.EnvoyProxy) int32 {
	if envoy.Spec.MaxConnections > 0 {
		return envoy.Spec.MaxConnections
	}
	return DefaultMaxConnections
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

func GetServiceType(envoy *operatorv1alpha1.EnvoyProxy) operatorv1alpha1.EnvoyServiceType {
	if envoy.Spec.ServiceType != "" {
		return envoy.Spec.ServiceType
	}
	return operatorv1alpha1.EnvoyServiceTypeLoadBalancer
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.EnvoyProxy] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewIngressSubReconciler(c, scheme),
	)
}
