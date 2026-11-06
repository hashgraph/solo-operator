// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func reasonOf(err error) errx.Reason {
	r, _ := errx.ReasonOf(err)
	return r
}

func assertContains(t *testing.T, cfg string, want, unwanted []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(cfg, w) {
			t.Errorf("expected %q in:\n%s", w, cfg)
		}
	}
	for _, u := range unwanted {
		if strings.Contains(cfg, u) {
			t.Errorf("unexpected %q in:\n%s", u, cfg)
		}
	}
}

func mustRender(t *testing.T, envoy *operatorv1alpha1.EnvoyProxy, acl []netip.Prefix) map[string]string {
	t.Helper()
	files, err := RenderConfig(envoy, acl)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}
	return files
}

func allListeners(tlsSecret string) *operatorv1alpha1.EnvoyListenersSpec {
	return &operatorv1alpha1.EnvoyListenersSpec{
		GRPCWeb: &operatorv1alpha1.EnvoyGRPCWebListenerSpec{},
		GRPC:    &operatorv1alpha1.EnvoyListenerSpec{},
		GRPCS:   &operatorv1alpha1.EnvoyListenerSpec{TLSSecretName: tlsSecret},
	}
}

func TestParseACL(t *testing.T) {
	got, err := parseACL("# header\n203.0.113.7\n\n198.51.100.9/24  # inline\n2001:db8::1/32\n2001:db8::5\n", "cm/key")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[203.0.113.7/32 198.51.100.0/24 2001:db8::/32 2001:db8::5/128]" {
		t.Errorf("unexpected prefixes %v", got)
	}

	_, err = parseACL("10.0.0.1\nnot-an-ip\n", "cm/key")
	if reasonOf(err) != ReasonACLEntryInvalid || !strings.Contains(err.Error(), "cm/key line 2") {
		t.Errorf("expected ACLEntryInvalid on line 2, got %v", err)
	}
}

func TestLoadACL(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ACLFiles = []operatorv1alpha1.EnvoyConfigMapKeyRef{{Name: "ofac", Key: "list"}, {Name: "extra", Key: "ips"}}

	if _, err := loadACL(ctx, newTestClient(scheme, envoy), envoy); reasonOf(err) != ReasonACLSourceMissing {
		t.Errorf("expected ACLSourceMissing, got %v", err)
	}

	ofac := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "ofac", Namespace: envoy.Namespace}, Data: map[string]string{"list": "10.0.0.2\n10.0.0.1\n10.0.0.0/16\n"}}
	extra := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "extra", Namespace: envoy.Namespace}, Data: map[string]string{"other": "x"}}
	if _, err := loadACL(ctx, newTestClient(scheme, envoy, ofac, extra), envoy); reasonOf(err) != ReasonACLSourceMissing {
		t.Errorf("expected ACLSourceMissing for absent key, got %v", err)
	}

	extra.Data = map[string]string{"ips": "10.0.0.1\n2001:db8::/32\n192.0.2.1\n"}
	got, err := loadACL(ctx, newTestClient(scheme, envoy, ofac, extra), envoy)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[10.0.0.0/16 192.0.2.1/32 2001:db8::/32]" {
		t.Errorf("expected sorted prefixes without covered entries, got %v", got)
	}
}

func TestRenderConfig_Bootstrap(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.MaxConnections = 500
	envoy.Spec.MetricsPort = 9091
	assertContains(t, mustRender(t, envoy, nil)[ConfigFileName], []string{
		"max_active_downstream_connections: 500",
		"path: /etc/envoy/lds.yaml",
		"path: /etc/envoy/cds.yaml",
		"watched_directory:\n        path: /etc/envoy",
		"id: test-envoy",
		"address: 127.0.0.1",
		"port_value: 9091",
		"prefix_rewrite: /stats/prometheus",
	}, []string{"grpc_web"})

	envoy.Spec.ExposeAdmin = true
	assertContains(t, mustRender(t, envoy, nil)[ConfigFileName], []string{"address: 0.0.0.0\n      port_value: 9901"}, nil)
}

