// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"cmp"
	_ "embed"
	"maps"
	"net/netip"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

//go:embed tmpl/envoy.yaml.tmpl
var envoyYamlTmpl string

//go:embed tmpl/lds.yaml.tmpl
var ldsTmpl string

//go:embed tmpl/cds.yaml.tmpl
var cdsTmpl string

//go:embed tmpl/sds.yaml.tmpl
var sdsTmpl string

var (
	bootstrapTemplate = template.Must(template.New(ConfigFileName).Parse(envoyYamlTmpl))
	ldsTemplate       = template.Must(template.New(LDSFileName).Parse(ldsTmpl))
	cdsTemplate       = template.Must(template.New(CDSFileName).Parse(cdsTmpl))
	sdsTemplate       = template.Must(template.New("sds.yaml").Parse(sdsTmpl))
)

type bootstrapData struct {
	NodeID         string
	NodeCluster    string
	AdminAddress   string
	AdminPort      int32
	MetricsPort    int32
	MaxConnections int32
	ConfigDir      string
}

type xdsData struct {
	Listeners      []Listener
	ACLRanges      string
	Timeouts       timeoutsData
	MaxConnections int32
	ConfigDir      string
}

type timeoutsData struct {
	Connect string
	Client  string
	Server  string
	Stream  string
}

type corsData struct {
	AllowOrigins  []string
	AllowMethods  string
	AllowHeaders  string
	MaxAge        string
	ExposeHeaders string
}

type upstreamData struct {
	Host       string
	Port       int32
	HTTPS      bool
	ServerName string
	GRPCCheck  bool
}

// Listener is a resolved traffic listener; Name is the Envoy name, PortName the Kubernetes one.
type Listener struct {
	Name              string
	PortName          string
	Port              int32
	TLSSecretName     string
	ALPN              string
	CORS              *corsData
	TrustedProxyCIDRs string
	Upstream          upstreamData
	corsSpec          *operatorv1alpha1.CORSConfig
	grpcWeb           bool
}

func (l Listener) TLSVolume() string { return "tls-" + l.PortName }
func (l Listener) TLSDir() string    { return TLSMountRoot + "/" + l.PortName }

func ResolveListeners(envoy *operatorv1alpha1.EnvoyProxy) []Listener {
	spec := envoy.Spec.Listeners
	if spec == nil {
		spec = &operatorv1alpha1.EnvoyListenersSpec{GRPCWeb: &operatorv1alpha1.EnvoyGRPCWebListenerSpec{}}
	}
	host := GetUpstreamService(envoy)
	var out []Listener
	if l := spec.GRPCWeb; l != nil {
		web := newListener("grpc_web", "grpc-web", DefaultGRPCWebPort, "h2, http/1.1", operatorv1alpha1.EnvoyUpstreamProtocolHTTP, host, l.EnvoyListenerSpec)
		web.grpcWeb = true
		web.corsSpec = l.CORS
		web.TrustedProxyCIDRs = rangesYAML(parseCIDRs(l.TrustedProxyCIDRs))
		out = append(out, web)
	}
	if l := spec.GRPC; l != nil {
		out = append(out, newListener("grpc", "grpc", common.GrpcPort, "h2", operatorv1alpha1.EnvoyUpstreamProtocolHTTP, host, *l))
	}
	if l := spec.GRPCS; l != nil {
		out = append(out, newListener("grpcs", "grpcs", common.GrpcsPort, "h2", operatorv1alpha1.EnvoyUpstreamProtocolHTTPS, host, *l))
	}
	return out
}

func newListener(name, portName string, defaultPort int32, alpn string, defaultProtocol operatorv1alpha1.EnvoyUpstreamProtocol, host string, spec operatorv1alpha1.EnvoyListenerSpec) Listener {
	up := spec.Upstream
	if up == nil {
		up = &operatorv1alpha1.EnvoyUpstreamSpec{}
	}
	https := cmp.Or(up.Protocol, defaultProtocol) == operatorv1alpha1.EnvoyUpstreamProtocolHTTPS
	port := up.Port
	if port == 0 {
		port = common.GrpcPort
		if https {
			port = common.GrpcsPort
		}
	}
	return Listener{
		Name:          name,
		PortName:      portName,
		Port:          cmp.Or(spec.Port, defaultPort),
		TLSSecretName: spec.TLSSecretName,
		ALPN:          alpn,
		Upstream: upstreamData{
			Host:       host,
			Port:       port,
			HTTPS:      https,
			ServerName: up.ServerName,
			GRPCCheck:  up.HealthCheck == operatorv1alpha1.EnvoyHealthCheckGRPC,
		},
	}
}

