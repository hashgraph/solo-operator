// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	LabelManagedBy  = "app.kubernetes.io/managed-by"
	LabelPartOf     = "app.kubernetes.io/part-of"
	LabelApp        = "app"
	ManagedByValue  = "solo-operator"
	PartOfHAProxy   = "haproxy"
	ConfigMapSuffix = "-config"
	DefaultImage    = "haproxy:2.8.10-alpine3.20"
)

const (
	DefaultGrpcPort    = int32(50211)
	DefaultGrpcsPort   = int32(50212)
	DefaultMetricsPort = int32(9090)
)

func GetGrpcPort(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.GrpcPort != 0 {
		return haproxy.Spec.GrpcPort
	}
	return DefaultGrpcPort
}

func GetGrpcsPort(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.GrpcsPort != 0 {
		return haproxy.Spec.GrpcsPort
	}
	return DefaultGrpcsPort
}

func GetMetricsPort(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.MetricsPort != 0 {
		return haproxy.Spec.MetricsPort
	}
	return DefaultMetricsPort
}

// GetUpstreamService strips the "haproxy-" prefix from the CR name when UpstreamService is not set.
func GetUpstreamService(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	if haproxy.Spec.UpstreamService != "" {
		return haproxy.Spec.UpstreamService
	}
	name := haproxy.Name
	if len(name) > 8 && name[:8] == "haproxy-" {
		return name[8:]
	}
	return name
}

// Volume and mount paths
const (
	ConfigVolumeName = "config"
	ConfigMountPath  = "/usr/local/etc/haproxy/haproxy.cfg"
	ConfigFileName   = "haproxy.cfg"
	TLSVolumeName    = "tls-secrets"
	TLSMountPath     = "/etc/haproxy/certs"
	ContainerName    = "haproxy"
)

func HAProxyConfig(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	grpcPort := GetGrpcPort(haproxy)
	grpcsPort := GetGrpcsPort(haproxy)
	metricsPort := GetMetricsPort(haproxy)
	upstream := GetUpstreamService(haproxy)
	tlsEnabled := haproxy.Spec.TLSSecretName != ""

	if tlsEnabled {
		return fmt.Sprintf(`
global
    log stdout format raw local0

defaults
    log global
    mode tcp
    option tcplog
    timeout connect 10s
    timeout client 30s
    timeout server 30s

frontend grpc_frontend
    bind *:%d
    default_backend consensus_nodes

frontend grpcs_frontend
    bind *:%d ssl crt /etc/haproxy/certs/tls.crt alpn h2
    mode http
    default_backend consensus_nodes_tls

backend consensus_nodes
    balance roundrobin
    server upstream %s:%d check

backend consensus_nodes_tls
    mode http
    balance roundrobin
    server upstream %s:%d ssl verify none check

listen stats
    bind *:%d
    mode http
    stats enable
    stats uri /stats
    stats refresh 10s
`, grpcPort, grpcsPort, upstream, grpcPort, upstream, grpcsPort, metricsPort)
	}

	return fmt.Sprintf(`
global
    log stdout format raw local0

defaults
    log global
    mode tcp
    option tcplog
    timeout connect 10s
    timeout client 30s
    timeout server 30s

frontend grpc_frontend
    bind *:%d
    default_backend consensus_nodes

frontend grpcs_frontend
    bind *:%d
    default_backend consensus_nodes_tls

backend consensus_nodes
    balance roundrobin
    server upstream %s:%d check

backend consensus_nodes_tls
    balance roundrobin
    server upstream %s:%d check

listen stats
    bind *:%d
    mode http
    stats enable
    stats uri /stats
    stats refresh 10s
`, grpcPort, grpcsPort, upstream, grpcPort, upstream, grpcsPort, metricsPort)
}

// CommonLabels returns the pod selector labels. These must remain stable — never include AdditionalLabels here.
func CommonLabels(haproxy *operatorv1alpha1.HAProxyCapsule) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfHAProxy,
		LabelApp:       haproxy.Name,
	}
}

// ResourceLabels merges CommonLabels with AdditionalLabels. Use on ObjectMeta and pod template, not selectors.
func ResourceLabels(haproxy *operatorv1alpha1.HAProxyCapsule) map[string]string {
	labels := CommonLabels(haproxy)
	for k, v := range haproxy.Spec.AdditionalLabels {
		labels[k] = v
	}
	return labels
}

func DefaultHealthCheck() operatorv1alpha1.HAProxyHealthCheckSpec {
	return operatorv1alpha1.HAProxyHealthCheckSpec{
		Enabled:                    true,
		StartupInitialDelaySeconds: 60,
		StartupPeriodSeconds:       5,
		StartupFailureThreshold:    720,
		PeriodSeconds:              10,
		FailureThreshold:           3,
		TimeoutSeconds:             1,
	}
}

func GetHealthCheck(haproxy *operatorv1alpha1.HAProxyCapsule) operatorv1alpha1.HAProxyHealthCheckSpec {
	defaults := DefaultHealthCheck()
	if haproxy.Spec.HealthCheck == nil {
		return defaults
	}
	hc := *haproxy.Spec.HealthCheck
	if hc.StartupInitialDelaySeconds == 0 {
		hc.StartupInitialDelaySeconds = defaults.StartupInitialDelaySeconds
	}
	if hc.StartupPeriodSeconds == 0 {
		hc.StartupPeriodSeconds = defaults.StartupPeriodSeconds
	}
	if hc.StartupFailureThreshold == 0 {
		hc.StartupFailureThreshold = defaults.StartupFailureThreshold
	}
	if hc.PeriodSeconds == 0 {
		hc.PeriodSeconds = defaults.PeriodSeconds
	}
	if hc.FailureThreshold == 0 {
		hc.FailureThreshold = defaults.FailureThreshold
	}
	if hc.TimeoutSeconds == 0 {
		hc.TimeoutSeconds = defaults.TimeoutSeconds
	}
	return hc
}

func ConfigMapName(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	return fmt.Sprintf("%s%s", haproxy.Name, ConfigMapSuffix)
}

func GetImage(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	if haproxy.Spec.Image != "" {
		return haproxy.Spec.Image
	}
	return DefaultImage
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.HAProxyCapsule] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewIngressSubReconciler(c, scheme),
	)
}