func TestRenderConfig_GRPCWebDefaults(t *testing.T) {
	files := mustRender(t, newTestEnvoy(), nil)
	assertContains(t, files[LDSFileName], []string{
		"name: grpc_web",
		"port_value: 8081",
		"use_remote_address: true",
		"idle_timeout: 120s",
		"stream_idle_timeout: 3600s",
		"timeout: 0s",
		"grpc_timeout_header_max: 0s",
		"envoy.extensions.filters.http.grpc_web.v3.GrpcWeb",
		"- prefix: '*'",
		"allow_headers: 'keep-alive,user-agent,cache-control,content-type,content-transfer-encoding,custom-header-1,x-accept-content-transfer-encoding,x-accept-response-streaming,x-user-agent,x-grpc-web,grpc-timeout'",
		"max_age: '1728000'",
		"expose_headers: 'custom-header-1,grpc-status,grpc-message'",
	}, []string{"filter_chain_matcher", "transport_socket", "name: grpc\n", "name: grpcs\n"})
	assertContains(t, files[CDSFileName], []string{
		"name: node_grpc_web",
		"connect_timeout: 10s",
		"value: 0",
		"max_connections: 10000",
		"tcp_health_check: {}",
		"consecutive_local_origin_failure: 5",
		"max_ejection_percent: 100",
		"address: test-envoy",
		"port_value: 50211",
	}, []string{"http_health_check", "UpstreamTlsContext"})
	for k := range files {
		if strings.HasPrefix(k, "sds-") {
			t.Errorf("unexpected SDS file %s without TLS", k)
		}
	}
}

func TestRenderConfig_CORS(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = &operatorv1alpha1.EnvoyListenersSpec{GRPCWeb: &operatorv1alpha1.EnvoyGRPCWebListenerSpec{
		CORS: &operatorv1alpha1.CORSConfig{
			AllowOrigins:  []string{"https://it's.example.com"},
			AllowMethods:  "GET, POST",
			AllowHeaders:  []string{"content-type", "x-grpc-web"},
			ExposeHeaders: []string{"grpc-status"},
			MaxAge:        "3600",
		},
	}}
	assertContains(t, mustRender(t, envoy, nil)[LDSFileName], []string{
		"- prefix: 'https://it''s.example.com'",
		"allow_methods: 'GET, POST'",
		"allow_headers: 'content-type,x-grpc-web'",
		"expose_headers: 'grpc-status'",
		"max_age: '3600'",
	}, nil)

	envoy.Spec.Listeners.GRPCWeb.CORS.AllowMethods = "GET\nPOST"
	if _, err := RenderConfig(envoy, nil); reasonOf(err) != ReasonCORSValueInvalid {
		t.Errorf("expected CORSValueInvalid, got %v", err)
	}
}

func TestRenderConfig_Listeners(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = allListeners("node-tls")
	envoy.Spec.Listeners.GRPCWeb.TLSSecretName = "web-tls"
	envoy.Spec.Listeners.GRPC.Upstream = &operatorv1alpha1.EnvoyUpstreamSpec{HealthCheck: operatorv1alpha1.EnvoyHealthCheckGRPC}
	envoy.Spec.Listeners.GRPCS.Upstream = &operatorv1alpha1.EnvoyUpstreamSpec{ServerName: "node1.example.com"}
	envoy.Spec.Timeouts = &operatorv1alpha1.EnvoyTimeoutsSpec{Connect: &metav1.Duration{Duration: 250 * time.Millisecond}}
	files := mustRender(t, envoy, nil)

	assertContains(t, files[LDSFileName], []string{
		"name: grpc\n", "port_value: 50211",
		"name: grpcs\n", "port_value: 50212",
		"alpn_protocols: [h2]",
		"alpn_protocols: [h2, http/1.1]",
		"tls_minimum_protocol_version: TLSv1_2",
		"path: /etc/envoy/sds-grpcs.yaml",
		"path: /etc/envoy/sds-grpc_web.yaml",
	}, []string{", DHE-", "[DHE-"})
	assertContains(t, files[CDSFileName], []string{
		"name: node_grpc\n",
		"path: /grpc.health.v1.Health/Check",
		"codec_client_type: HTTP2",
		"name: node_grpcs",
		"UpstreamTlsContext",
		"sni: node1.example.com",
		"port_value: 50212",
		"connect_timeout: 0.25s",
	}, []string{"validation_context"})
	assertContains(t, files["sds-grpcs.yaml"], []string{
		"name: grpcs_tls",
		"filename: /etc/envoy-tls/grpcs/tls.crt",
		"watched_directory:\n        path: /etc/envoy-tls/grpcs",
	}, nil)
	assertContains(t, files["sds-grpc_web.yaml"], []string{"filename: /etc/envoy-tls/grpc-web/tls.key"}, nil)
}

