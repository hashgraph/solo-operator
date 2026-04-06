// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
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
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-haproxy",
			Namespace: "test-namespace",
		},
	}

	labels := CommonLabels(haproxy)

	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, labels[LabelManagedBy])
	}
	if labels[LabelPartOf] != PartOfHAProxy {
		t.Errorf("expected part-of label %s, got %s", PartOfHAProxy, labels[LabelPartOf])
	}
	if labels[LabelApp] != "test-haproxy" {
		t.Errorf("expected app label test-haproxy, got %s", labels[LabelApp])
	}
}

func TestConfigMapName(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-haproxy",
		},
	}

	expected := "test-haproxy-config"
	if got := ConfigMapName(haproxy); got != expected {
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
			image:    "custom/haproxy:latest",
			expected: "custom/haproxy:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image: tt.image,
				},
			}

			got := GetImage(haproxy)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
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

func newTestHAProxy() *operatorv1alpha1.HAProxyCapsule {
	return &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-haproxy",
			Namespace: "test-namespace",
			UID:       "test-uid",
		},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			Image:                         "haproxy:2.8.10-alpine3.20",
			ServiceType:                   operatorv1alpha1.HAProxyServiceTypeLoadBalancer,
			TerminationGracePeriodSeconds: 10,
		},
	}
}

func TestConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	reconciler := NewConfigMapSubReconciler(c, scheme)

	// First reconcile should create the config map
	result, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify config map was created
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	if cm.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, cm.Labels[LabelManagedBy])
	}

	if cm.Data[ConfigFileName] == "" {
		t.Error("expected haproxy.cfg to be set")
	}
}

func TestDeploymentSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	reconciler := NewDeploymentSubReconciler(c, scheme)

	// First reconcile should create the deployment
	result, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify deployment was created
	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	if deploy.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, deploy.Labels[LabelManagedBy])
	}

	// Verify container
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	if container.Name != ContainerName {
		t.Errorf("expected container name %s, got %s", ContainerName, container.Name)
	}
	if container.Image != haproxy.Spec.Image {
		t.Errorf("expected image %s, got %s", haproxy.Spec.Image, container.Image)
	}

	// Verify ports
	expectedPorts := map[string]int32{
		"grpc":    DefaultGrpcPort,
		"grpcs":   DefaultGrpcsPort,
		"metrics": DefaultMetricsPort,
	}
	for _, port := range container.Ports {
		if expected, ok := expectedPorts[port.Name]; ok {
			if port.ContainerPort != expected {
				t.Errorf("expected port %s to be %d, got %d", port.Name, expected, port.ContainerPort)
			}
		}
	}
}

func TestDeploymentSubReconciler_WithTLS(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.TLSSecretName = "tls-secret"
	c := newTestClient(scheme, haproxy)

	reconciler := NewDeploymentSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify deployment was created with TLS volume
	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	// Check for TLS volume
	foundTLSVolume := false
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == TLSVolumeName {
			foundTLSVolume = true
			if vol.Secret == nil || vol.Secret.SecretName != "tls-secret" {
				t.Errorf("expected TLS volume to reference secret tls-secret")
			}
			break
		}
	}
	if !foundTLSVolume {
		t.Error("expected TLS volume to be present")
	}

	// Check for TLS volume mount
	container := deploy.Spec.Template.Spec.Containers[0]
	foundTLSMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == TLSVolumeName {
			foundTLSMount = true
			if mount.MountPath != TLSMountPath {
				t.Errorf("expected TLS mount path %s, got %s", TLSMountPath, mount.MountPath)
			}
			if !mount.ReadOnly {
				t.Error("expected TLS mount to be read-only")
			}
			break
		}
	}
	if !foundTLSMount {
		t.Error("expected TLS volume mount to be present")
	}
}

func TestServiceSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	reconciler := NewServiceSubReconciler(c, scheme)

	// First reconcile should create the service
	result, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify service was created
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("expected service type LoadBalancer, got %s", svc.Spec.Type)
	}

	// Verify ports
	expectedPorts := map[string]int32{
		"grpc":    DefaultGrpcPort,
		"grpcs":   DefaultGrpcsPort,
		"metrics": DefaultMetricsPort,
	}
	for _, port := range svc.Spec.Ports {
		if expected, ok := expectedPorts[port.Name]; ok {
			if port.Port != expected {
				t.Errorf("expected port %s to be %d, got %d", port.Name, expected, port.Port)
			}
		}
	}
}

func TestServiceSubReconciler_ClusterIP(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.HAProxyServiceTypeClusterIP
	c := newTestClient(scheme, haproxy)

	reconciler := NewServiceSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify service was created with ClusterIP type
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected service type ClusterIP, got %s", svc.Spec.Type)
	}
}

func TestConfigMapSubReconciler_TLSConfig(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.TLSSecretName = "tls-secret"
	c := newTestClient(scheme, haproxy)

	reconciler := NewConfigMapSubReconciler(c, scheme)

	_, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	cfg := cm.Data[ConfigFileName]
	if !strings.Contains(cfg, "ssl crt /etc/haproxy/certs/tls.crt") {
		t.Error("expected TLS config to contain ssl crt directive")
	}
	if !strings.Contains(cfg, "alpn h2") {
		t.Error("expected TLS config to contain alpn h2 directive")
	}
}

func TestServiceSubReconciler_IngressType(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	c := newTestClient(scheme, haproxy)

	reconciler := NewServiceSubReconciler(c, scheme)

	_, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ingress service type should create a ClusterIP service
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ClusterIP service for Ingress type, got %s", svc.Spec.Type)
	}
}

func TestIngressSubReconciler_Skipped(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err == nil {
		t.Error("expected no ingress to be created for LoadBalancer type")
	}
}

func TestIngressSubReconciler_CreatesIngress(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	haproxy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: "haproxy.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test-haproxy",
										Port: networkingv1.ServiceBackendPort{Number: DefaultGrpcPort},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	c := newTestClient(scheme, haproxy)

	reconciler := NewIngressSubReconciler(c, scheme)

	_, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if len(ing.Spec.Rules) != 1 || ing.Spec.Rules[0].Host != "haproxy.example.com" {
		t.Errorf("unexpected ingress spec: %+v", ing.Spec)
	}
}

func TestIngressSubReconciler_SkippedForClusterIP(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeClusterIP
	c := newTestClient(scheme, haproxy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err == nil {
		t.Error("expected no ingress to be created for ClusterIP type")
	}
}

func TestIngressSubReconciler_NilIngressSpec(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	haproxy.Spec.IngressSpec = nil
	c := newTestClient(scheme, haproxy)

	reconciler := NewIngressSubReconciler(c, scheme)
	_, err := reconciler.Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error with nil ingressSpec: %v", err)
	}
	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err == nil {
		t.Error("expected no ingress to be created when ingressSpec is nil")
	}
}

func TestIngressSubReconciler_IngressClassName(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	className := "nginx"
	pathType := networkingv1.PathTypePrefix
	haproxy.Spec.IngressSpec = &networkingv1.IngressSpec{
		IngressClassName: &className,
		Rules: []networkingv1.IngressRule{
			{
				Host: "haproxy.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "test-haproxy",
									Port: networkingv1.ServiceBackendPort{Number: DefaultGrpcPort},
								},
							}},
						},
					},
				},
			},
		},
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("expected ingressClassName nginx, got %v", ing.Spec.IngressClassName)
	}
}

func TestIngressSubReconciler_Labels(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	haproxy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{Host: "haproxy.example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-haproxy",
								Port: networkingv1.ServiceBackendPort{Number: DefaultGrpcPort},
							},
						}},
					},
				},
			}},
		},
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewIngressSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label, got %v", ing.Labels)
	}
	if ing.Labels[LabelApp] != haproxy.Name {
		t.Errorf("expected app label %s, got %s", haproxy.Name, ing.Labels[LabelApp])
	}
}

