// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
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
	capsule := newTestCapsule()
	labels := PodLabels(capsule)

	// operator labels
	if labels[LabelComponent] != ComponentDeployment {
		t.Errorf("expected component %s, got %s", ComponentDeployment, labels[LabelComponent])
	}
	if labels[LabelOrbit] != "test-orbit" {
		t.Errorf("expected orbit test-orbit, got %s", labels[LabelOrbit])
	}

	// solo.hedera.com labels
	checks := map[string]string{
		LabelNodeId:        "0",
		LabelSoloNodeName:  "node0",
		LabelSoloAccountId: "0.0.3",
		LabelSoloType:      "network-node",
	}
	for k, want := range checks {
		if got := labels[k]; got != want {
			t.Errorf("label %s: want %q, got %q", k, want, got)
		}
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

	if got := HapiAppConfigMapName(capsule); got != "node0-hapi-app-cm" {
		t.Errorf("expected hapi-app config map name node0-hapi-app-cm, got %s", got)
	}

	if got := DataConfigMapName(capsule); got != "node0-data-config-cm" {
		t.Errorf("expected data config map name node0-data-config-cm, got %s", got)
	}

	if got := BlockNodesConfigMapName(capsule); got != "node0-block-nodes-cm" {
		t.Errorf("expected block-nodes config map name node0-block-nodes-cm, got %s", got)
	}

	if got := StatefulSetName(capsule); got != "node0" {
		t.Errorf("expected statefulset name node0, got %s", got)
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

	expected := "node0-0.consensus.test-namespace.svc.cluster.local"
	if got := InternalHostname(capsule); got != expected {
		t.Errorf("expected internal hostname %s, got %s", expected, got)
	}
}

func TestInternalHostname_WithInternalAddress(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-namespace"},
		Spec:       operatorv1alpha1.ConsensusCapsuleSpec{InternalAddress: "10.0.0.5"},
	}
	if got := InternalHostname(capsule); got != "10.0.0.5" {
		t.Errorf("expected InternalAddress override 10.0.0.5, got %s", got)
	}
}

func TestRenderProperties(t *testing.T) {
	m := map[string]string{
		"zebra": "z",
		"apple": "a",
		"mango": "m",
	}
	got := RenderProperties(m)
	expected := "apple=a\nmango=m\nzebra=z\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestRenderProperties_Empty(t *testing.T) {
	got := RenderProperties(nil)
	if got != "" {
		t.Errorf("expected empty string for nil map, got %q", got)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

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

func newTestOrbit() *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings: map[string]string{
					"maxTransactionBytesPerSec": "2097152",
					"useTLS":                    "1",
				},
				ApplicationConfiguration: map[string]string{
					"blockStream.streamMode": "BOTH",
					"hedera.shard":           "0",
				},
				PermissionConfiguration: map[string]string{
					"getVersionInfo": "EVERYBODY",
				},
			},
		},
	}
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
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
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

// ── ServiceAccount ────────────────────────────────────────────────────────────

func TestServiceAccountSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewServiceAccountSubReconciler(c, scheme)

	if reconciler.ShouldReconcile(ctx, capsule) != true {
		t.Error("expected ShouldReconcile to return true")
	}

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-sa", Namespace: "test-namespace"}, sa); err != nil {
		t.Fatalf("failed to get service account: %v", err)
	}

	if sa.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("expected managed-by label %s, got %s", ManagedByValue, sa.Labels[LabelManagedBy])
	}
}

func TestServiceAccountSubReconciler_CustomName(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.ServiceAccount = &operatorv1alpha1.ServiceAccountConfig{
		Name: "my-custom-sa",
	}
	c := newTestClient(scheme, capsule)
	reconciler := NewServiceAccountSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, client.ObjectKey{Name: ServiceAccountName(capsule), Namespace: capsule.Namespace}, sa); err == nil {
		t.Error("expected operator-managed SA to not be created when custom name is set")
	}
}