func TestRenderConfig_ACL(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = &operatorv1alpha1.EnvoyListenersSpec{GRPCWeb: &operatorv1alpha1.EnvoyGRPCWebListenerSpec{}, GRPC: &operatorv1alpha1.EnvoyListenerSpec{}}
	acl := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("2001:db8::/32")}

	lds := mustRender(t, envoy, acl)[LDSFileName]
	ranges := "ranges: [{address_prefix: '203.0.113.0', prefix_len: 24}, {address_prefix: '2001:db8::', prefix_len: 32}]"
	if got := strings.Count(lds, ranges); got != 2 {
		t.Errorf("expected the ACL on both listeners, found %d:\n%s", got, lds)
	}
	assertContains(t, lds, []string{"filter_chain_matcher", "name: blocked", "direct_response"}, []string{"envoy.filters.http.rbac", "xff_trusted_cidrs"})

	envoy.Spec.Listeners.GRPCWeb.TrustedProxyCIDRs = []operatorv1alpha1.EnvoyCIDR{"192.0.2.0/24"}
	lds = mustRender(t, envoy, acl)[LDSFileName]
	assertContains(t, lds, []string{"cidrs: [{address_prefix: '192.0.2.0', prefix_len: 24}]", "action: DENY"}, nil)
	if got := strings.Count(lds, "envoy.filters.http.rbac"); got != 1 {
		t.Errorf("expected HTTP RBAC only on grpc_web, found %d", got)
	}
	if got := strings.Count(lds, "use_remote_address: true"); got != 1 {
		t.Errorf("expected grpc to keep use_remote_address, found %d", got)
	}
}

func TestRenderConfig_FilesOverlay(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Files = map[string]string{CDSFileName: "custom", "extra.yaml": "x"}
	files := mustRender(t, envoy, nil)
	if files[CDSFileName] != "custom" || files["extra.yaml"] != "x" || !strings.Contains(files[LDSFileName], "grpc_web") {
		t.Errorf("expected spec.files to overlay rendered files, got keys %v", slices.Sorted(maps.Keys(files)))
	}
}

func TestRenderConfig_TooLarge(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = allListeners("t")
	envoy.Spec.Listeners.GRPCWeb.TrustedProxyCIDRs = []operatorv1alpha1.EnvoyCIDR{"192.0.2.0/24"}
	acl := make([]netip.Prefix, 0, 6000)
	for i := range 6000 {
		acl = append(acl, netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24))
	}
	if _, err := RenderConfig(envoy, acl); reasonOf(err) != ReasonConfigTooLarge {
		t.Errorf("expected ConfigTooLarge, got %v", err)
	}
}

func reconcileEnvoyDeployment(t *testing.T, c client.Client, envoy *operatorv1alpha1.EnvoyProxy) *appsv1.Deployment {
	t.Helper()
	ctx := context.Background()
	if _, err := NewDeploymentSubReconciler(c, newTestScheme()).Reconcile(ctx, envoy); err != nil {
		t.Fatal(err)
	}
	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, deploy); err != nil {
		t.Fatal(err)
	}
	return deploy
}

