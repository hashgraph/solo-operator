// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestCommonLabels(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-namespace",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:  "test-orbit",
			NodeId: 0,
		},
	}

	labels := CommonLabels(capsule)

	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, labels[LabelManagedBy])
	}
	if labels[LabelPartOf] != PartOfConsensusNode {
		t.Errorf("expected part-of label %s, got %s", PartOfConsensusNode, labels[LabelPartOf])
	}
	if labels[LabelOrbit] != "test-orbit" {
		t.Errorf("expected orbit label test-orbit, got %s", labels[LabelOrbit])
	}
	if labels[LabelNodeId] != "0" {
		t.Errorf("expected node-id label 0, got %s", labels[LabelNodeId])
	}
}

func TestPodLabels(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-namespace",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:  "test-orbit",
			NodeId: 0,
		},
	}

	labels := PodLabels(capsule)

	if labels[LabelComponent] != ComponentDeployment {
		t.Errorf("expected component label %s, got %s", ComponentDeployment, labels[LabelComponent])
	}
}

func TestResourceNames(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-namespace",
		},
	}

	if got := ServiceAccountName(capsule); got != "node0-sa" {
		t.Errorf("expected service account name node0-sa, got %s", got)
	}

	if got := RoleBindingName(capsule); got != "node0-rb" {
		t.Errorf("expected role binding name node0-rb, got %s", got)
	}

	if got := ConfigMapName(capsule); got != "node0-cm" {
		t.Errorf("expected config map name node0-cm, got %s", got)
	}

	if got := PVCName(capsule, VolumeTypeLogs); got != "node0-logs" {
		t.Errorf("expected PVC name node0-logs, got %s", got)
	}
}

func TestInternalHostname(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-namespace",
		},
	}

	expected := "node0.consensus.test-namespace.svc.cluster.local"
	if got := InternalHostname(capsule); got != expected {
		t.Errorf("expected internal hostname %s, got %s", expected, got)
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

func newTestCapsule() *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node0",
			Namespace: "test-namespace",
			UID:       "test-uid",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     "test-orbit",
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
			Log4j2XML: "<Configuration></Configuration>",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Name: "consensus-node",
						SoftwareVersion: operatorv1alpha1.SoftwareVersion{
							Repository: "hashgraph",
							ImageName:  "hedera-node",
							ImageTag:   "v0.50.0",
						},
					},
				},
			},
		},
	}
}

func TestServiceAccountSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewServiceAccountSubReconciler(c, scheme)

	// Should reconcile (always returns true)
	if reconciler.ShouldReconcile(ctx, capsule) != true {
		t.Error("expected ShouldReconcile to return true")
	}

	// First reconcile should create the service account
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify service account was created
	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-sa", Namespace: "test-namespace"}, sa); err != nil {
		t.Fatalf("failed to get service account: %v", err)
	}

	if sa.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, sa.Labels[LabelManagedBy])
	}
}

func TestConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.NodeProperties = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	c := newTestClient(scheme, capsule)

	reconciler := NewConfigMapSubReconciler(c, scheme)

	// First reconcile should create the config map
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify config map was created
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-cm", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get config map: %v", err)
	}

	if cm.Data["log4j2.xml"] != "<Configuration></Configuration>" {
		t.Errorf("expected log4j2.xml content, got %s", cm.Data["log4j2.xml"])
	}

	if cm.Data["nodeProperties"] == "" {
		t.Error("expected nodeProperties to be set")
	}
}

func TestServiceSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.ServiceType = operatorv1alpha1.ServiceTypeClusterIP
	c := newTestClient(scheme, capsule)

	reconciler := NewServiceSubReconciler(c, scheme)

	// First reconcile should create the service
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify service was created
	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected service type ClusterIP, got %s", svc.Spec.Type)
	}

	// Verify ports
	expectedPorts := map[string]int32{
		"gossip":     GossipPort,
		"grpc":       GrpcPort,
		"grpcs":      GrpcsPort,
		"prometheus": PrometheusPort,
	}

	for _, port := range svc.Spec.Ports {
		if expected, ok := expectedPorts[port.Name]; ok {
			if port.Port != expected {
				t.Errorf("expected port %s to be %d, got %d", port.Name, expected, port.Port)
			}
		}
	}
}

func TestPVCSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()

	// Configure PVCs
	storageClass := "standard"
	capsule.Spec.PersistentVolumeClaims = &operatorv1alpha1.ConsensusPVCConfig{
		Logs: &corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Stats: &corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}

	c := newTestClient(scheme, capsule)
	reconciler := NewPVCSubReconciler(c, scheme)

	// Should reconcile because PVCs are configured
	if !reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile to return true when PVCs are configured")
	}

	// First reconcile should create the PVCs
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify logs PVC was created
	logsPVC := &corev1.PersistentVolumeClaim{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-logs", Namespace: "test-namespace"}, logsPVC); err != nil {
		t.Fatalf("failed to get logs PVC: %v", err)
	}

	// Verify stats PVC was created
	statsPVC := &corev1.PersistentVolumeClaim{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-stats", Namespace: "test-namespace"}, statsPVC); err != nil {
		t.Fatalf("failed to get stats PVC: %v", err)
	}
}

func TestPVCSubReconciler_NoPVCs(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	// No PVCs configured

	c := newTestClient(scheme, capsule)
	reconciler := NewPVCSubReconciler(c, scheme)

	// Should not reconcile when PVCs are not configured
	if reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile to return false when PVCs are not configured")
	}
}

func TestRoleBindingSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewRoleBindingSubReconciler(c, scheme)

	// First reconcile should create the role binding
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}
}

func TestDeploymentSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewDeploymentSubReconciler(c, scheme)

	// First reconcile should create the deployment
	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}
}

func TestBuildImage(t *testing.T) {
	tests := []struct {
		name     string
		sv       operatorv1alpha1.SoftwareVersion
		expected string
	}{
		{
			name: "with tag",
			sv: operatorv1alpha1.SoftwareVersion{
				Repository: "hashgraph",
				ImageName:  "hedera-node",
				ImageTag:   "v0.50.0",
			},
			expected: "hashgraph/hedera-node:v0.50.0",
		},
		{
			name: "with manifest hash",
			sv: operatorv1alpha1.SoftwareVersion{
				Repository:   "hashgraph",
				ImageName:    "hedera-node",
				ImageTag:     "v0.50.0",
				ManifestHash: "sha256:abc123",
			},
			expected: "hashgraph/hedera-node@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildImage(tt.sv)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
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

func TestGetPVCNames(t *testing.T) {
	capsule := newTestCapsule()

	// No PVCs configured
	names := GetPVCNames(capsule)
	if len(names) != 0 {
		t.Errorf("expected 0 PVC names, got %d", len(names))
	}

	// Configure some PVCs
	storageClass := "standard"
	capsule.Spec.PersistentVolumeClaims = &operatorv1alpha1.ConsensusPVCConfig{
		Logs: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
		Stats: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
		Streams: &operatorv1alpha1.ConsensusPVCStreams{
			Record: &corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClass,
			},
		},
	}

	names = GetPVCNames(capsule)
	if len(names) != 3 {
		t.Errorf("expected 3 PVC names, got %d", len(names))
	}
}