func TestServiceAccountSubReconciler_LabelsAndAnnotations(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.ServiceAccount = &operatorv1alpha1.ServiceAccountConfig{
		Labels:      map[string]string{"custom-label": "val"},
		Annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::123:role/test"},
	}
	c := newTestClient(scheme, capsule)
	reconciler := NewServiceAccountSubReconciler(c, scheme)

	if _, err := reconciler.Reconcile(ctx, capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, client.ObjectKey{Name: ServiceAccountName(capsule), Namespace: capsule.Namespace}, sa); err != nil {
		t.Fatalf("failed to get SA: %v", err)
	}
	if sa.Labels["custom-label"] != "val" {
		t.Errorf("expected custom-label=val, got %q", sa.Labels["custom-label"])
	}
	if sa.Annotations["eks.amazonaws.com/role-arn"] != "arn:aws:iam::123:role/test" {
		t.Error("expected role-arn annotation to be set")
	}
}

func TestServiceAccountSubReconciler_UpdateAnnotations(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)
	reconciler := NewServiceAccountSubReconciler(c, scheme)

	// Create without annotations
	if _, err := reconciler.Reconcile(ctx, capsule); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Now add an annotation and reconcile again
	capsule.Spec.PodProperties.ServiceAccount = &operatorv1alpha1.ServiceAccountConfig{
		Annotations: map[string]string{"iam.gke.io/gcp-service-account": "sa@project.iam.gserviceaccount.com"},
	}
	if _, err := reconciler.Reconcile(ctx, capsule); err != nil {
		t.Fatalf("update: %v", err)
	}

	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, client.ObjectKey{Name: ServiceAccountName(capsule), Namespace: capsule.Namespace}, sa); err != nil {
		t.Fatalf("get: %v", err)
	}
	if sa.Annotations["iam.gke.io/gcp-service-account"] != "sa@project.iam.gserviceaccount.com" {
		t.Error("expected gcp-service-account annotation after update")
	}
}

// ── HapiAppConfigMap ──────────────────────────────────────────────────────────

func TestHapiAppConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	orbit := newTestOrbit()
	c := newTestClient(scheme, capsule, orbit)

	reconciler := NewHapiAppConfigMapSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-hapi-app-cm", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get hapi-app ConfigMap: %v", err)
	}

	if cm.Data["log4j2.xml"] != "<Configuration></Configuration>" {
		t.Errorf("unexpected log4j2.xml: %s", cm.Data["log4j2.xml"])
	}

	if cm.Data["settings.txt"] == "" {
		t.Error("expected settings.txt to be populated from Orbit")
	}
	if got := cm.Data["settings.txt"]; got != "maxTransactionBytesPerSec=2097152\nuseTLS=1\n" {
		t.Errorf("unexpected settings.txt content: %q", got)
	}
}

func TestHapiAppConfigMapSubReconciler_OrbitNotFound(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule() // orbit "test-orbit" not in client
	c := newTestClient(scheme, capsule)

	reconciler := NewHapiAppConfigMapSubReconciler(c, scheme)

	// Should succeed (orbit not found is handled gracefully)
	_, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error when orbit is missing: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-hapi-app-cm", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get hapi-app ConfigMap: %v", err)
	}

	// log4j2.xml should be present; settings.txt absent (no orbit)
	if cm.Data["log4j2.xml"] != "<Configuration></Configuration>" {
		t.Errorf("unexpected log4j2.xml: %s", cm.Data["log4j2.xml"])
	}
	if _, ok := cm.Data["settings.txt"]; ok {
		t.Error("expected settings.txt to be absent when orbit is not found")
	}
}

// ── DataConfigMap ─────────────────────────────────────────────────────────────

func TestDataConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	orbit := newTestOrbit()
	c := newTestClient(scheme, capsule, orbit)

	reconciler := NewDataConfigMapSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-data-config-cm", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get data-config ConfigMap: %v", err)
	}

	if cm.Data["application.properties"] == "" {
		t.Error("expected application.properties to be populated from Orbit")
	}
	if got := cm.Data["application.properties"]; got != "blockStream.streamMode=BOTH\nhedera.shard=0\n" {
		t.Errorf("unexpected application.properties: %q", got)
	}

	if cm.Data["api-permission.properties"] == "" {
		t.Error("expected api-permission.properties to be populated from Orbit")
	}
	if got := cm.Data["api-permission.properties"]; got != "getVersionInfo=EVERYBODY\n" {
		t.Errorf("unexpected api-permission.properties: %q", got)
	}
}

