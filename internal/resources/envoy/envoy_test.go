// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestCommonLabels(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-envoy",
			Namespace: "test-namespace",
		},
	}

	labels := CommonLabels(envoy)

	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, labels[LabelManagedBy])
	}
	if labels[LabelPartOf] != PartOfEnvoyProxy {
		t.Errorf("expected part-of label %s, got %s", PartOfEnvoyProxy, labels[LabelPartOf])
	}
	if labels[LabelApp] != "test-envoy" {
		t.Errorf("expected app label test-envoy, got %s", labels[LabelApp])
	}
}

func TestConfigMapName(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-envoy",
		},
	}

	expected := "test-envoy-config"
	if got := ConfigMapName(envoy); got != expected {
		t.Errorf("expected config map name %s, got %s", expected, got)
	}
}

func TestGetImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "default image",
			image:    "",
			expected: DefaultImage,
		},
		{
			name:     "custom image",
			image:    "custom/envoy:latest",
			expected: "custom/envoy:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envoy := &operatorv1alpha1.EnvoyProxy{
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image: tt.image,
				},
			}

			got := GetImage(envoy)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestGetReplicas(t *testing.T) {
	tests := []struct {
		name     string
		replicas *int32
		expected int32
	}{
		{
			name:     "default replicas",
			replicas: nil,
			expected: DefaultReplicas,
		},
		{
			name:     "custom replicas",
			replicas: int32Ptr(5),
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envoy := &operatorv1alpha1.EnvoyProxy{
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Replicas: tt.replicas,
				},
			}

			got := GetReplicas(envoy)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestGetProxyPort(t *testing.T) {
	tests := []struct {
		name      string
		proxyPort int32
		expected  int32
	}{
		{
			name:      "default port",
			proxyPort: 0,
			expected:  DefaultProxyPort,
		},
		{
			name:      "custom port",
			proxyPort: 9090,
			expected:  9090,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envoy := &operatorv1alpha1.EnvoyProxy{
				Spec: operatorv1alpha1.EnvoyProxySpec{
					ProxyPort: tt.proxyPort,
				},
			}

			got := GetProxyPort(envoy)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestGetAdminPort(t *testing.T) {
	tests := []struct {
		name      string
		adminPort int32
		expected  int32
	}{
		{
			name:      "default port",
			adminPort: 0,
			expected:  DefaultAdminPort,
		},
		{
			name:      "custom port",
			adminPort: 9999,
			expected:  9999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envoy := &operatorv1alpha1.EnvoyProxy{
				Spec: operatorv1alpha1.EnvoyProxySpec{
					AdminPort: tt.adminPort,
				},
			}

			got := GetAdminPort(envoy)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	t.Run("default env vars", func(t *testing.T) {
		envoy := &operatorv1alpha1.EnvoyProxy{}

		envVars := BuildEnv(envoy)

		if len(envVars) != 2 {
			t.Fatalf("expected 2 env vars, got %d", len(envVars))
		}

		// Check SERVICE_CLUSTER
		found := false
		for _, e := range envVars {
			if e.Name == "SERVICE_CLUSTER" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected SERVICE_CLUSTER env var")
		}

		// Check SERVICE_NODE
		found = false
		for _, e := range envVars {
			if e.Name == "SERVICE_NODE" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected SERVICE_NODE env var")
		}
	})

	t.Run("custom env vars", func(t *testing.T) {
		envoy := &operatorv1alpha1.EnvoyProxy{
			Spec: operatorv1alpha1.EnvoyProxySpec{
				Env: []corev1.EnvVar{
					{Name: "CUSTOM_VAR", Value: "custom_value"},
				},
			},
		}

		envVars := BuildEnv(envoy)

		if len(envVars) != 1 {
			t.Fatalf("expected 1 env var, got %d", len(envVars))
		}
		if envVars[0].Name != "CUSTOM_VAR" {
			t.Errorf("expected CUSTOM_VAR, got %s", envVars[0].Name)
		}
	})
}

func TestDefaultEnvoyConfig(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		Spec: operatorv1alpha1.EnvoyProxySpec{
			ProxyPort: 8080,
		},
	}

	config := DefaultEnvoyConfig(envoy)

	if _, ok := config[ConfigFileName]; !ok {
		t.Errorf("expected %s to be present in config", ConfigFileName)
	}
}

func TestDefaultEnvoyConfig_PrometheusListener(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		Spec: operatorv1alpha1.EnvoyProxySpec{
			ProxyPort: 8081,
		},
	}

	config := DefaultEnvoyConfig(envoy)
	cfg := config[ConfigFileName]

	if !containsStr(cfg, "prometheus_listener") {
		t.Error("expected prometheus_listener in default envoy config")
	}
	if !containsStr(cfg, "/metrics") {
		t.Error("expected /metrics route in default envoy config")
	}
	if !containsStr(cfg, "/stats/prometheus") {
		t.Error("expected /stats/prometheus prefix_rewrite in default envoy config")
	}
}

func TestDefaultEnvoyConfig_MetricsPort_Default(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{}

	config := DefaultEnvoyConfig(envoy)
	cfg := config[ConfigFileName]

	expected := fmt.Sprintf("port_value: %d", DefaultMetricsPort)
	if !containsStr(cfg, expected) {
		t.Errorf("expected metrics port %d in config, got:\n%s", DefaultMetricsPort, cfg)
	}
}

func TestDefaultEnvoyConfig_MetricsPort_Custom(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		Spec: operatorv1alpha1.EnvoyProxySpec{
			MetricsPort: 9091,
		},
	}

	config := DefaultEnvoyConfig(envoy)
	cfg := config[ConfigFileName]

	if !containsStr(cfg, "port_value: 9091") {
		t.Errorf("expected custom metrics port 9091 in config, got:\n%s", cfg)
	}
}

func TestDefaultEnvoyConfig_CORSDefaults(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{}

	config := DefaultEnvoyConfig(envoy)
	cfg := config[ConfigFileName]

	if !containsStr(cfg, "x-grpc-web") {
		t.Error("expected default gRPC-Web CORS headers in config")
	}
	if !containsStr(cfg, "custom-header-1") {
		t.Error("expected custom-header-1 in default CORS config")
	}
	if !containsStr(cfg, "1728000") {
		t.Error("expected default max_age 1728000 in config")
	}
	if !containsStr(cfg, "grpc-status,grpc-message") {
		t.Error("expected default expose_headers in config")
	}
}

func TestDefaultEnvoyConfig_CORSConfig_Override(t *testing.T) {
	envoy := &operatorv1alpha1.EnvoyProxy{
		Spec: operatorv1alpha1.EnvoyProxySpec{
			CORSConfig: &operatorv1alpha1.CORSConfig{
				AllowOrigins:  []string{"https://app.example.com"},
				AllowMethods:  "GET, POST",
				AllowHeaders:  []string{"content-type", "x-grpc-web"},
				ExposeHeaders: []string{"grpc-status"},
				MaxAge:        "3600",
			},
		},
	}

	config := DefaultEnvoyConfig(envoy)
	cfg := config[ConfigFileName]

	if !containsStr(cfg, "https://app.example.com") {
		t.Error("expected custom AllowOrigins in config")
	}
	if !containsStr(cfg, "GET, POST") {
		t.Error("expected custom AllowMethods in config")
	}
	if !containsStr(cfg, "content-type,x-grpc-web") {
		t.Error("expected custom AllowHeaders in config")
	}
	if !containsStr(cfg, "grpc-status") {
		t.Error("expected custom ExposeHeaders in config")
	}
	if !containsStr(cfg, "3600") {
		t.Error("expected custom MaxAge in config")
	}
	// Default wildcard must not appear when overridden
	if containsStr(cfg, "prefix: '*'") {
		t.Error("default wildcard origin must not appear when AllowOrigins is overridden")
	}
}

func TestDeploymentSubReconciler_Probes(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]

	if container.StartupProbe == nil {
		t.Fatal("expected startup probe to be set")
	}
	if container.StartupProbe.HTTPGet == nil {
		t.Fatal("expected startup probe to use HTTPGet")
	}
	if container.StartupProbe.HTTPGet.Path != "/ready" {
		t.Errorf("expected startup probe path /ready, got %s", container.StartupProbe.HTTPGet.Path)
	}
	if container.StartupProbe.HTTPGet.Port.IntVal != DefaultAdminPort {
		t.Errorf("expected startup probe port %d, got %d", DefaultAdminPort, container.StartupProbe.HTTPGet.Port.IntVal)
	}
	if container.StartupProbe.FailureThreshold == 0 {
		t.Error("expected startup probe failure threshold to be set")
	}

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe to be set")
	}
	if container.LivenessProbe.HTTPGet == nil || container.LivenessProbe.HTTPGet.Path != "/ready" {
		t.Error("expected liveness probe GET /ready")
	}
	if container.LivenessProbe.InitialDelaySeconds != 0 {
		t.Error("liveness probe should have no initial delay (startup probe gates it)")
	}

	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe to be set")
	}
	if container.ReadinessProbe.HTTPGet == nil || container.ReadinessProbe.HTTPGet.Path != "/ready" {
		t.Error("expected readiness probe GET /ready")
	}
}

func TestDefaultEnvoyConfig_TLS(t *testing.T) {
	t.Run("no transport_socket without TLS", func(t *testing.T) {
		envoy := &operatorv1alpha1.EnvoyProxy{}
		cfg := DefaultEnvoyConfig(envoy)[ConfigFileName]
		if containsStr(cfg, "transport_socket") {
			t.Error("transport_socket must not appear when TLSSecretName is empty")
		}
	})

	t.Run("transport_socket with TLS", func(t *testing.T) {
		envoy := &operatorv1alpha1.EnvoyProxy{
			Spec: operatorv1alpha1.EnvoyProxySpec{TLSSecretName: "my-tls-secret"},
		}
		cfg := DefaultEnvoyConfig(envoy)[ConfigFileName]
		if !containsStr(cfg, "transport_socket") {
			t.Error("expected transport_socket in config when TLSSecretName is set")
		}
		if !containsStr(cfg, TLSCertPath) {
			t.Errorf("expected cert path %s in TLS config", TLSCertPath)
		}
		if !containsStr(cfg, TLSKeyPath) {
			t.Errorf("expected key path %s in TLS config", TLSKeyPath)
		}
		if !containsStr(cfg, "DownstreamTlsContext") {
			t.Error("expected DownstreamTlsContext in TLS config")
		}
	})
}

func TestDeployment_TLS(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()

	t.Run("no TLS volume without TLSSecretName", func(t *testing.T) {
		envoy := newTestEnvoy()
		c := newTestClient(scheme, envoy)
		_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		deploy := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, deploy); err != nil {
			t.Fatalf("failed to get deployment: %v", err)
		}
		for _, v := range deploy.Spec.Template.Spec.Volumes {
			if v.Name == TLSVolumeName {
				t.Error("TLS volume must not be present when TLSSecretName is empty")
			}
		}
		if len(deploy.Spec.Template.Spec.Volumes) != 2 {
			t.Errorf("expected 2 volumes, got %d", len(deploy.Spec.Template.Spec.Volumes))
		}
	})

	t.Run("TLS volume present with TLSSecretName", func(t *testing.T) {
		envoy := newTestEnvoy()
		envoy.Spec.TLSSecretName = "my-tls-secret"
		c := newTestClient(scheme, envoy)
		_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		deploy := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, deploy); err != nil {
			t.Fatalf("failed to get deployment: %v", err)
		}
		if len(deploy.Spec.Template.Spec.Volumes) != 3 {
			t.Errorf("expected 3 volumes (config+admin+tls), got %d", len(deploy.Spec.Template.Spec.Volumes))
		}
		found := false
		for _, v := range deploy.Spec.Template.Spec.Volumes {
			if v.Name == TLSVolumeName {
				found = true
				if v.Secret == nil || v.Secret.SecretName != "my-tls-secret" {
					t.Error("TLS volume must reference the configured secret")
				}
			}
		}
		if !found {
			t.Error("TLS volume not found in deployment")
		}
		container := deploy.Spec.Template.Spec.Containers[0]
		foundMount := false
		for _, m := range container.VolumeMounts {
			if m.Name == TLSVolumeName {
				foundMount = true
				if m.MountPath != TLSMountPath {
					t.Errorf("expected TLS mount path %s, got %s", TLSMountPath, m.MountPath)
				}
			}
		}
		if !foundMount {
			t.Error("TLS volume mount not found in container")
		}
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	return scheme
}

func newTestClient(scheme *runtime.Scheme, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
}

func newTestEnvoy() *operatorv1alpha1.EnvoyProxy {
	replicas := int32(2)
	return &operatorv1alpha1.EnvoyProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-envoy",
			Namespace: "test-namespace",
			UID:       "test-uid",
		},
		Spec: operatorv1alpha1.EnvoyProxySpec{
			Image:     "envoyproxy/envoy:v1.21.6",
			Replicas:  &replicas,
			ProxyPort: 8081,
			AdminPort: 9901,
		},
	}
}

func TestConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)

	reconciler := NewConfigMapSubReconciler(c, scheme)

	// First reconcile should create the config map
	result, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify config map was created
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	if cm.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, cm.Labels[LabelManagedBy])
	}

	if cm.Data[ConfigFileName] == "" {
		t.Error("expected envoy.yaml to be set")
	}
}

func TestConfigMapSubReconciler_CustomFiles(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.Files = map[string]string{
		"custom.yaml": "custom config content",
	}
	c := newTestClient(scheme, envoy)

	reconciler := NewConfigMapSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify config map was created with custom files
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	if cm.Data["custom.yaml"] != "custom config content" {
		t.Errorf("expected custom.yaml content, got %s", cm.Data["custom.yaml"])
	}
}

func TestDeploymentSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)

	reconciler := NewDeploymentSubReconciler(c, scheme)

	// First reconcile should create the deployment
	result, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify deployment was created
	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if deploy.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, deploy.Labels[LabelManagedBy])
	}

	// Verify replicas
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 2 {
		t.Error("expected 2 replicas")
	}

	// Verify container
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	if container.Name != ContainerName {
		t.Errorf("expected container name %s, got %s", ContainerName, container.Name)
	}
	if container.Image != envoy.Spec.Image {
		t.Errorf("expected image %s, got %s", envoy.Spec.Image, container.Image)
	}

	// Verify ports
	if len(container.Ports) != 3 {
		t.Fatalf("expected 3 ports (grpc, admin, metrics), got %d", len(container.Ports))
	}

	// Verify probes
	if container.LivenessProbe == nil {
		t.Error("expected liveness probe")
	}
	if container.ReadinessProbe == nil {
		t.Error("expected readiness probe")
	}

	// Verify volumes
	if len(deploy.Spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(deploy.Spec.Template.Spec.Volumes))
	}
}

func TestServiceSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeLoadBalancer
	c := newTestClient(scheme, envoy)

	reconciler := NewServiceSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("expected service type LoadBalancer, got %s", svc.Spec.Type)
	}

	if len(svc.Spec.Ports) != 2 {
		t.Fatalf("expected 2 ports (grpc + metrics), got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Port != envoy.Spec.ProxyPort {
		t.Errorf("expected proxy port %d, got %d", envoy.Spec.ProxyPort, svc.Spec.Ports[0].Port)
	}
	if svc.Spec.Ports[1].Port != DefaultMetricsPort {
		t.Errorf("expected metrics port %d, got %d", DefaultMetricsPort, svc.Spec.Ports[1].Port)
	}
}

func TestServiceSubReconciler_ClusterIP(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeClusterIP
	c := newTestClient(scheme, envoy)

	reconciler := NewServiceSubReconciler(c, scheme)

	_, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected service type ClusterIP, got %s", svc.Spec.Type)
	}
}

func TestServiceSubReconciler_IngressType(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	c := newTestClient(scheme, envoy)

	reconciler := NewServiceSubReconciler(c, scheme)

	_, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	// Ingress type should use ClusterIP service
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ClusterIP service for Ingress type, got %s", svc.Spec.Type)
	}
}

func TestIngressSubReconciler_Skipped(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeLoadBalancer
	c := newTestClient(scheme, envoy)

	reconciler := NewIngressSubReconciler(c, scheme)

	if reconciler.ShouldReconcile(ctx, envoy) {
		t.Error("expected ShouldReconcile to return false for LoadBalancer type")
	}
}

