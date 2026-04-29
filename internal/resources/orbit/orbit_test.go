// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func TestStandardLabels(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
	}

	labels := StandardLabels(orbit)

	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("Expected managed-by label %s, got %s", ManagedByValue, labels[LabelManagedBy])
	}

	if labels[LabelOrbit] != "test-orbit" {
		t.Errorf("Expected orbit label %s, got %s", "test-orbit", labels[LabelOrbit])
	}
}

func TestNewSubReconcilers(t *testing.T) {
	chain := NewSubReconcilers(nil, nil)

	if chain == nil {
		t.Fatal("Expected chain to be created")
	}
}

func TestNamespaceSubReconciler_Name(t *testing.T) {
	sr := NewNamespaceSubReconciler(nil, nil)

	if sr.Name() != "OrbitNamespace" {
		t.Errorf("Expected name 'OrbitNamespace', got %s", sr.Name())
	}
}

func TestConfigMapSubReconciler_Name(t *testing.T) {
	sr := NewConfigMapSubReconciler(nil, nil)

	if sr.Name() != "ConsensusConfigMap" {
		t.Errorf("Expected name 'ConsensusConfigMap', got %s", sr.Name())
	}
}

func TestRoleSubReconciler_Name(t *testing.T) {
	sr := NewRoleSubReconciler(nil, nil)

	if sr.Name() != "ConsensusCapsuleRole" {
		t.Errorf("Expected name 'ConsensusCapsuleRole', got %s", sr.Name())
	}
}

func TestHeadlessServiceSubReconciler_Name(t *testing.T) {
	sr := NewHeadlessServiceSubReconciler(nil, nil)

	if sr.Name() != "ConsensusHeadlessService" {
		t.Errorf("Expected name 'ConsensusHeadlessService', got %s", sr.Name())
	}
}

func TestBuildConsensusConfigData(t *testing.T) {
	data := buildConsensusConfigData()

	expectedKeys := []string{
		"empty",
		"s6-start-consensus-run",
		"s6-type-longrun",
		"s6-start-consensus-finish",
		"is-consensus-active",
	}

	for _, key := range expectedKeys {
		if _, ok := data[key]; !ok {
			t.Errorf("Expected key %s in config data", key)
		}
	}

	if data["s6-type-longrun"] != "longrun" {
		t.Errorf("Expected s6-type-longrun to be 'longrun', got %s", data["s6-type-longrun"])
	}
}

func TestNamespaceSubReconciler_buildNamespace(t *testing.T) {
	sr := NewNamespaceSubReconciler(nil, nil)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
	}

	ns := sr.buildNamespace(orbit)

	if ns.Name != "test-orbit" {
		t.Errorf("Expected namespace name 'test-orbit', got %s", ns.Name)
	}

	if ns.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("Expected managed-by label")
	}

	if ns.Labels[LabelOrbit] != "test-orbit" {
		t.Errorf("Expected orbit label")
	}
}

func TestConfigMapSubReconciler_buildConfigMap(t *testing.T) {
	sr := NewConfigMapSubReconciler(nil, nil)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
	}

	cm := sr.buildConfigMap(orbit)

	if cm.Name != ConsensusConfigMapName {
		t.Errorf("Expected ConfigMap name %s, got %s", ConsensusConfigMapName, cm.Name)
	}

	if cm.Namespace != "test-orbit" {
		t.Errorf("Expected namespace 'test-orbit', got %s", cm.Namespace)
	}

	if len(cm.Data) == 0 {
		t.Error("Expected ConfigMap to have data")
	}
}

func TestRoleSubReconciler_buildRole(t *testing.T) {
	sr := NewRoleSubReconciler(nil, nil)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
	}

	role := sr.buildRole(orbit)

	if role.Name != ConsensusCapsuleRoleName {
		t.Errorf("Expected Role name %s, got %s", ConsensusCapsuleRoleName, role.Name)
	}

	if role.Namespace != "test-orbit" {
		t.Errorf("Expected namespace 'test-orbit', got %s", role.Namespace)
	}

	if len(role.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(role.Rules))
	}

	// Check configmaps rule
	foundConfigMaps := false
	foundSecrets := false
	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			if resource == "configmaps" {
				foundConfigMaps = true
			}
			if resource == "secrets" {
				foundSecrets = true
			}
		}
	}

	if !foundConfigMaps {
		t.Error("Expected configmaps rule")
	}

	if !foundSecrets {
		t.Error("Expected secrets rule")
	}
}

func TestHeadlessServiceSubReconciler_buildService(t *testing.T) {
	sr := NewHeadlessServiceSubReconciler(nil, nil)
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-orbit",
		},
	}

	svc := sr.buildService(orbit)

	if svc.Name != common.ConsensusSubdomain {
		t.Errorf("Expected Service name %s, got %s", common.ConsensusSubdomain, svc.Name)
	}

	if svc.Namespace != "test-orbit" {
		t.Errorf("Expected namespace 'test-orbit', got %s", svc.Namespace)
	}

	if svc.Spec.ClusterIP != "None" {
		t.Errorf("Expected headless service (ClusterIP=None), got %s", svc.Spec.ClusterIP)
	}

	if !svc.Spec.PublishNotReadyAddresses {
		t.Error("Expected PublishNotReadyAddresses=true so pods are DNS-resolvable before they are Ready")
	}

	if len(svc.Spec.Ports) != 3 {
		t.Errorf("Expected 3 ports, got %d", len(svc.Spec.Ports))
	}

	// Check port names
	portNames := make(map[string]bool)
	for _, port := range svc.Spec.Ports {
		portNames[port.Name] = true
	}

	if !portNames["gossip"] {
		t.Error("Expected gossip port")
	}
	if !portNames["grpc"] {
		t.Error("Expected grpc port")
	}
	if !portNames["grpcs"] {
		t.Error("Expected grpcs port")
	}
}

func TestHeadlessServiceSubReconciler_needsUpdate(t *testing.T) {
	sr := NewHeadlessServiceSubReconciler(nil, nil)
	orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"}}
	desired := sr.buildService(orbit)

	existing := sr.buildService(orbit)
	if sr.needsUpdate(existing, desired) {
		t.Error("identical services should not need update")
	}

	existing.Spec.PublishNotReadyAddresses = false
	if !sr.needsUpdate(existing, desired) {
		t.Error("expected needsUpdate=true when PublishNotReadyAddresses differs")
	}
}

func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "equal slices",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different length",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "different content",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringSliceEqual(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("stringSliceEqual() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestMapsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]string
		b        map[string]string
		expected bool
	}{
		{
			name:     "equal maps",
			a:        map[string]string{"key": "value"},
			b:        map[string]string{"key": "value"},
			expected: true,
		},
		{
			name:     "different length",
			a:        map[string]string{"key": "value"},
			b:        map[string]string{"key": "value", "key2": "value2"},
			expected: false,
		},
		{
			name:     "different values",
			a:        map[string]string{"key": "value1"},
			b:        map[string]string{"key": "value2"},
			expected: false,
		},
		{
			name:     "both empty",
			a:        map[string]string{},
			b:        map[string]string{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mapsEqual(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("mapsEqual() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
