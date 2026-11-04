// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func reconcileDeployment(t *testing.T, c client.Client, haproxy *operatorv1alpha1.HAProxyCapsule) *appsv1.Deployment {
	t.Helper()
	ctx := context.Background()
	if _, err := NewDeploymentSubReconciler(c, newTestScheme()).Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, deploy); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	return deploy
}

func findContainer(t *testing.T, deploy *appsv1.Deployment, name string) *corev1.Container {
	t.Helper()
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == name {
			return &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	t.Fatalf("container %s not found", name)
	return nil
}

func assertConfig(t *testing.T, cfg string, want, unwanted []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(cfg, w) {
			t.Errorf("expected %q in config:\n%s", w, cfg)
		}
	}
	for _, u := range unwanted {
		if strings.Contains(cfg, u) {
			t.Errorf("unexpected %q in config:\n%s", u, cfg)
		}
	}
}

func findVolume(deploy *appsv1.Deployment, name string) *corev1.Volume {
	for i := range deploy.Spec.Template.Spec.Volumes {
		if deploy.Spec.Template.Spec.Volumes[i].Name == name {
			return &deploy.Spec.Template.Spec.Volumes[i]
		}
	}
	return nil
}

func envValue(c *corev1.Container, name string) string {
	for _, e := range c.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

func TestDeployment_HotReloadDefault(t *testing.T) {
	haproxy := newTestHAProxy()
	haproxy.Spec.TLSSecretName = "tls"
	haproxy.Spec.ACLFiles = []operatorv1alpha1.HAProxyACLFile{
		{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "node0-ofac-list", Key: "list"}},
	}
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)

	if _, ok := deploy.Spec.Template.Annotations[ConfigChecksumAnnotation]; ok {
		t.Error("HotReload must not put the config checksum on the pod template")
	}

	main := findContainer(t, deploy, ContainerName)
	wantArgs := []string{"-W", "-db", "-f", ConfigFilePath, "-S", "ipv4@127.0.0.1:9999"}
	if !slices.Equal(main.Command, []string{"haproxy"}) || !slices.Equal(main.Args, wantArgs) {
		t.Errorf("unexpected haproxy command %v args %v", main.Command, main.Args)
	}
	for _, m := range main.VolumeMounts {
		if m.SubPath != "" {
			t.Errorf("mount %s uses subPath; kubelet would not propagate updates", m.Name)
		}
	}

	reloader := findContainer(t, deploy, ReloaderContainerName)
	if reloader.Image != haproxy.Spec.Image {
		t.Errorf("reloader image should default to haproxy image, got %s", reloader.Image)
	}
	if !slices.Equal(reloader.Command, []string{"/bin/sh", "-c", reloaderScript}) {
		t.Error("reloader must run the embedded script")
	}
	wantDirs := strings.Join([]string{ConfigMountPath, TLSMountPath, ACLMountPath}, " ")
	if got := envValue(reloader, "WATCH_DIRS"); got != wantDirs {
		t.Errorf("WATCH_DIRS = %q, want %q", got, wantDirs)
	}
	if got := envValue(reloader, "RELOAD_INTERVAL_SECONDS"); got != "5" {
		t.Errorf("RELOAD_INTERVAL_SECONDS = %q, want 5", got)
	}
	if !equality.Semantic.DeepEqual(reloader.VolumeMounts, main.VolumeMounts) {
		t.Error("reloader must see the same mounts as haproxy")
	}
}

func TestDeployment_ReloaderOverrides(t *testing.T) {
	haproxy := newTestHAProxy()
	haproxy.Spec.Reloader = &operatorv1alpha1.HAProxyReloaderSpec{Image: "busybox:1.37.0-musl", IntervalSeconds: 2}
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)

	reloader := findContainer(t, deploy, ReloaderContainerName)
	if reloader.Image != "busybox:1.37.0-musl" {
		t.Errorf("reloader image override ignored: %s", reloader.Image)
	}
	if got := envValue(reloader, "RELOAD_INTERVAL_SECONDS"); got != "2" {
		t.Errorf("RELOAD_INTERVAL_SECONDS = %q, want 2", got)
	}
}

func TestDeployment_RestartStrategy(t *testing.T) {
	haproxy := newTestHAProxy()
	haproxy.Spec.ReloadStrategy = operatorv1alpha1.HAProxyReloadStrategyRestart
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)

	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Restart strategy must not add the reloader, got %d containers", len(deploy.Spec.Template.Spec.Containers))
	}
	if slices.Contains(deploy.Spec.Template.Spec.Containers[0].Args, "-S") {
		t.Error("Restart strategy must not expose the master CLI")
	}
}

