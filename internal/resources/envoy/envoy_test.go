// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

func int32Ptr(i int32) *int32 {
	return &i
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
	if len(container.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(container.Ports))
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
	c := newTestClient(scheme, envoy)

	reconciler := NewServiceSubReconciler(c, scheme)

	// First reconcile should create the service
	result, err := reconciler.Reconcile(ctx, envoy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify service was created
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected service type ClusterIP, got %s", svc.Spec.Type)
	}

	// Verify ports
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(svc.Spec.Ports))
	}

	if svc.Spec.Ports[0].Port != envoy.Spec.ProxyPort {
		t.Errorf("expected port %d, got %d", envoy.Spec.ProxyPort, svc.Spec.Ports[0].Port)
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
