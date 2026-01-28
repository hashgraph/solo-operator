// SPDX-License-Identifier: Apache-2.0

package haproxy

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
			Image:       "haproxy:2.8.10-alpine3.20",
			ServiceType: operatorv1alpha1.HAProxyServiceTypeLoadBalancer,
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
		"grpc":  GrpcPort,
		"grpcs": GrpcsPort,
		"stats": StatsPort,
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
	haproxy.Spec.SecretName = "tls-secret"
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
		"grpc":  GrpcPort,
		"grpcs": GrpcsPort,
		"stats": StatsPort,
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

func TestNewSubReconcilers(t *testing.T) {
	scheme := newTestScheme()
	c := newTestClient(scheme)

	chain := NewSubReconcilers(c, scheme)

	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
}
