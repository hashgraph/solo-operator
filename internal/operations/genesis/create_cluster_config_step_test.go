// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func clusterConfigOp() *operatorv1alpha1.NetworkOperation {
	return &operatorv1alpha1.NetworkOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-op", Namespace: "test"},
		Spec:       operatorv1alpha1.NetworkOperationSpec{Orbit: "test-orbit"},
	}
}

func orbitWithLedger(name, ledgerId string) *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						LedgerId: ledgerId,
					},
				},
			},
		},
	}
}

func plainCapsule(name string, nodeId int64) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Spec:       operatorv1alpha1.ConsensusCapsuleSpec{NodeId: nodeId},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCreateClusterConfigStep_Run_Success(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	step := NewCreateClusterConfigStep(c)
	status := step.Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm); err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}

	required := []string{ConfigKeyConfigTxt, ConfigKeyPodIPs, "settings.txt", "application.properties", "api-permission.properties", "bootstrap.properties", "genesis-throttles.json"}
	for _, key := range required {
		if _, ok := cm.Data[key]; !ok {
			t.Errorf("ConfigMap missing required key: %s", key)
		}
	}
}

func TestCreateClusterConfigStep_Run_OrbitNotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(plainCapsule("node0", 0)).Build()
	status := NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when orbit is not found")
	}
}

func TestCreateClusterConfigStep_Run_NoCapsules(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).WithObjects(orbit).Build()
	status := NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{})

	if status.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected PhaseFailed when no capsules exist")
	}
}

func TestCreateClusterConfigStep_Run_Update(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterConfigName, Namespace: "test"},
		Data:       map[string]string{"stale-key": "stale-value"},
	}

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule, existing).Build()
	status := NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	if status.Phase == operatorv1alpha1.PhaseFailed {
		t.Fatalf("expected success on update: %v", status.Conditions)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm); err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if _, ok := cm.Data[ConfigKeyConfigTxt]; !ok {
		t.Error("updated ConfigMap should contain config.txt")
	}
	if _, ok := cm.Data["stale-key"]; ok {
		t.Error("updated ConfigMap should not retain stale keys")
	}
}

func TestCreateClusterConfigStep_ConfigTxt_DefaultAccountId(t *testing.T) {
	// capsule with no accountId should default to common.AccountIdForNode(nodeId)
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	step := NewCreateClusterConfigStep(c)
	step.Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	expected := common.AccountIdForNode(0) // "0.0.3"
	if !strings.Contains(cm.Data[ConfigKeyConfigTxt], expected) {
		t.Errorf("config.txt should contain default account %s, got:\n%s", expected, cm.Data[ConfigKeyConfigTxt])
	}
}

func TestCreateClusterConfigStep_ConfigTxt_CustomAccountId(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)
	capsule.Spec.AccountId = "0.0.999"

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	if !strings.Contains(cm.Data[ConfigKeyConfigTxt], "0.0.999") {
		t.Errorf("config.txt should contain custom accountId 0.0.999, got:\n%s", cm.Data[ConfigKeyConfigTxt])
	}
}

func TestCreateClusterConfigStep_ConfigTxt_LedgerId(t *testing.T) {
	orbit := orbitWithLedger("test-orbit", "298")
	capsule := plainCapsule("node0", 0)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	if !strings.HasPrefix(cm.Data[ConfigKeyConfigTxt], "swirld, 298") {
		t.Errorf("config.txt should start with ledgerId 298, got:\n%s", cm.Data[ConfigKeyConfigTxt])
	}
}

func TestCreateClusterConfigStep_ConfigTxt_SortedByNodeId(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	// Add nodes out of order; expect config.txt sorted by nodeId
	node1 := plainCapsule("node1", 1)
	node0 := plainCapsule("node0", 0)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, node1, node0).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	configTxt := cm.Data[ConfigKeyConfigTxt]
	pos0 := strings.Index(configTxt, "address, 0,")
	pos1 := strings.Index(configTxt, "address, 1,")
	if pos0 < 0 || pos1 < 0 {
		t.Fatalf("config.txt missing expected address entries:\n%s", configTxt)
	}
	if pos0 > pos1 {
		t.Error("node0 address should appear before node1 in config.txt")
	}
}

func TestCreateClusterConfigStep_ConfigTxt_ExternalAddress(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: &operatorv1alpha1.OrbitGenesis{
					AddressBook: &operatorv1alpha1.OrbitAddressBook{
						ExternalAddresses: []operatorv1alpha1.AddressBookEntry{
							{
								NodeId:           5,
								NickName:         "ext",
								Weight:           100,
								InternalHostname: "10.0.0.5",
								InternalPort:     50111,
								ExternalHostname: "ext.example.com",
								ExternalPort:     50111,
								NodeAccount:      "0.0.8",
							},
						},
					},
				},
			},
		},
	}
	capsule := plainCapsule("node0", 0)

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	configTxt := cm.Data[ConfigKeyConfigTxt]
	if !strings.Contains(configTxt, "ext.example.com") {
		t.Errorf("config.txt should contain external address entry, got:\n%s", configTxt)
	}
	if !strings.Contains(configTxt, "0.0.8") {
		t.Errorf("config.txt should contain external node account 0.0.8, got:\n%s", configTxt)
	}
}