func TestIngressSubReconciler_Update(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	haproxy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{Host: "haproxy-v1.example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-haproxy",
								Port: networkingv1.ServiceBackendPort{Number: DefaultGrpcPort},
							},
						}},
					},
				},
			}},
		},
	}
	c := newTestClient(scheme, haproxy)
	reconciler := NewIngressSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	// Update the host
	haproxy.Spec.IngressSpec.Rules[0].Host = "haproxy-v2.example.com"
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err != nil {
		t.Fatalf("failed to get ingress: %v", err)
	}
	if ing.Spec.Rules[0].Host != "haproxy-v2.example.com" {
		t.Errorf("expected updated host haproxy-v2.example.com, got %s", ing.Spec.Rules[0].Host)
	}
}

func TestIngressSubReconciler_DeletesOnServiceTypeChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeIngress
	pathType := networkingv1.PathTypePrefix
	haproxy.Spec.IngressSpec = &networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{Host: "haproxy.example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-haproxy",
								Port: networkingv1.ServiceBackendPort{Number: DefaultGrpcPort},
							},
						}},
					},
				},
			}},
		},
	}
	c := newTestClient(scheme, haproxy)
	reconciler := NewIngressSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("create reconcile failed: %v", err)
	}

	// Verify ingress was created
	ing := &networkingv1.Ingress{}
	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err != nil {
		t.Fatalf("expected ingress to exist after first reconcile: %v", err)
	}

	// Switch away from Ingress
	haproxy.Spec.ServiceType = operatorv1alpha1.ProxyServiceTypeLoadBalancer
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("delete reconcile failed: %v", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, ing); err == nil {
		t.Error("expected ingress to be deleted after switching to LoadBalancer type")
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

// --- Health check tests ---

func TestDeploymentSubReconciler_HealthProbes_Default(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]

	if container.StartupProbe == nil {
		t.Fatal("expected startup probe to be set by default")
	}
	if container.StartupProbe.HTTPGet == nil {
		t.Fatal("expected startup probe to use HTTPGet, not exec")
	}
	if container.StartupProbe.HTTPGet.Path != "/stats" {
		t.Errorf("expected startup probe path /stats, got %s", container.StartupProbe.HTTPGet.Path)
	}
	if container.StartupProbe.HTTPGet.Port.IntVal != DefaultMetricsPort {
		t.Errorf("expected startup probe port %d, got %d", DefaultMetricsPort, container.StartupProbe.HTTPGet.Port.IntVal)
	}
	defaults := DefaultHealthCheck()
	if container.StartupProbe.InitialDelaySeconds != defaults.StartupInitialDelaySeconds {
		t.Errorf("expected startup probe initialDelaySeconds %d, got %d", defaults.StartupInitialDelaySeconds, container.StartupProbe.InitialDelaySeconds)
	}
	if container.StartupProbe.PeriodSeconds != defaults.StartupPeriodSeconds {
		t.Errorf("expected startup probe periodSeconds %d, got %d", defaults.StartupPeriodSeconds, container.StartupProbe.PeriodSeconds)
	}
	if container.StartupProbe.FailureThreshold != defaults.StartupFailureThreshold {
		t.Errorf("expected startup probe failureThreshold %d, got %d", defaults.StartupFailureThreshold, container.StartupProbe.FailureThreshold)
	}

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe to be set by default")
	}
	if container.LivenessProbe.HTTPGet == nil || container.LivenessProbe.HTTPGet.Path != "/stats" {
		t.Error("expected liveness probe GET /stats")
	}
	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe to be set by default")
	}
	if container.ReadinessProbe.HTTPGet == nil || container.ReadinessProbe.HTTPGet.Path != "/stats" {
		t.Error("expected readiness probe GET /stats")
	}
}

func TestDeploymentSubReconciler_HealthProbes_Disabled(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.HealthCheck = &operatorv1alpha1.HAProxyHealthCheckSpec{Enabled: false}
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	if container.StartupProbe != nil {
		t.Error("expected no startup probe when healthCheck.enabled=false")
	}
	if container.LivenessProbe != nil {
		t.Error("expected no liveness probe when healthCheck.enabled=false")
	}
	if container.ReadinessProbe != nil {
		t.Error("expected no readiness probe when healthCheck.enabled=false")
	}
}