func RenderConfig(envoy *operatorv1alpha1.EnvoyProxy, acl []netip.Prefix) (map[string]string, error) {
	bootstrap, err := BootstrapConfig(envoy)
	if err != nil {
		return nil, err
	}
	listeners := ResolveListeners(envoy)
	for i := range listeners {
		if listeners[i].grpcWeb {
			if listeners[i].CORS, err = corsConfig(listeners[i].corsSpec); err != nil {
				return nil, err
			}
		}
	}
	data := xdsData{
		Listeners:      listeners,
		ACLRanges:      rangesYAML(acl),
		Timeouts:       getTimeouts(envoy),
		MaxConnections: GetMaxConnections(envoy),
		ConfigDir:      ConfigMountPath,
	}

	files := map[string]string{ConfigFileName: bootstrap}
	if files[LDSFileName], err = execute(ldsTemplate, data); err != nil {
		return nil, err
	}
	if files[CDSFileName], err = execute(cdsTemplate, data); err != nil {
		return nil, err
	}
	for _, l := range listeners {
		if l.TLSSecretName == "" {
			continue
		}
		if files["sds-"+l.Name+".yaml"], err = execute(sdsTemplate, l); err != nil {
			return nil, err
		}
	}
	maps.Copy(files, envoy.Spec.Files)

	size := 0
	for _, f := range files {
		size += len(f)
	}
	if size > maxRenderedSize {
		return nil, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("rendered Envoy config is %d bytes, over the %d byte ConfigMap budget", size, maxRenderedSize), ReasonConfigTooLarge),
			"Reduce the ACL entries; trustedProxyCIDRs on the grpcWeb listener adds a second copy of the list",
		)
	}
	return files, nil
}

func BootstrapConfig(envoy *operatorv1alpha1.EnvoyProxy) (string, error) {
	adminAddress := "127.0.0.1"
	if envoy.Spec.ExposeAdmin {
		adminAddress = "0.0.0.0"
	}
	return execute(bootstrapTemplate, bootstrapData{
		NodeID:         cmp.Or(envoy.Name, "envoy"),
		NodeCluster:    cmp.Or(envoy.Namespace, "default"),
		AdminAddress:   adminAddress,
		AdminPort:      GetAdminPort(envoy),
		MetricsPort:    GetMetricsPort(envoy),
		MaxConnections: GetMaxConnections(envoy),
		ConfigDir:      ConfigMountPath,
	})
}

func execute(t *template.Template, data any) (string, error) {
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", errx.WithReason(errTemplate.Wrap(err, "envoy %s template execution failed", t.Name()), ReasonTemplateFailed)
	}
	return b.String(), nil
}

func yamlEscapeString(s string) (string, error) {
	if strings.ContainsAny(s, "\n\r") {
		return "", errx.WithHints(
			errx.WithReason(errConfigInvalid.New("CORS value contains newline, which is not allowed: %q", s), ReasonCORSValueInvalid),
			"Remove newline characters from spec.listeners.grpcWeb.cors",
		)
	}
	return strings.ReplaceAll(s, "'", "''"), nil
}

func corsConfig(c *operatorv1alpha1.CORSConfig) (*corsData, error) {
	if c == nil {
		c = &operatorv1alpha1.CORSConfig{}
	}
	origins := c.AllowOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	allowHeaders := "keep-alive,user-agent,cache-control,content-type,content-transfer-encoding,custom-header-1,x-accept-content-transfer-encoding,x-accept-response-streaming,x-user-agent,x-grpc-web,grpc-timeout"
	if len(c.AllowHeaders) > 0 {
		allowHeaders = strings.Join(c.AllowHeaders, ",")
	}
	exposeHeaders := "custom-header-1,grpc-status,grpc-message"
	if len(c.ExposeHeaders) > 0 {
		exposeHeaders = strings.Join(c.ExposeHeaders, ",")
	}
	raw := append([]string{cmp.Or(c.AllowMethods, "GET, PUT, DELETE, POST, OPTIONS"), allowHeaders, exposeHeaders, cmp.Or(c.MaxAge, "1728000")}, origins...)
	esc := make([]string, len(raw))
	for i, v := range raw {
		var err error
		if esc[i], err = yamlEscapeString(v); err != nil {
			return nil, err
		}
	}
	return &corsData{AllowMethods: esc[0], AllowHeaders: esc[1], ExposeHeaders: esc[2], MaxAge: esc[3], AllowOrigins: esc[4:]}, nil
}

func parseCIDRs(cidrs []operatorv1alpha1.EnvoyCIDR) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		if p, err := netip.ParsePrefix(string(c)); err == nil {
			prefixes = append(prefixes, p.Masked())
		}
	}
	return prefixes
}

func rangesYAML(prefixes []netip.Prefix) string {
	if len(prefixes) == 0 {
		return ""
	}
	b := make([]byte, 0, len(prefixes)*48)
	b = append(b, '[')
	for i, p := range prefixes {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = append(b, "{address_prefix: '"...)
		b = p.Addr().AppendTo(b)
		b = append(b, "', prefix_len: "...)
		b = strconv.AppendInt(b, int64(p.Bits()), 10)
		b = append(b, '}')
	}
	return string(append(b, ']'))
}

func formatDuration(d *metav1.Duration, fallback time.Duration) string {
	if d != nil && d.Duration > 0 {
		fallback = d.Duration
	}
	return strconv.FormatFloat(fallback.Seconds(), 'f', -1, 64) + "s"
}

func getTimeouts(envoy *operatorv1alpha1.EnvoyProxy) timeoutsData {
	t := envoy.Spec.Timeouts
	if t == nil {
		t = &operatorv1alpha1.EnvoyTimeoutsSpec{}
	}
	return timeoutsData{
		Connect: formatDuration(t.Connect, defaultConnectTimeout),
		Client:  formatDuration(t.Client, defaultClientTimeout),
		Server:  formatDuration(t.Server, defaultServerTimeout),
		Stream:  formatDuration(t.Stream, defaultStreamTimeout),
	}
}