func TestDeployment_RestartChecksumTracksSourceData(t *testing.T) {
	ctx := context.Background()
	haproxy := newTestHAProxy()
	haproxy.Spec.ReloadStrategy = operatorv1alpha1.HAProxyReloadStrategyRestart
	haproxy.Spec.TLSSecretName = "tls"
	haproxy.Spec.ACLFiles = []operatorv1alpha1.HAProxyACLFile{
		{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "ofac", Key: "list"}},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: haproxy.Namespace},
		Data:       map[string][]byte{"tls.crt": []byte("a"), "tls.key": []byte("b")},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "ofac", Namespace: haproxy.Namespace},
		Data:       map[string]string{"list": "1.2.3.4\n", "unrelated": "x"},
	}
	c := newTestClient(newTestScheme(), haproxy, secret, cm)
	checksum := func() string {
		return reconcileDeployment(t, c, haproxy).Spec.Template.Annotations[ConfigChecksumAnnotation]
	}

	initial := checksum()

	cm.Data["unrelated"] = "y"
	if err := c.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	if checksum() != initial {
		t.Error("unreferenced ConfigMap keys must not roll the pod")
	}

	cm.Data["list"] = "1.2.3.4\n5.6.7.8\n"
	if err := c.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	afterACL := checksum()
	if afterACL == initial {
		t.Error("ACL list change must change the checksum")
	}

	secret.Data["tls.crt"] = []byte("rotated")
	if err := c.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	if checksum() == afterACL {
		t.Error("TLS secret rotation must change the checksum")
	}
}

func TestDeployment_MissingSourcesDoNotBlock(t *testing.T) {
	haproxy := newTestHAProxy()
	haproxy.Spec.ReloadStrategy = operatorv1alpha1.HAProxyReloadStrategyRestart
	haproxy.Spec.TLSSecretName = "not-yet-issued"
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)
	if deploy.Spec.Template.Annotations[ConfigChecksumAnnotation] == "" {
		t.Error("expected checksum even when the TLS secret is missing")
	}
}

func TestDeployment_ACLVolume(t *testing.T) {
	haproxy := newTestHAProxy()
	haproxy.Spec.TLSSecretName = "tls"
	haproxy.Spec.ACLFiles = []operatorv1alpha1.HAProxyACLFile{
		{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "node0-ofac-list", Key: "list"}},
		{Name: "extra", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "extra-cm", Key: "ips"}},
	}
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)

	acl := findVolume(deploy, ACLVolumeName)
	if acl == nil || acl.Projected == nil || len(acl.Projected.Sources) != 2 {
		t.Fatalf("expected projected ACL volume with 2 sources, got %+v", acl)
	}
	src := acl.Projected.Sources[0].ConfigMap
	if src.Name != "node0-ofac-list" || src.Items[0].Key != "list" || src.Items[0].Path != "ofac.acl" {
		t.Errorf("unexpected ACL projection %+v", src)
	}
}

func TestDeployment_NoOptionalVolumesByDefault(t *testing.T) {
	haproxy := newTestHAProxy()
	deploy := reconcileDeployment(t, newTestClient(newTestScheme(), haproxy), haproxy)
	for _, name := range []string{TLSVolumeName, ACLVolumeName} {
		if findVolume(deploy, name) != nil {
			t.Errorf("unexpected volume %s", name)
		}
	}
}

func TestHAProxyConfig_Defaults(t *testing.T) {
	cfg := mustHAProxyConfig(t, &operatorv1alpha1.HAProxyCapsule{ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"}})
	assertConfig(t, cfg, []string{
		"maxconn 10000",
		"hard-stop-after 1800000ms",
		"ssl-default-bind-options ssl-min-ver TLSv1.2",
		"bind *:50211 proto h2",
		"bind *:50212 proto h2",
		"timeout connect 10000ms",
		"timeout client 120000ms",
		"timeout server 120000ms",
		"timeout tunnel 3600000ms",
		"http-reuse safe",
		"http-request use-service prometheus-exporter if { path /metrics }",
	}, []string{"option httpchk", "timeout http-keep-alive", " -f /usr/local/etc/haproxy/acl"})
}

func TestHAProxyConfig_TimeoutsAndMaxConn(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			MaxConnections: 500,
			Timeouts: &operatorv1alpha1.HAProxyTimeoutsSpec{
				Client:        &metav1.Duration{Duration: 30 * time.Second},
				HTTPKeepAlive: &metav1.Duration{Duration: 10 * time.Second},
				HardStopAfter: &metav1.Duration{Duration: 5 * time.Minute},
			},
		},
	}
	assertConfig(t, mustHAProxyConfig(t, haproxy), []string{
		"maxconn 500",
		"timeout client 30000ms",
		"timeout server 120000ms",
		"timeout http-keep-alive 10000ms",
		"hard-stop-after 300000ms",
	}, nil)
}