func TestIngressSubReconciler_CreatesIngress(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	envoy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: "envoy.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test-envoy",
										Port: networkingv1.ServiceBackendPort{Number: DefaultProxyPort},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	c := newTestClient(scheme, envoy)

	reconciler := NewIngressSubReconciler(c, scheme)

	if !reconciler.ShouldReconcile(ctx, envoy) {
		t.Fatal("expected ShouldReconcile to return true for Ingress type")
	}

	_, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if len(ing.Spec.Rules) != 1 || ing.Spec.Rules[0].Host != "envoy.example.com" {
		t.Errorf("unexpected ingress spec: %+v", ing.Spec)
	}
}

func TestIngressSubReconciler_SkippedForClusterIP(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeClusterIP
	c := newTestClient(scheme, envoy)

	reconciler := NewIngressSubReconciler(c, scheme)
	if reconciler.ShouldReconcile(ctx, envoy) {
		t.Error("expected ShouldReconcile to return false for ClusterIP type")
	}
}

func TestIngressSubReconciler_NilIngressSpec(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	envoy.Spec.IngressSpec = nil
	c := newTestClient(scheme, envoy)

	reconciler := NewIngressSubReconciler(c, scheme)
	_, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error with nil ingressSpec: %v", err)
	}
	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, ing); err == nil {
		t.Error("expected no ingress to be created when ingressSpec is nil")
	}
}

