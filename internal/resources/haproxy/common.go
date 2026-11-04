// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

//go:embed tmpl/haproxy.cfg.tmpl
var haproxyCfgTmpl string

//go:embed tmpl/reloader.sh
var reloaderScript string

var haproxyTemplate = template.Must(template.New("haproxy.cfg").Parse(haproxyCfgTmpl))

type haproxyConfigData struct {
	GrpcPort       int32
	GrpcsPort      int32
	MetricsPort    int32
	TLSCertPath    string
	MaxConnections int32
	Timeouts       timeoutsData
	Grpc           backendData
	Grpcs          *backendData
	ACLs           []string
}

// timeoutsData is in milliseconds.
type timeoutsData struct {
	Connect       int64
	Client        int64
	Server        int64
	Tunnel        int64
	HTTPKeepAlive int64
	HardStopAfter int64
}

type backendData struct {
	Name       string
	Host       string
	Port       int32
	HTTPS      bool
	ServerName string
	GRPCCheck  bool
}

const (
	LabelManagedBy  = common.LabelManagedBy
	LabelPartOf     = common.LabelPartOf
	LabelApp        = "app"
	ManagedByValue  = common.ManagedByValue
	PartOfHAProxy   = "haproxy"
	ConfigMapSuffix = "-config"
	DefaultImage    = "haproxy:2.8.10-alpine3.20"
)

const (
	DefaultMetricsPort    = int32(9090)
	DefaultMaxConnections = int32(10000)
	DefaultReloadInterval = int32(5)
	MasterCLIHost         = "127.0.0.1"
	MasterCLIPort         = int32(9999)
)

var (
	defaultConnectTimeout       = 10 * time.Second
	defaultClientTimeout        = 120 * time.Second
	defaultServerTimeout        = 120 * time.Second
	defaultTunnelTimeout        = 3600 * time.Second
	defaultHardStopAfterTimeout = 30 * time.Minute
)

func GetGrpcPort(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.GrpcPort != 0 {
		return haproxy.Spec.GrpcPort
	}
	return common.GrpcPort
}

func GetGrpcsPort(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.GrpcsPort != 0 {
		return haproxy.Spec.GrpcsPort
	}
	return common.GrpcsPort
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
	ConfigVolumeName      = "config"
	ConfigMountPath       = "/usr/local/etc/haproxy/conf"
	ConfigFileName        = "haproxy.cfg"
	ConfigFilePath        = ConfigMountPath + "/" + ConfigFileName
	TLSVolumeName         = "tls-secrets"
	TLSMountPath          = "/etc/haproxy/certs"
	TLSCertPath           = TLSMountPath + "/tls.crt"
	ACLVolumeName         = "acl-files"
	ACLMountPath          = "/usr/local/etc/haproxy/acl"
	ContainerName         = "haproxy"
	ReloaderContainerName = "reloader"
)

func HAProxyConfig(haproxy *operatorv1alpha1.HAProxyCapsule) (string, error) {
	in := resolve(haproxy)
	data := haproxyConfigData{
		GrpcPort:       GetGrpcPort(haproxy),
		GrpcsPort:      GetGrpcsPort(haproxy),
		MetricsPort:    GetMetricsPort(haproxy),
		TLSCertPath:    TLSCertPath,
		MaxConnections: GetMaxConnections(haproxy),
		Timeouts:       getTimeouts(haproxy),
		Grpc:           in.grpc,
		Grpcs:          in.grpcs,
		ACLs:           in.acls,
	}
	var buf bytes.Buffer
	if err := haproxyTemplate.Execute(&buf, data); err != nil {
		return "", errx.WithReason(errTemplate.Wrap(err, "haproxy config template execution failed"), ReasonTemplateFailed)
	}
	return buf.String(), nil
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
		StartupInitialDelaySeconds: 5,
		StartupPeriodSeconds:       5,
		StartupFailureThreshold:    12, // 1 min window
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

func Images(haproxy *operatorv1alpha1.HAProxyCapsule) []string {
	if HotReloadEnabled(haproxy) {
		return []string{GetImage(haproxy), GetReloaderImage(haproxy)}
	}
	return []string{GetImage(haproxy)}
}

func GetReloaderImage(haproxy *operatorv1alpha1.HAProxyCapsule) string {
	if haproxy.Spec.Reloader != nil && haproxy.Spec.Reloader.Image != "" {
		return haproxy.Spec.Reloader.Image
	}
	return GetImage(haproxy)
}

func GetReloadInterval(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.Reloader != nil && haproxy.Spec.Reloader.IntervalSeconds > 0 {
		return haproxy.Spec.Reloader.IntervalSeconds
	}
	return DefaultReloadInterval
}

func HotReloadEnabled(haproxy *operatorv1alpha1.HAProxyCapsule) bool {
	return haproxy.Spec.ReloadStrategy != operatorv1alpha1.HAProxyReloadStrategyRestart
}

func GetMaxConnections(haproxy *operatorv1alpha1.HAProxyCapsule) int32 {
	if haproxy.Spec.MaxConnections > 0 {
		return haproxy.Spec.MaxConnections
	}
	return DefaultMaxConnections
}

func durationMs(d *metav1.Duration, fallback time.Duration) int64 {
	if d != nil && d.Duration > 0 {
		return d.Milliseconds()
	}
	return fallback.Milliseconds()
}

func getTimeouts(haproxy *operatorv1alpha1.HAProxyCapsule) timeoutsData {
	t := haproxy.Spec.Timeouts
	if t == nil {
		t = &operatorv1alpha1.HAProxyTimeoutsSpec{}
	}
	return timeoutsData{
		Connect:       durationMs(t.Connect, defaultConnectTimeout),
		Client:        durationMs(t.Client, defaultClientTimeout),
		Server:        durationMs(t.Server, defaultServerTimeout),
		Tunnel:        durationMs(t.Tunnel, defaultTunnelTimeout),
		HTTPKeepAlive: durationMs(t.HTTPKeepAlive, 0),
		HardStopAfter: durationMs(t.HardStopAfter, defaultHardStopAfterTimeout),
	}
}

type sourceKind string

const (
	secretSource    sourceKind = "Secret"
	configMapSource sourceKind = "ConfigMap"
)

type source struct {
	kind  sourceKind
	name  string
	items []corev1.KeyToPath
}

func (s source) projection() corev1.VolumeProjection {
	ref := corev1.LocalObjectReference{Name: s.name}
	if s.kind == configMapSource {
		return corev1.VolumeProjection{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: ref, Items: s.items}}
	}
	return corev1.VolumeProjection{Secret: &corev1.SecretProjection{LocalObjectReference: ref, Items: s.items}}
}