func TestHAProxyConfig_ACLFiles(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			TLSSecretName: "tls",
			ACLFiles: []operatorv1alpha1.HAProxyACLFile{
				{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "a", Key: "b"}},
				{Name: "extra", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "c", Key: "d"}},
			},
		},
	}
	cfg := mustHAProxyConfig(t, haproxy)
	for _, want := range []string{
		"tcp-request connection reject if { src -f /usr/local/etc/haproxy/acl/ofac.acl }",
		"http-request deny if { hdr_ip(x-forwarded-for) -f /usr/local/etc/haproxy/acl/ofac.acl }",
		"tcp-request connection reject if { src -f /usr/local/etc/haproxy/acl/extra.acl }",
		"http-request deny if { hdr_ip(x-forwarded-for) -f /usr/local/etc/haproxy/acl/extra.acl }",
	} {
		if got := strings.Count(cfg, want); got != 2 {
			t.Errorf("expected %q on both frontends, found %d times:\n%s", want, got, cfg)
		}
	}
	// HAProxy warns when tcp-request rules follow http-request rules.
	if strings.Index(cfg, "tcp-request connection reject if { src -f /usr/local/etc/haproxy/acl/extra.acl }") > strings.Index(cfg, "http-request deny") {
		t.Error("all tcp-request rules must precede http-request rules")
	}
}

func TestHAProxyConfig_Backends(t *testing.T) {
	tests := []struct {
		name     string
		tls      bool
		upstream string
		backends *operatorv1alpha1.HAProxyBackendsSpec
		want     []string
		unwanted []string
	}{
		{
			name: "grpc over HTTPS with verify none by default",
			backends: &operatorv1alpha1.HAProxyBackendsSpec{
				Grpc: &operatorv1alpha1.HAProxyBackendSpec{Protocol: operatorv1alpha1.HAProxyBackendProtocolHTTPS},
			},
			want:     []string{"server upstream node1:50212 ssl alpn h2 verify none check inter 10s downinter 10s observe layer4 error-limit 5 on-error mark-down resolvers default init-addr last,libc,none"},
			unwanted: []string{"proto h2 check"},
		},
		{
			name: "custom port",
			backends: &operatorv1alpha1.HAProxyBackendsSpec{
				Grpc: &operatorv1alpha1.HAProxyBackendSpec{Port: 60211},
			},
			want: []string{"server upstream node1:60211 proto h2 check"},
		},
		{
			name: "grpc health check over h2c",
			backends: &operatorv1alpha1.HAProxyBackendsSpec{
				Grpc: &operatorv1alpha1.HAProxyBackendSpec{HealthCheck: operatorv1alpha1.HAProxyBackendHealthCheckGRPC},
			},
			want: []string{
				"option httpchk",
				"http-check send meth POST uri /grpc.health.v1.Health/Check ver HTTP/2 hdr content-type application/grpc hdr te trailers",
				"http-check expect status 200",
				"proto h2 check check-proto h2 inter",
			},
		},
		{
			name: "grpcs SNI with grpc health check",
			tls:  true,
			backends: &operatorv1alpha1.HAProxyBackendsSpec{
				Grpcs: &operatorv1alpha1.HAProxyBackendSpec{
					HealthCheck: operatorv1alpha1.HAProxyBackendHealthCheckGRPC,
					TLS:         &operatorv1alpha1.HAProxyBackendTLSSpec{ServerName: "node1.example"},
				},
			},
			want: []string{"server upstream node1:50212 ssl alpn h2 sni str(node1.example) verify none check check-alpn h2 inter"},
		},
		{
			name:     "external upstream by IP",
			upstream: "10.20.30.40",
			backends: &operatorv1alpha1.HAProxyBackendsSpec{
				Grpc: &operatorv1alpha1.HAProxyBackendSpec{Protocol: operatorv1alpha1.HAProxyBackendProtocolHTTPS},
			},
			want: []string{"server upstream 10.20.30.40:50212 ssl alpn h2 verify none check"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
				Spec:       operatorv1alpha1.HAProxyCapsuleSpec{Backends: tt.backends, UpstreamService: tt.upstream},
			}
			if tt.tls {
				haproxy.Spec.TLSSecretName = "tls"
			}
			assertConfig(t, mustHAProxyConfig(t, haproxy), tt.want, tt.unwanted)
		})
	}
}

func TestReferencedSources(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			TLSSecretName: "tls",
			ACLFiles: []operatorv1alpha1.HAProxyACLFile{
				{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: "node0-ofac-list", Key: "k"}},
			},
		},
	}
	if got := ReferencedSecrets(haproxy); !slices.Equal(got, []string{"tls"}) {
		t.Errorf("ReferencedSecrets = %v", got)
	}
	if got := ReferencedConfigMaps(haproxy); !slices.Equal(got, []string{"node0-ofac-list"}) {
		t.Errorf("ReferencedConfigMaps = %v", got)
	}

	haproxy.Spec.TLSSecretName = ""
	if got := ReferencedSecrets(haproxy); len(got) != 0 {
		t.Errorf("expected no secrets without TLS termination, got %v", got)
	}
}