func TestIngressSubReconciler_IngressClassName(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	className := "nginx"
	pathType := networkingv1.PathTypePrefix
	envoy.Spec.IngressSpec = &networkingv1.IngressSpec{
		IngressClassName: &className,
		Rules: []networkingv1.IngressRule{
			{
				Host: "envoy.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "test-envoy",
									Port: networkingv1.ServiceBackendPort{Number: DefaultProxyPort},
								},
							}},
						},
					},
				},
			},
		},
	}
	c := newTestClient(scheme, envoy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("expected ingressClassName nginx, got %v", ing.Spec.IngressClassName)
	}
}

func TestIngressSubReconciler_Labels(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	envoy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{Host: "envoy.example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-envoy",
								Port: networkingv1.ServiceBackendPort{Number: DefaultProxyPort},
							},
						}},
					},
				},
			}},
		},
	}
	c := newTestClient(scheme, envoy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label, got %v", ing.Labels)
	}
	if ing.Labels[LabelApp] != envoy.Name {
		t.Errorf("expected app label %s, got %s", envoy.Name, ing.Labels[LabelApp])
	}
}

func TestIngressSubReconciler_Update(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	envoy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{Host: "envoy-v1.example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-envoy",
								Port: networkingv1.ServiceBackendPort{Number: DefaultProxyPort},
							},
						}},
					},
				},
			}},
		},
	}
	c := newTestClient(scheme, envoy)
	reconciler := NewIngressSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	// Update the host
	envoy.Spec.IngressSpec.Rules[0].Host = "envoy-v2.example.com"
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Spec.Rules[0].Host != "envoy-v2.example.com" {
		t.Errorf("expected updated host envoy-v2.example.com, got %s", ing.Spec.Rules[0].Host)
	}
}

func TestNewSubReconcilers(t *testing.T) {
	scheme := newTestScheme()
	c := newTestClient(scheme)

	chain := NewSubReconcilers(c, scheme)

	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
}

// --- Node scheduling tests ---

func TestDeploymentSubReconciler_NodeScheduling(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.NodeSelector = map[string]string{"sphere.hashgraph.io/role": "auxiliary"}
	envoy.Spec.Tolerations = []corev1.Toleration{
		{Key: "sphere.hashgraph.io/role", Operator: corev1.TolerationOpEqual, Value: "auxiliary", Effect: corev1.TaintEffectNoSchedule},
	}
	envoy.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"sphere.hashgraph.io/role": "consensus"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	podSpec := deploy.Spec.Template.Spec

	if podSpec.NodeSelector["sphere.hashgraph.io/role"] != "auxiliary" {
		t.Errorf("expected nodeSelector sphere.hashgraph.io/role=auxiliary, got %v", podSpec.NodeSelector)
	}
	if len(podSpec.Tolerations) != 1 || podSpec.Tolerations[0].Key != "sphere.hashgraph.io/role" {
		t.Errorf("expected toleration sphere.hashgraph.io/role, got %v", podSpec.Tolerations)
	}
	if podSpec.Affinity == nil || podSpec.Affinity.PodAntiAffinity == nil {
		t.Error("expected pod anti-affinity to be set")
	}
}