// ── BlockNodesConfigMap ───────────────────────────────────────────────────────

func TestBlockNodesConfigMapSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.BlockNodes = &operatorv1alpha1.BlockNodesConfig{
		Nodes: []operatorv1alpha1.BlockNodeEndpoint{
			{Address: "block-node-1:8080"},
			{Address: "block-node-2:8080"},
		},
		BlockItemBatchSize: 256,
	}
	c := newTestClient(scheme, capsule)

	reconciler := NewBlockNodesConfigMapSubReconciler(c, scheme)

	if !reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile true when BlockNodes is set")
	}

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0-block-nodes-cm", Namespace: "test-namespace"}, cm); err != nil {
		t.Fatalf("failed to get block-nodes ConfigMap: %v", err)
	}

	jsonStr, ok := cm.Data["block-nodes.json"]
	if !ok {
		t.Fatal("expected block-nodes.json key in ConfigMap")
	}

	var parsed operatorv1alpha1.BlockNodesConfig
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to parse block-nodes.json: %v", err)
	}

	if len(parsed.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(parsed.Nodes))
	}
	if parsed.Nodes[0].Address != "block-node-1:8080" {
		t.Errorf("unexpected first node address: %s", parsed.Nodes[0].Address)
	}
	if parsed.BlockItemBatchSize != 256 {
		t.Errorf("expected batch size 256, got %d", parsed.BlockItemBatchSize)
	}
}

func TestBlockNodesConfigMapSubReconciler_Disabled(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule() // BlockNodes is nil
	c := newTestClient(scheme, capsule)

	reconciler := NewBlockNodesConfigMapSubReconciler(c, scheme)

	if reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile false when BlockNodes is nil")
	}
}

// ── Service ───────────────────────────────────────────────────────────────────

func TestServiceSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	capsule.Spec.ServiceType = operatorv1alpha1.ServiceTypeClusterIP
	c := newTestClient(scheme, capsule)

	reconciler := NewServiceSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: "node0", Namespace: "test-namespace"}, svc); err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected service type ClusterIP, got %s", svc.Spec.Type)
	}

	// solo.hedera.com service labels
	svcLabelChecks := map[string]string{
		LabelSoloType:      "network-node-svc",
		LabelSoloPromEndpt: "active",
	}
	for k, want := range svcLabelChecks {
		if got := svc.Labels[k]; got != want {
			t.Errorf("service label %s: want %q, got %q", k, want, got)
		}
	}

	expectedPorts := map[string]int32{
		"gossip":  GossipPort,
		"grpc":    GrpcPort,
		"grpcs":   GrpcsPort,
		"metrics": PrometheusPort,
	}
	for _, port := range svc.Spec.Ports {
		if expected, ok := expectedPorts[port.Name]; ok {
			if port.Port != expected {
				t.Errorf("expected port %s to be %d, got %d", port.Name, expected, port.Port)
			}
		}
	}
}

// ── PVC ───────────────────────────────────────────────────────────────────────

func TestPVCSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()

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
		State: &corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}

	c := newTestClient(scheme, capsule)
	reconciler := NewPVCSubReconciler(c, scheme)

	if !reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile to return true when PVCs are configured")
	}

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	for _, name := range []string{"node0-logs", "node0-stats", "node0-state"} {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: "test-namespace"}, pvc); err != nil {
			t.Errorf("failed to get PVC %s: %v", name, err)
		}
	}
}

func TestPVCSubReconciler_NoPVCs(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewPVCSubReconciler(c, scheme)

	if reconciler.ShouldReconcile(ctx, capsule) {
		t.Error("expected ShouldReconcile to return false when PVCs are not configured")
	}
}

// ── RoleBinding ───────────────────────────────────────────────────────────────

func TestRoleBindingSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewRoleBindingSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}
}

// ── StatefulSet ───────────────────────────────────────────────────────────────