type sourceVolume struct {
	name      string
	mountPath string
	sources   []source
}

type inputs struct {
	grpc    backendData
	grpcs   *backendData
	acls    []string
	volumes []sourceVolume
}

func resolve(haproxy *operatorv1alpha1.HAProxyCapsule) inputs {
	tls := sourceVolume{name: TLSVolumeName, mountPath: TLSMountPath}
	acl := sourceVolume{name: ACLVolumeName, mountPath: ACLMountPath}

	var backends operatorv1alpha1.HAProxyBackendsSpec
	if haproxy.Spec.Backends != nil {
		backends = *haproxy.Spec.Backends
	}
	host := GetUpstreamService(haproxy)
	in := inputs{grpc: resolveBackend(host, backends.Grpc, "consensus_nodes", operatorv1alpha1.HAProxyBackendProtocolHTTP)}

	if name := haproxy.Spec.TLSSecretName; name != "" {
		// HAProxy auto-pairs tls.crt with tls.crt.key, which keeps cert-manager secrets usable as-is.
		tls.sources = []source{{kind: secretSource, name: name, items: []corev1.KeyToPath{
			{Key: "tls.crt", Path: "tls.crt"},
			{Key: "tls.key", Path: "tls.crt.key"},
		}}}
		grpcs := resolveBackend(host, backends.Grpcs, "consensus_nodes_tls", operatorv1alpha1.HAProxyBackendProtocolHTTPS)
		in.grpcs = &grpcs
	}

	for _, f := range haproxy.Spec.ACLFiles {
		file := f.Name + ".acl"
		acl.sources = append(acl.sources, source{kind: configMapSource, name: f.ConfigMapRef.Name, items: []corev1.KeyToPath{{Key: f.ConfigMapRef.Key, Path: file}}})
		in.acls = append(in.acls, ACLMountPath+"/"+file)
	}

	for _, v := range []sourceVolume{tls, acl} {
		if len(v.sources) > 0 {
			in.volumes = append(in.volumes, v)
		}
	}
	return in
}

func resolveBackend(host string, spec *operatorv1alpha1.HAProxyBackendSpec, name string, defaultProtocol operatorv1alpha1.HAProxyBackendProtocol) backendData {
	if spec == nil {
		spec = &operatorv1alpha1.HAProxyBackendSpec{}
	}
	https := cmp.Or(spec.Protocol, defaultProtocol) == operatorv1alpha1.HAProxyBackendProtocolHTTPS
	port := spec.Port
	if port == 0 {
		port = common.GrpcPort
		if https {
			port = common.GrpcsPort
		}
	}
	bd := backendData{
		Name:      name,
		Host:      host,
		Port:      port,
		HTTPS:     https,
		GRPCCheck: spec.HealthCheck == operatorv1alpha1.HAProxyBackendHealthCheckGRPC,
	}
	if https && spec.TLS != nil {
		bd.ServerName = spec.TLS.ServerName
	}
	return bd
}

func referencedNames(haproxy *operatorv1alpha1.HAProxyCapsule, kind sourceKind) []string {
	var names []string
	for _, v := range resolve(haproxy).volumes {
		for _, s := range v.sources {
			if s.kind == kind {
				names = append(names, s.name)
			}
		}
	}
	return names
}

func ReferencedSecrets(haproxy *operatorv1alpha1.HAProxyCapsule) []string {
	return referencedNames(haproxy, secretSource)
}

func ReferencedConfigMaps(haproxy *operatorv1alpha1.HAProxyCapsule) []string {
	return referencedNames(haproxy, configMapSource)
}

func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.HAProxyCapsule] {
	return subreconciler.NewChain(
		NewConfigMapSubReconciler(c, scheme),
		NewDeploymentSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewIngressSubReconciler(c, scheme),
	)
}