// --- Termination grace period tests ---

func TestDeploymentSubReconciler_TerminationGracePeriod_Default(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	grace := deploy.Spec.Template.Spec.TerminationGracePeriodSeconds
	if grace == nil || *grace != 10 {
		t.Errorf("expected terminationGracePeriodSeconds 10, got %v", grace)
	}
}

func TestDeploymentSubReconciler_TerminationGracePeriod_Custom(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.TerminationGracePeriodSeconds = 30
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	grace := deploy.Spec.Template.Spec.TerminationGracePeriodSeconds
	if grace == nil || *grace != 30 {
		t.Errorf("expected terminationGracePeriodSeconds 30, got %v", grace)
	}
}

// --- AdditionalLabels tests ---

func TestDeploymentSubReconciler_AdditionalLabels(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.AdditionalLabels = map[string]string{
		"solo.hedera.com/node-id":   "0",
		"solo.hedera.com/node-name": "node1",
	}
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if deploy.Labels["solo.hedera.com/node-id"] != "0" {
		t.Errorf("expected deployment label solo.hedera.com/node-id=0, got %v", deploy.Labels)
	}
	if deploy.Spec.Template.Labels["solo.hedera.com/node-name"] != "node1" {
		t.Errorf("expected pod label solo.hedera.com/node-name=node1, got %v", deploy.Spec.Template.Labels)
	}

	// Selector must not include additional labels
	if _, ok := deploy.Spec.Selector.MatchLabels["solo.hedera.com/node-id"]; ok {
		t.Error("additional labels must not bleed into the pod selector")
	}
}

func TestServiceSubReconciler_AdditionalLabels(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.AdditionalLabels = map[string]string{"solo.hedera.com/prometheus-endpoint": "active"}
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Labels["solo.hedera.com/prometheus-endpoint"] != "active" {
		t.Errorf("expected service label solo.hedera.com/prometheus-endpoint=active, got %v", svc.Labels)
	}

	// Selector must not include additional labels
	if _, ok := svc.Spec.Selector["solo.hedera.com/prometheus-endpoint"]; ok {
		t.Error("additional labels must not bleed into the service selector")
	}
}

// --- Service ExternalTrafficPolicy tests ---