func TestStatefulSetSubReconciler(t *testing.T) {
	ctx := context.Background()
	scheme := newTestScheme()
	capsule := newTestCapsule()
	c := newTestClient(scheme, capsule)

	reconciler := NewStatefulSetSubReconciler(c, scheme)

	result, err := reconciler.Reconcile(ctx, capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}
}

func TestStatefulSetVolumes_DefaultEmptyDir(t *testing.T) {
	ctx := context.Background()
	capsule := newTestCapsule()
	// No PVCs configured — all data volumes should be emptyDir

	r := &StatefulSetSubReconciler{}
	volumes, mounts := r.buildVolumes(capsule)

	// Index volumes and mounts by name for easy lookup
	volByName := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volByName[v.Name] = v
	}
	mountByName := make(map[string]corev1.VolumeMount)
	for _, m := range mounts {
		mountByName[m.Name] = m
	}

	// ConfigMap volumes must be present
	for _, cmVol := range []string{"hapi-app-config", "data-config"} {
		v, ok := volByName[cmVol]
		if !ok {
			t.Errorf("expected volume %s", cmVol)
			continue
		}
		if v.ConfigMap == nil {
			t.Errorf("expected %s to be a ConfigMap volume", cmVol)
		}
	}

	// hapi-app-config must be subPath-mounted for log4j2.xml and settings.txt
	// at the HapiApp2.0 directory (where the entrypoint script looks for them)
	const hapiBase = "/opt/hgcapp/services-hedera/HapiApp2.0"
	hapiConfigMounts := map[string]bool{}
	for _, m := range mounts {
		if m.Name == "hapi-app-config" {
			hapiConfigMounts[m.SubPath] = m.ReadOnly
		}
	}
	for _, subPath := range []string{"log4j2.xml", "settings.txt"} {
		readOnly, ok := hapiConfigMounts[subPath]
		if !ok {
			t.Errorf("expected hapi-app-config subPath mount for %s", subPath)
			continue
		}
		if !readOnly {
			t.Errorf("expected hapi-app-config/%s to be mounted readonly", subPath)
		}
		// verify mount path includes hapiBase
		for _, m := range mounts {
			if m.Name == "hapi-app-config" && m.SubPath == subPath {
				if m.MountPath != hapiBase+"/"+subPath {
					t.Errorf("unexpected mount path for %s: %s", subPath, m.MountPath)
				}
			}
		}
	}

	// All PVC-optional volumes must default to emptyDir
	for _, name := range []string{VolumeTypeLogs, VolumeTypeStats, VolumeTypeSaved, VolumeTypeState, VolumeTypeUpgrade, VolumeTypeBlocks, VolumeTypeRecords, VolumeTypeEvents} {
		v, ok := volByName[name]
		if !ok {
			t.Errorf("expected volume %s to exist (emptyDir fallback)", name)
			continue
		}
		if v.EmptyDir == nil {
			t.Errorf("expected volume %s to be emptyDir when no PVC configured, got %+v", name, v.VolumeSource)
		}
	}

	// block-nodes-config must NOT be present (BlockNodes is nil)
	if _, ok := volByName["block-nodes-config"]; ok {
		t.Error("expected block-nodes-config volume to be absent when BlockNodes is nil")
	}

	// consensus-config volume must be gone
	if _, ok := volByName["consensus-config"]; ok {
		t.Error("consensus-config volume should be removed")
	}

	_ = ctx
	_ = mounts
}

func TestStatefulSetVolumes_WithPVC(t *testing.T) {
	capsule := newTestCapsule()
	storageClass := "fast"
	capsule.Spec.PersistentVolumeClaims = &operatorv1alpha1.ConsensusPVCConfig{
		Saved: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
		State: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
	}

	r := &StatefulSetSubReconciler{}
	volumes, _ := r.buildVolumes(capsule)

	volByName := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volByName[v.Name] = v
	}

	// states (data/saved) should be PVC-backed
	v, ok := volByName[VolumeTypeSaved]
	if !ok {
		t.Fatal("expected states volume")
	}
	if v.PersistentVolumeClaim == nil {
		t.Error("expected states to be PVC-backed")
	}
	if v.PersistentVolumeClaim.ClaimName != "node0-saved" {
		t.Errorf("unexpected PVC claim name: %s", v.PersistentVolumeClaim.ClaimName)
	}

	// state (HapiApp2.0/state) should be PVC-backed
	v, ok = volByName[VolumeTypeState]
	if !ok {
		t.Fatal("expected state volume")
	}
	if v.PersistentVolumeClaim == nil {
		t.Error("expected state to be PVC-backed")
	}

	// logs should still be emptyDir (not in PVC config)
	v, ok = volByName[VolumeTypeLogs]
	if !ok {
		t.Fatal("expected logs volume")
	}
	if v.EmptyDir == nil {
		t.Error("expected logs to fall back to emptyDir")
	}
}

