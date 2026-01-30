// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"fmt"

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
	// PartOfHAProxy is the value for HAProxy part-of label
	PartOfHAProxy = "haproxy"
)

// Resource name suffixes
const (
	ConfigMapSuffix = "-config"
)

// Default values
const (
	// DefaultImage is the default HAProxy image
	DefaultImage = "haproxy:2.8.10-alpine3.20"
)

// Port numbers
const (
	GrpcPort  = 50211
	GrpcsPort = 50212
	StatsPort = 8404
)

// Volume and mount paths
const (
	ConfigVolumeName = "config"
	ConfigMountPath  = "/usr/local/etc/haproxy/haproxy.cfg"
	ConfigFileName   = "haproxy.cfg"
	TLSVolumeName    = "tls-secrets"
	TLSMountPath     = "/etc/haproxy/certs"
	ContainerName    = "haproxy"
)

// DefaultHAProxyConfig is the default HAProxy configuration
const DefaultHAProxyConfig = `
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
    bind *:50211
    default_backend consensus_nodes

frontend grpcs_frontend
    bind *:50212
    default_backend consensus_nodes_tls

backend consensus_nodes
    balance roundrobin
    server node1 consensus:50211 check

backend consensus_nodes_tls
    balance roundrobin
    server node1 consensus:50212 check

listen stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s
`

// CommonLabels returns the common labels for HAProxy capsule resources
func CommonLabels(haproxy *operatorv1alpha1.HAProxyCapsule) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfHAProxy,
		LabelApp:       haproxy.Name,
	}
}

// ConfigMapName returns the config map name for an HAProxy capsule
func ConfigMapName(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	return fmt.Sprintf("%s%s", haproxy.Name, ConfigMapSuffix)
}

// GetImage returns the image to use, with default fallback
func GetImage(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	if haproxy.Spec.Image != "" {
		return haproxy.Spec.Image
	}
	return DefaultImage
}

// NewSubReconcilers creates all HAProxy capsule subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.HAProxyCapsule] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
	)
}