func TestDeploymentSubReconciler_HealthProbes_CustomValues(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.HealthCheck = &operatorv1alpha1.HAProxyHealthCheckSpec{
		Enabled:                    true,
		StartupInitialDelaySeconds: 30,
		StartupPeriodSeconds:       10,
		StartupFailureThreshold:    60,
		PeriodSeconds:              15,
		FailureThreshold:           5,
		TimeoutSeconds:             2,
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	if container.StartupProbe.InitialDelaySeconds != 30 {
		t.Errorf("expected startupInitialDelaySeconds 30, got %d", container.StartupProbe.InitialDelaySeconds)
	}
	if container.StartupProbe.PeriodSeconds != 10 {
		t.Errorf("expected startupPeriodSeconds 10, got %d", container.StartupProbe.PeriodSeconds)
	}
	if container.StartupProbe.FailureThreshold != 60 {
		t.Errorf("expected startupFailureThreshold 60, got %d", container.StartupProbe.FailureThreshold)
	}
	if container.LivenessProbe.PeriodSeconds != 15 {
		t.Errorf("expected liveness periodSeconds 15, got %d", container.LivenessProbe.PeriodSeconds)
	}
	if container.LivenessProbe.FailureThreshold != 5 {
		t.Errorf("expected liveness failureThreshold 5, got %d", container.LivenessProbe.FailureThreshold)
	}
}

func TestGetHealthCheck_Defaults(t *testing.T) {
	haproxy := newTestHAProxy() // no HealthCheck set

	hc := GetHealthCheck(haproxy)

	if !hc.Enabled {
		t.Error("expected health check enabled by default")
	}
	if hc.StartupInitialDelaySeconds != 5 {
		t.Errorf("expected default startupInitialDelaySeconds 5, got %d", hc.StartupInitialDelaySeconds)
	}
	if hc.StartupPeriodSeconds != 5 {
		t.Errorf("expected default startupPeriodSeconds 5, got %d", hc.StartupPeriodSeconds)
	}
	if hc.StartupFailureThreshold != 12 {
		t.Errorf("expected default startupFailureThreshold 12, got %d", hc.StartupFailureThreshold)
	}
	if hc.PeriodSeconds != 10 {
		t.Errorf("expected default periodSeconds 10, got %d", hc.PeriodSeconds)
	}
	if hc.FailureThreshold != 3 {
		t.Errorf("expected default failureThreshold 3, got %d", hc.FailureThreshold)
	}
	if hc.TimeoutSeconds != 1 {
		t.Errorf("expected default timeoutSeconds 1, got %d", hc.TimeoutSeconds)
	}
}

// --- Node scheduling tests ---

func TestDeploymentSubReconciler_NodeScheduling(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.NodeSelector = map[string]string{"sphere.hashgraph.io/role": "auxiliary"}
	haproxy.Spec.Tolerations = []corev1.Toleration{
		{Key: "sphere.hashgraph.io/role", Operator: corev1.TolerationOpEqual, Value: "auxiliary", Effect: corev1.TaintEffectNoSchedule},
	}
	haproxy.Spec.Affinity = &corev1.Affinity{
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
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
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
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
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
	haproxy := newTestHAProxy()
	haproxy.Spec.TerminationGracePeriodSeconds = 30
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
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
	haproxy := newTestHAProxy()
	haproxy.Spec.AdditionalLabels = map[string]string{
		"solo.hedera.com/node-id":   "0",
		"solo.hedera.com/node-name": "node1",
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	// Extra labels appear on the deployment and pod template
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
	haproxy := newTestHAProxy()
	haproxy.Spec.AdditionalLabels = map[string]string{"solo.hedera.com/prometheus-endpoint": "active"}
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
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
	haproxy := newTestHAProxy() // LoadBalancer, no explicit ETP
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		t.Errorf("expected ExternalTrafficPolicy Local, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestServiceSubReconciler_ExternalTrafficPolicy_Cluster(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeCluster {
		t.Errorf("expected ExternalTrafficPolicy Cluster, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestServiceSubReconciler_ClusterIP_NoExternalTrafficPolicy(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceType = operatorv1alpha1.HAProxyServiceTypeClusterIP
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
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
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceAnnotations = map[string]string{
		"cloud.google.com/neg": `{"ingress":true}`,
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Annotations["cloud.google.com/neg"] != `{"ingress":true}` {
		t.Errorf("expected NEG annotation, got %v", svc.Annotations)
	}
}

// --- Stats port test ---

func TestConfigMapSubReconciler_StatsPort(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	_, err := NewConfigMapSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	cfg := cm.Data[ConfigFileName]
	if !strings.Contains(cfg, "bind *:9090") {
		t.Errorf("expected stats to bind on 9090, config:\n%s", cfg)
	}
}

// --- Update path / needsUpdate tests ---

func TestConfigMapSubReconciler_UpdateOnChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewConfigMapSubReconciler(c, scheme)

	// Create
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change the config by adding a TLS secret (switches to TLS config template)
	haproxy.Spec.TLSSecretName = "tls-secret"
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy-config", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}
	if !strings.Contains(cm.Data[ConfigFileName], "ssl crt") {
		t.Error("expected config map to be updated to TLS config")
	}
}

func TestDeploymentSubReconciler_UpdateOnImageChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewDeploymentSubReconciler(c, scheme)

	// Create
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change the image
	haproxy.Spec.Image = "haproxy:3.0-alpine"
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if deploy.Spec.Template.Spec.Containers[0].Image != "haproxy:3.0-alpine" {
		t.Errorf("expected updated image, got %s", deploy.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestServiceSubReconciler_UpdateOnTypeChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy() // starts as LoadBalancer
	c := newTestClient(scheme, haproxy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change type to ClusterIP
	haproxy.Spec.ServiceType = operatorv1alpha1.HAProxyServiceTypeClusterIP
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected updated service type ClusterIP, got %s", svc.Spec.Type)
	}
}

func TestServiceSubReconciler_UpdateOnAnnotationChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create without annotations
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Add annotation
	haproxy.Spec.ServiceAnnotations = map[string]string{"cloud.google.com/neg": `{"ingress":true}`}
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Annotations["cloud.google.com/neg"] != `{"ingress":true}` {
		t.Errorf("expected annotation to be updated, got %v", svc.Annotations)
	}
}

func TestServiceSubReconciler_UpdateOnExternalTrafficPolicyChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create with default Local policy
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Switch to Cluster
	haproxy.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeCluster {
		t.Errorf("expected ExternalTrafficPolicy Cluster after update, got %s", svc.Spec.ExternalTrafficPolicy)
	}
}

// --- Custom ServiceSpec override ---

func TestServiceSubReconciler_CustomServiceSpec(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceSpec = &corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{Name: "grpc", Port: 30211, Protocol: corev1.ProtocolTCP},
		},
		// nil Selector — should be filled in by the builder
	}
	c := newTestClient(scheme, haproxy)

	_, err := NewServiceSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("expected NodePort service type from custom spec, got %s", svc.Spec.Type)
	}
	if svc.Spec.Ports[0].Port != 30211 {
		t.Errorf("expected port 30211 from custom spec, got %d", svc.Spec.Ports[0].Port)
	}
	// Selector must be set even though custom spec had nil selector
	if len(svc.Spec.Selector) == 0 {
		t.Error("expected selector to be set even with custom ServiceSpec")
	}
}

// --- GetHealthCheck with partial spec (zero-value fallback) ---

func TestGetHealthCheck_PartialSpec_ZeroValueFallback(t *testing.T) {
	haproxy := newTestHAProxy()
	// Only enable is set; all numeric fields are zero → should fall back to defaults
	haproxy.Spec.HealthCheck = &operatorv1alpha1.HAProxyHealthCheckSpec{
		Enabled: true,
	}

	defaults := DefaultHealthCheck()
	hc := GetHealthCheck(haproxy)

	if hc.StartupInitialDelaySeconds != defaults.StartupInitialDelaySeconds {
		t.Errorf("expected zero StartupInitialDelaySeconds to fall back to %d, got %d", defaults.StartupInitialDelaySeconds, hc.StartupInitialDelaySeconds)
	}
	if hc.StartupPeriodSeconds != defaults.StartupPeriodSeconds {
		t.Errorf("expected zero StartupPeriodSeconds to fall back to %d, got %d", defaults.StartupPeriodSeconds, hc.StartupPeriodSeconds)
	}
	if hc.StartupFailureThreshold != defaults.StartupFailureThreshold {
		t.Errorf("expected zero StartupFailureThreshold to fall back to %d, got %d", defaults.StartupFailureThreshold, hc.StartupFailureThreshold)
	}
	if hc.PeriodSeconds != defaults.PeriodSeconds {
		t.Errorf("expected zero PeriodSeconds to fall back to %d, got %d", defaults.PeriodSeconds, hc.PeriodSeconds)
	}
	if hc.FailureThreshold != defaults.FailureThreshold {
		t.Errorf("expected zero FailureThreshold to fall back to %d, got %d", defaults.FailureThreshold, hc.FailureThreshold)
	}
	if hc.TimeoutSeconds != defaults.TimeoutSeconds {
		t.Errorf("expected zero TimeoutSeconds to fall back to %d, got %d", defaults.TimeoutSeconds, hc.TimeoutSeconds)
	}
}

// mustHAProxyConfig calls HAProxyConfig and fails the test on error.
func mustHAProxyConfig(t *testing.T, haproxy *operatorv1alpha1.HAProxyCapsule) string {
	t.Helper()
	cfg, err := HAProxyConfig(haproxy)
	if err != nil {
		t.Fatalf("HAProxyConfig returned unexpected error: %v", err)
	}
	return cfg
}

// --- HAProxyConfig template tests ---

func TestHAProxyConfig_NonTLS(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
	}
	cfg := mustHAProxyConfig(t, haproxy)

	if !strings.Contains(cfg, "bind *:50211") {
		t.Errorf("expected grpc frontend on 50211, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "bind *:50212") {
		t.Errorf("expected grpcs frontend on 50212, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "server upstream node1:50211") {
		t.Errorf("expected upstream node1:50211 (haproxy- prefix stripped), config:\n%s", cfg)
	}
	if strings.Contains(cfg, "ssl") {
		t.Error("non-TLS config must not contain ssl directive")
	}
	if strings.Contains(cfg, "consensus_nodes_tls") {
		t.Error("non-TLS config must not contain consensus_nodes_tls backend")
	}
}

func TestHAProxyConfig_TLS(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			TLSSecretName: "my-tls",
		},
	}
	cfg := mustHAProxyConfig(t, haproxy)

	if !strings.Contains(cfg, "ssl crt /etc/haproxy/certs/tls.crt alpn h2") {
		t.Errorf("expected TLS grpcs bind directive, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "consensus_nodes_tls") {
		t.Errorf("expected consensus_nodes_tls backend in TLS config, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "ssl verify none") {
		t.Errorf("expected ssl verify none on TLS backend, config:\n%s", cfg)
	}
}

func TestHAProxyConfig_CustomPorts(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			GrpcPort:    50311,
			GrpcsPort:   50312,
			MetricsPort: 9191,
		},
	}
	cfg := mustHAProxyConfig(t, haproxy)

	if !strings.Contains(cfg, "bind *:50311") {
		t.Errorf("expected custom grpc port 50311, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "bind *:50312") {
		t.Errorf("expected custom grpcs port 50312, config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "bind *:9191") {
		t.Errorf("expected custom metrics port 9191, config:\n%s", cfg)
	}
}

func TestHAProxyConfig_CustomUpstreamService(t *testing.T) {
	haproxy := &operatorv1alpha1.HAProxyCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node1"},
		Spec: operatorv1alpha1.HAProxyCapsuleSpec{
			UpstreamService: "my-custom-svc",
		},
	}
	cfg := mustHAProxyConfig(t, haproxy)

	if !strings.Contains(cfg, "my-custom-svc") {
		t.Errorf("expected custom upstream service in config, config:\n%s", cfg)
	}
	// Prefix stripping must not apply when UpstreamService is explicit
	if strings.Contains(cfg, "node1") {
		t.Error("explicit UpstreamService must override prefix stripping")
	}
}

// --- GetUpstreamService tests ---

func TestGetUpstreamService(t *testing.T) {
	tests := []struct {
		name     string
		crName   string
		upstream string
		want     string
	}{
		{"strips haproxy- prefix", "haproxy-node1", "", "node1"},
		{"strips haproxy- short", "haproxy-x", "", "x"},
		{"no prefix to strip", "node1", "", "node1"},
		{"name too short for prefix", "haproxy", "", "haproxy"},
		{"explicit upstream overrides", "haproxy-node1", "custom-svc", "custom-svc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: tt.crName},
				Spec:       operatorv1alpha1.HAProxyCapsuleSpec{UpstreamService: tt.upstream},
			}
			if got := GetUpstreamService(haproxy); got != tt.want {
				t.Errorf("GetUpstreamService() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Config checksum annotation tests ---

func TestDeploymentSubReconciler_ConfigChecksumAnnotation(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)

	_, err := NewDeploymentSubReconciler(c, scheme).Reconcile(ctx, haproxy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	checksum := deploy.Spec.Template.Annotations[configChecksumAnnotation]
	if checksum == "" {
		t.Errorf("expected config checksum annotation %q to be set on pod template", configChecksumAnnotation)
	}
}

func TestDeploymentSubReconciler_UpdateOnConfigChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewDeploymentSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	checksumBefore := deploy.Spec.Template.Annotations[configChecksumAnnotation]

	// Change config by adding upstream service (changes generated HAProxy config)
	haproxy.Spec.UpstreamService = "new-upstream"
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, deploy); err != nil {
		t.Fatalf("failed to get deployment after update: %v", err)
	}
	checksumAfter := deploy.Spec.Template.Annotations[configChecksumAnnotation]

	if checksumBefore == checksumAfter {
		t.Error("expected config checksum annotation to change when upstream service changes")
	}
}

// --- Service label update test ---

func TestServiceSubReconciler_UpdateOnLabelChange(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	c := newTestClient(scheme, haproxy)
	reconciler := NewServiceSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Add an additional label — needsUpdate must detect this
	haproxy.Spec.AdditionalLabels = map[string]string{"solo.hedera.com/env": "prod"}
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Labels["solo.hedera.com/env"] != "prod" {
		t.Errorf("expected service label to be updated, got %v", svc.Labels)
	}
}

// --- Managed annotation deletion tests ---

func TestServiceSubReconciler_AnnotationDeletion(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	haproxy := newTestHAProxy()
	haproxy.Spec.ServiceAnnotations = map[string]string{
		"cloud.google.com/neg": `{"ingress": true}`,
		"foo/bar":              "baz",
	}
	c := newTestClient(scheme, haproxy)
	reconciler := NewServiceSubReconciler(c, scheme)

	// Create with two annotations
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if svc.Annotations["foo/bar"] != "baz" {
		t.Fatalf("expected foo/bar annotation on create, got %v", svc.Annotations)
	}

	// Remove foo/bar from spec — reconciler must delete it from the Service
	haproxy.Spec.ServiceAnnotations = map[string]string{
		"cloud.google.com/neg": `{"ingress": true}`,
	}
	if _, err := reconciler.Reconcile(ctx, haproxy); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if _, found := svc.Annotations["foo/bar"]; found {
		t.Error("expected foo/bar annotation to be removed after deletion from spec")
	}
	if svc.Annotations["cloud.google.com/neg"] == "" {
		t.Error("expected cloud.google.com/neg annotation to be preserved")
	}
}