func TestStatefulSetVolumes_BlockNodes(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.BlockNodes = &operatorv1alpha1.BlockNodesConfig{
		Nodes: []operatorv1alpha1.BlockNodeEndpoint{{Address: "bn:8080"}},
	}

	r := &StatefulSetSubReconciler{}
	volumes, mounts := r.buildVolumes(capsule)

	volByName := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volByName[v.Name] = v
	}
	mountByPath := make(map[string]corev1.VolumeMount)
	for _, m := range mounts {
		mountByPath[m.MountPath] = m
	}

	v, ok := volByName["block-nodes-config"]
	if !ok {
		t.Fatal("expected block-nodes-config volume when BlockNodes is set")
	}
	if v.ConfigMap == nil {
		t.Error("expected block-nodes-config to be a ConfigMap volume")
	}

	m, ok := mountByPath["/opt/hgcapp/services-hedera/HapiApp2.0/data/config/block-nodes.json"]
	if !ok {
		t.Error("expected block-nodes.json mount")
	} else if m.SubPath != "block-nodes.json" {
		t.Errorf("expected subPath block-nodes.json, got %s", m.SubPath)
	}
}

func TestStatefulSetVolumes_HapiAppSecrets(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.HapiAppSecretsName = "node0-tls-secret"

	r := &StatefulSetSubReconciler{}
	volumes, mounts := r.buildVolumes(capsule)

	volByName := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volByName[v.Name] = v
	}
	mountByPath := make(map[string]corev1.VolumeMount)
	for _, m := range mounts {
		mountByPath[m.MountPath] = m
	}

	v, ok := volByName["hapi-app-secrets"]
	if !ok {
		t.Fatal("expected hapi-app-secrets volume when HapiAppSecretsName is set")
	}
	if v.Secret == nil || v.Secret.SecretName != "node0-tls-secret" {
		t.Errorf("unexpected secret name: %+v", v.Secret)
	}

	const base = "/opt/hgcapp/services-hedera/HapiApp2.0"
	for _, path := range []string{base + "/hedera.crt", base + "/hedera.key"} {
		m, ok := mountByPath[path]
		if !ok {
			t.Errorf("expected mount at %s", path)
			continue
		}
		if m.SubPath == "" {
			t.Errorf("expected non-empty SubPath for %s", path)
		}
	}
}

func TestStatefulSetVolumes_DataConfigDir(t *testing.T) {
	r := &StatefulSetSubReconciler{}
	volumes, mounts := r.buildVolumes(newTestCapsule())

	volByName := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volByName[v.Name] = v
	}
	mountByPath := make(map[string]corev1.VolumeMount)
	for _, m := range mounts {
		mountByPath[m.MountPath] = m
	}

	// data-config-dir must be an emptyDir
	v, ok := volByName["data-config-dir"]
	if !ok {
		t.Fatal("expected data-config-dir volume")
	}
	if v.EmptyDir == nil {
		t.Error("expected data-config-dir to be emptyDir")
	}

	// data-config-dir must be mounted at data/config (writable, no subPath)
	m, ok := mountByPath[HapiBase+"/data/config"]
	if !ok {
		t.Fatal("expected mount at data/config")
	}
	if m.Name != "data-config-dir" {
		t.Errorf("expected data-config-dir at data/config, got %s", m.Name)
	}
	if m.SubPath != "" {
		t.Error("data-config-dir must not use subPath")
	}

	// genesis-config must be an optional ConfigMap volume (for the init container)
	gc, ok := volByName["genesis-config"]
	if !ok {
		t.Fatal("expected genesis-config volume")
	}
	if gc.ConfigMap == nil || gc.ConfigMap.Optional == nil || !*gc.ConfigMap.Optional {
		t.Error("expected genesis-config to be an optional ConfigMap volume")
	}

	// genesis-network.json must NOT be subPath-mounted in the main container
	for _, m := range mounts {
		if m.SubPath == GenesisNetworkFileName {
			t.Error("genesis-network.json must not be subPath-mounted in the main container")
		}
	}
}