func TestServiceSubReconciler_ExternalTrafficPolicy_Default(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy() // LoadBalancer, no explicit ETP
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		t.Errorf("expected ExternalTrafficPolicy Local, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestServiceSubReconciler_ExternalTrafficPolicy_Cluster(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeCluster {
		t.Errorf("expected ExternalTrafficPolicy Cluster, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestServiceSubReconciler_ClusterIP_NoExternalTrafficPolicy(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeClusterIP
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	// ExternalTrafficPolicy must not be set for ClusterIP services
	if svc.Spec.ExternalTrafficPolicy != "" {
		t.Errorf("expected no ExternalTrafficPolicy for ClusterIP, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

// --- ServiceAnnotations tests ---

func TestServiceSubReconciler_ServiceAnnotations(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceAnnotations = map[string]string{
		"cloud.google.com/neg": `{"ingress":true}`,
	}
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Annotations["cloud.google.com/neg"] != `{"ingress":true}` {
		t.Errorf("expected NEG annotation, got %v", svc.Annotations)
	}
}

// --- Metrics port tests ---

func TestDeploymentSubReconciler_MetricsPort_Default(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	found := false
	for _, p := range container.Ports {
		if p.Name == "metrics" && p.ContainerPort == DefaultMetricsPort {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected metrics container port %d, ports: %v", DefaultMetricsPort, container.Ports)
	}
}

func TestDeploymentSubReconciler_MetricsPort_Custom(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.MetricsPort = 9091
	c := newTestClient(scheme, envoy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	found := false
	for _, p := range container.Ports {
		if p.Name == "metrics" && p.ContainerPort == 9091 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected custom metrics port 9091, ports: %v", container.Ports)
	}
}

func TestServiceSubReconciler_MetricsPort_Custom(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.MetricsPort = 9091
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	found := false
	for _, p := range svc.Spec.Ports {
		if p.Name == "metrics" && p.Port == 9091 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected custom metrics service port 9091, ports: %v", svc.Spec.Ports)
	}
}

// --- GetServiceType ---

func TestGetServiceType(t *testing.T) {
	tests := []struct {
		name     string
		svcType  operatorv1alpha1.ProxyServiceType
		expected operatorv1alpha1.ProxyServiceType
	}{
		{
			name:     "empty falls back to LoadBalancer",
			svcType:  "",
			expected: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
		},
		{
			name:     "explicit ClusterIP",
			svcType:  operatorv1alpha1.ProxyServiceTypeClusterIP,
			expected: operatorv1alpha1.ProxyServiceTypeClusterIP,
		},
		{
			name:     "explicit LoadBalancer",
			svcType:  operatorv1alpha1.ProxyServiceTypeLoadBalancer,
			expected: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envoy := &operatorv1alpha1.EnvoyProxy{
				Spec: operatorv1alpha1.EnvoyProxySpec{ServiceType: tt.svcType},
			}
			got := GetServiceType(envoy)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// --- Update path / needsUpdate tests ---

func TestConfigMapSubReconciler_UpdateOnChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewConfigMapSubReconciler(c, scheme)

	// Create with default config
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change to custom files
	envoy.Spec.Files = map[string]string{"envoy.yaml": "# custom config"}
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}
	if cm.Data["envoy.yaml"] != "# custom config" {
		t.Errorf("expected config map to be updated, got: %s", cm.Data["envoy.yaml"])
	}
}

func TestDeploymentSubReconciler_UpdateOnImageChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewDeploymentSubReconciler(c, scheme)

	// Create
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change the image
	envoy.Spec.Image = "envoyproxy/envoy:v1.30.0"
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if deploy.Spec.Template.Spec.Containers[0].Image != "envoyproxy/envoy:v1.30.0" {
		t.Errorf("expected updated image, got %s", deploy.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeploymentSubReconciler_UpdateOnReplicaChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewDeploymentSubReconciler(c, scheme)

	// Create with 2 replicas
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change replicas
	replicas := int32(5)
	envoy.Spec.Replicas = &replicas
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 5 {
		t.Errorf("expected 5 replicas after update, got %v", deploy.Spec.Replicas)
	}
}

func TestServiceSubReconciler_UpdateOnTypeChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create as LoadBalancer
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Switch to ClusterIP
	envoy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeClusterIP
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected updated service type ClusterIP, got %s", svc.Spec.Type)
	}
}

func TestServiceSubReconciler_UpdateOnAnnotationChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create without annotations
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Add annotation
	envoy.Spec.ServiceAnnotations = map[string]string{"cloud.google.com/neg": `{"ingress":true}`}
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Annotations["cloud.google.com/neg"] != `{"ingress":true}` {
		t.Errorf("expected annotation to be updated, got %v", svc.Annotations)
	}
}

func TestServiceSubReconciler_UpdateOnExternalTrafficPolicyChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	c := newTestClient(scheme, envoy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create with default Local
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Switch to Cluster
	envoy.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	if _, err := reconciler.Reconcile(ctx, envoy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeCluster {
		t.Errorf("expected ExternalTrafficPolicy Cluster after update, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

// --- Custom ServiceSpec override ---

func TestServiceSubReconciler_CustomServiceSpec_NilSelector(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	envoy := newTestEnvoy()
	envoy.Spec.ServiceSpec = &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 30081, Protocol: corev1.ProtocolTCP},
		},
		// nil Selector — should be filled in by the builder
	}
	c := newTestClient(scheme, envoy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("expected NodePort from custom spec, got %s", svc.Spec.Type)
	}
	if svc.Spec.Ports[0].Port != 30081 {
		t.Errorf("expected port 30081 from custom spec, got %d", svc.Spec.Ports[0].Port)
	}
	// Selector must be injected even though custom spec had nil selector
	if len(svc.Spec.Selector) == 0 {
		t.Error("expected selector to be set even with nil selector in custom ServiceSpec")
	}
}