func TestDeployment_ChecksumOnlyTracksBootstrap(t *testing.T) {
	envoy := newTestEnvoy()
	c := newTestClient(newTestScheme(), envoy)
	checksum := func() string {
		return reconcileEnvoyDeployment(t, c, envoy).Spec.Template.Annotations[ConfigChecksumAnnotation]
	}
	initial := checksum()

	envoy.Spec.ACLFiles = []operatorv1alpha1.EnvoyConfigMapKeyRef{{Name: "a", Key: "b"}}
	envoy.Spec.Files = map[string]string{CDSFileName: "custom"}
	if checksum() != initial {
		t.Error("xDS-only changes must not roll the pod")
	}

	envoy.Spec.MaxConnections = 20000
	afterBootstrap := checksum()
	if afterBootstrap == initial {
		t.Error("bootstrap changes must roll the pod")
	}

	envoy.Spec.Files = map[string]string{ConfigFileName: "custom bootstrap"}
	if checksum() == afterBootstrap {
		t.Error("an overridden bootstrap must roll the pod")
	}
}

func TestDeployment_ListenersAndResources(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = allListeners("node-tls")
	envoy.Spec.Listeners.GRPCWeb.TLSSecretName = "web-tls"
	envoy.Spec.Timeouts = &operatorv1alpha1.EnvoyTimeoutsSpec{Drain: &metav1.Duration{Duration: 5 * time.Minute}}
	envoy.Spec.Resources = &corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")}}
	deploy := reconcileEnvoyDeployment(t, newTestClient(newTestScheme(), envoy), envoy)
	container := deploy.Spec.Template.Spec.Containers[0]

	if !slices.Equal(container.Args, []string{"-c", "/etc/envoy/envoy.yaml", "--drain-time-s", "300"}) {
		t.Errorf("unexpected args %v", container.Args)
	}
	if container.Resources.Limits.Memory().String() != "512Mi" {
		t.Errorf("resources not applied: %+v", container.Resources)
	}
	ports := map[string]int32{}
	for _, p := range container.Ports {
		ports[p.Name] = p.ContainerPort
	}
	if ports["grpc"] != 50211 || ports["grpcs"] != 50212 || ports["grpc-web"] != DefaultGRPCWebPort {
		t.Errorf("unexpected container ports %v", ports)
	}
	if container.ReadinessProbe.TCPSocket == nil || container.ReadinessProbe.TCPSocket.Port.IntVal != DefaultGRPCWebPort {
		t.Errorf("expected readiness on the first listener, got %+v", container.ReadinessProbe)
	}

	secrets := map[string]string{}
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Secret != nil {
			secrets[v.Name] = v.Secret.SecretName
		}
	}
	if secrets["tls-grpcs"] != "node-tls" || secrets["tls-grpc-web"] != "web-tls" || len(secrets) != 2 {
		t.Errorf("unexpected TLS volumes %v", secrets)
	}
	mounts := map[string]string{}
	for _, m := range container.VolumeMounts {
		mounts[m.Name] = m.MountPath
	}
	if mounts["tls-grpcs"] != "/etc/envoy-tls/grpcs" || mounts["tls-grpc-web"] != "/etc/envoy-tls/grpc-web" {
		t.Errorf("unexpected TLS mounts %v", mounts)
	}
}

func TestService_ListenerPorts(t *testing.T) {
	envoy := newTestEnvoy()
	envoy.Spec.Listeners = &operatorv1alpha1.EnvoyListenersSpec{GRPC: &operatorv1alpha1.EnvoyListenerSpec{Port: 60211}}
	ports := map[string]int32{}
	for _, p := range NewServiceSubReconciler(nil, newTestScheme()).buildService(envoy).Spec.Ports {
		ports[p.Name] = p.Port
	}
	if ports["grpc"] != 60211 || len(ports) != 2 {
		t.Errorf("expected only metrics and grpc, got %v", ports)
	}
}