func TestGenesisInitContainer(t *testing.T) {
	r := &StatefulSetSubReconciler{}
	c := r.buildGenesisInitContainer()

	if c.Name != "genesis-init" {
		t.Errorf("unexpected init container name: %s", c.Name)
	}
	if c.Image != GenesisInitImage {
		t.Errorf("unexpected image: %s", c.Image)
	}

	mountByName := make(map[string]corev1.VolumeMount)
	for _, m := range c.VolumeMounts {
		mountByName[m.Name] = m
	}

	if m, ok := mountByName["genesis-config"]; !ok {
		t.Error("expected genesis-config mount in init container")
	} else if m.MountPath != "/tmp/genesis-config" {
		t.Errorf("unexpected genesis-config mount path: %s", m.MountPath)
	}

	if m, ok := mountByName["data-config-dir"]; !ok {
		t.Error("expected data-config-dir mount in init container")
	} else if m.MountPath != HapiBase+"/data/config" {
		t.Errorf("unexpected data-config-dir mount path: %s", m.MountPath)
	}

	// saved volume must be mounted so the state directory check works
	if _, ok := mountByName[VolumeTypeSaved]; !ok {
		t.Error("expected saved volume mount in init container")
	}

	// state-dir check must appear before the copy in the command
	cmd := c.Command[len(c.Command)-1]
	stateDirIdx := strings.Index(cmd, SavedStateDir)
	cpIdx := strings.Index(cmd, "cp /tmp/genesis-config/")
	if stateDirIdx < 0 {
		t.Error("expected state directory reference in init container command")
	}
	if cpIdx < 0 {
		t.Error("expected cp command in init container")
	}
	if stateDirIdx > cpIdx {
		t.Error("state-dir check must appear before the cp command")
	}
}

func TestStatefulSetEnvVars(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.NodeAlias = "node1"
	capsule.Spec.PodProperties.Containers.ConsensusNode.JavaHeapMin = "256m"
	capsule.Spec.PodProperties.Containers.ConsensusNode.JavaHeapMax = "8g"

	r := &StatefulSetSubReconciler{}
	container, err := r.buildConsensusNodeContainer(capsule, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envByName := make(map[string]corev1.EnvVar)
	for _, e := range container.Env {
		envByName[e.Name] = e
	}

	if v, ok := envByName["CONSENSUS_NODE_ID"]; !ok || v.Value != "0" {
		t.Errorf("expected CONSENSUS_NODE_ID=0, got %+v", envByName["CONSENSUS_NODE_ID"])
	}
	if v, ok := envByName["CONSENSUS_NODE_ALIAS"]; !ok || v.Value != "node1" {
		t.Errorf("expected CONSENSUS_NODE_ALIAS=node1, got %+v", envByName["CONSENSUS_NODE_ALIAS"])
	}
	if v, ok := envByName["JAVA_HEAP_MIN"]; !ok || v.Value != "256m" {
		t.Errorf("expected JAVA_HEAP_MIN=256m, got %+v", envByName["JAVA_HEAP_MIN"])
	}
	if v, ok := envByName["JAVA_HEAP_MAX"]; !ok || v.Value != "8g" {
		t.Errorf("expected JAVA_HEAP_MAX=8g, got %+v", envByName["JAVA_HEAP_MAX"])
	}
	if _, ok := envByName["POD_IP"]; !ok {
		t.Error("expected POD_IP env var via fieldRef")
	}
	if _, ok := envByName["JAVA_OPTS"]; ok {
		t.Error("JAVA_OPTS with JDWP debug agent should not be present by default")
	}
}

// ── GetPVCNames ───────────────────────────────────────────────────────────────

func TestGetPVCNames(t *testing.T) {
	capsule := newTestCapsule()

	names := GetPVCNames(capsule)
	if len(names) != 0 {
		t.Errorf("expected 0 PVC names, got %d", len(names))
	}

	storageClass := "standard"
	capsule.Spec.PersistentVolumeClaims = &operatorv1alpha1.ConsensusPVCConfig{
		Logs:  &corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass},
		Stats: &corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass},
		State: &corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass},
		Streams: &operatorv1alpha1.ConsensusPVCStreams{
			Record: &corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass},
		},
	}

	names = GetPVCNames(capsule)
	if len(names) != 4 {
		t.Errorf("expected 4 PVC names, got %d: %v", len(names), names)
	}
}