func TestCreateClusterConfigStep_ConfigTxt_CustomGossipPort(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)
	capsule.Spec.GossipPort = 60111

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	configTxt := cm.Data[ConfigKeyConfigTxt]
	if !strings.Contains(configTxt, "60111") {
		t.Errorf("config.txt should contain custom gossip port 60111, got:\n%s", configTxt)
	}
	if strings.Contains(configTxt, "50111") {
		t.Errorf("config.txt should not contain default port 50111 when custom port is set, got:\n%s", configTxt)
	}
}

func TestCreateClusterConfigStep_ShouldSkip(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	op := clusterConfigOp()

	if step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhasePending}) {
		t.Error("should not skip when step has not run yet")
	}

	if !step.ShouldSkip(op, &GenesisStatus{Phase: operatorv1alpha1.PhaseFailed}) {
		t.Error("should skip when phase is Failed")
	}

	succeeded := &GenesisStatus{Phase: operatorv1alpha1.PhaseRunning}
	succeeded.AddCondition(metav1.Condition{
		Type:   StepConditionTypeConfigCreated,
		Status: metav1.ConditionTrue,
		Reason: StepConditionReasonClusterConfig,
	})
	if !step.ShouldSkip(op, succeeded) {
		t.Error("should skip when step already succeeded")
	}
}

// ── collectPodIPs ─────────────────────────────────────────────────────────────

func makeTestPod(name, ip string, phase corev1.PodPhase, ready, deleting bool) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Status: corev1.PodStatus{
			Phase: phase,
			PodIP: ip,
		},
	}
	if ready {
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
		}
	} else {
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse},
		}
	}
	if deleting {
		now := metav1.Now()
		pod.DeletionTimestamp = &now
	}
	return pod
}

func TestCollectPodIPs_ReadyRunningPod(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	pods := &corev1.PodList{Items: []corev1.Pod{
		makeTestPod("node0", "10.0.0.1", corev1.PodRunning, true, false),
	}}

	ips := step.collectPodIPs(pods)
	if len(ips) != 1 || ips[0] != "10.0.0.1" {
		t.Errorf("expected [10.0.0.1], got %v", ips)
	}
}

func TestCollectPodIPs_NotReadyPodExcluded(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	pods := &corev1.PodList{Items: []corev1.Pod{
		makeTestPod("node0", "10.0.0.1", corev1.PodRunning, false, false),
	}}

	ips := step.collectPodIPs(pods)
	if len(ips) != 0 {
		t.Errorf("expected empty slice for not-ready pod, got %v", ips)
	}
}

func TestCollectPodIPs_PendingPodExcluded(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	pods := &corev1.PodList{Items: []corev1.Pod{
		makeTestPod("node0", "10.0.0.1", corev1.PodPending, true, false),
	}}

	ips := step.collectPodIPs(pods)
	if len(ips) != 0 {
		t.Errorf("expected empty slice for pending pod, got %v", ips)
	}
}

func TestCollectPodIPs_DeletingPodExcluded(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	pods := &corev1.PodList{Items: []corev1.Pod{
		makeTestPod("node0", "10.0.0.1", corev1.PodRunning, true, true),
	}}

	ips := step.collectPodIPs(pods)
	if len(ips) != 0 {
		t.Errorf("expected empty slice for deleting pod, got %v", ips)
	}
}

func TestCollectPodIPs_MixedPods(t *testing.T) {
	step := NewCreateClusterConfigStep(nil)
	pods := &corev1.PodList{Items: []corev1.Pod{
		makeTestPod("node0", "10.0.0.1", corev1.PodRunning, true, false),  // included
		makeTestPod("node1", "10.0.0.2", corev1.PodRunning, false, false), // not ready
		makeTestPod("node2", "10.0.0.3", corev1.PodPending, true, false),  // pending phase
		makeTestPod("node3", "10.0.0.4", corev1.PodRunning, true, true),   // deleting
	}}

	ips := step.collectPodIPs(pods)
	if len(ips) != 1 || ips[0] != "10.0.0.1" {
		t.Errorf("expected only node0 IP, got %v", ips)
	}
}

// ── InternalHostname ─────────────────────────────────────────────────────────

func TestInternalHostname_DefaultsDNS(t *testing.T) {
	capsule := plainCapsule("node0", 0) // Namespace: "test"
	got := common.InternalHostname(capsule)
	want := "node0.test.svc.cluster.local"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestInternalHostname_OverrideUsed(t *testing.T) {
	capsule := plainCapsule("node0", 0)
	capsule.Spec.InternalAddress = "10.0.0.5"
	if got := common.InternalHostname(capsule); got != "10.0.0.5" {
		t.Errorf("expected InternalAddress override, got %q", got)
	}
}

func TestConfigTxt_UsesInternalHostname(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	capsule := plainCapsule("node0", 0)
	capsule.Spec.InternalAddress = "192.168.1.10"

	c := fake.NewClientBuilder().WithScheme(genesisTestScheme()).
		WithObjects(orbit, capsule).Build()
	NewCreateClusterConfigStep(c).Run(context.Background(), clusterConfigOp(), &GenesisStatus{Phase: operatorv1alpha1.PhasePending})

	cm := &corev1.ConfigMap{}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "test", Name: ClusterConfigName}, cm)

	if !strings.Contains(cm.Data[ConfigKeyConfigTxt], "192.168.1.10") {
		t.Errorf("config.txt should use InternalAddress override, got:\n%s", cm.Data[ConfigKeyConfigTxt])
	}
}