// ── StatefulSet needsUpdate ───────────────────────────────────────────────────

func TestStatefulSetSubReconciler_NeedsUpdate(t *testing.T) {
	r := &StatefulSetSubReconciler{}

	base := func() *appsv1.StatefulSet {
		replicas := int32(1)
		return &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "consensus-node", Image: "hashgraph/hedera-node:v0.50.0"},
						},
						Volumes: []corev1.Volume{{Name: "logs"}},
					},
				},
			},
		}
	}
	t.Run("identical", func(t *testing.T) {
		if r.needsUpdate(base(), base()) {
			t.Error("expected no update for identical StatefulSets")
		}
	})
	t.Run("image differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Template.Spec.Containers[0].Image = "hashgraph/hedera-node:v0.51.0"
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when image differs")
		}
	})
	t.Run("replica count differs", func(t *testing.T) {
		desired := base()
		two := int32(2)
		desired.Spec.Replicas = &two
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when replicas differ")
		}
	})
	t.Run("nil replicas on existing", func(t *testing.T) {
		existing := base()
		existing.Spec.Replicas = nil
		if !r.needsUpdate(existing, base()) {
			t.Error("expected update when existing replicas is nil")
		}
	})
	t.Run("container count differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Template.Spec.Containers = append(desired.Spec.Template.Spec.Containers, corev1.Container{Name: "sidecar"})
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when container count differs")
		}
	})
	t.Run("volume count differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Template.Spec.Volumes = append(desired.Spec.Template.Spec.Volumes, corev1.Volume{Name: "extra"})
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when volume count differs")
		}
	})
}

// ── Service needsUpdate ───────────────────────────────────────────────────────

func TestServiceSubReconciler_NeedsUpdate(t *testing.T) {
	r := &ServiceSubReconciler{}

	base := func() *corev1.Service {
		return &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: map[string]string{"app": "consensus"},
				Ports: []corev1.ServicePort{
					{Name: "gossip", Port: GossipPort, Protocol: corev1.ProtocolTCP},
					{Name: "grpc", Port: GrpcPort, Protocol: corev1.ProtocolTCP},
				},
			},
		}
	}

	t.Run("identical", func(t *testing.T) {
		if r.needsUpdate(base(), base()) {
			t.Error("expected no update for identical services")
		}
	})
	t.Run("type differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Type = corev1.ServiceTypeLoadBalancer
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when type differs")
		}
	})
	t.Run("selector differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Selector["app"] = "other"
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when selector differs")
		}
	})
	t.Run("port count differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Ports = append(desired.Spec.Ports, corev1.ServicePort{Name: "extra", Port: 9999, Protocol: corev1.ProtocolTCP})
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when port count differs")
		}
	})
	t.Run("port name differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Ports[0].Name = "other"
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when port name differs")
		}
	})
	t.Run("port number differs", func(t *testing.T) {
		desired := base()
		desired.Spec.Ports[0].Port = 50112
		if !r.needsUpdate(base(), desired) {
			t.Error("expected update when port number differs")
		}
	})
}

// ── Chain ─────────────────────────────────────────────────────────────────────

func TestNewSubReconcilers(t *testing.T) {
	scheme := newTestScheme()
	c := newTestClient(scheme)

	chain := NewSubReconcilers(c, scheme)

	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
}
