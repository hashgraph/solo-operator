// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// findRule returns the first rule granting a verb on the given resource, or nil.
func findRule(rules []rbacv1.PolicyRule, resource string) *rbacv1.PolicyRule {
	for i := range rules {
		for _, res := range rules[i].Resources {
			if res == resource {
				return &rules[i]
			}
		}
	}
	return nil
}

// TestBuildRole_UCPodStatusScopedToOwnPod verifies the UC's pods/status patch grant
// is restricted via resourceNames to the capsule's single StatefulSet pod ("<capsule>-0"),
// so the sidecar cannot patch any other pod's status in the namespace.
func TestBuildRole_UCPodStatusScopedToOwnPod(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}

	role := (&RoleSubReconciler{}).buildRole(capsule, true)

	rule := findRule(role.Rules, "pods/status")
	if rule == nil {
		t.Fatal("expected a pods/status rule when the UC sidecar is enabled")
	}
	if len(rule.Verbs) != 1 || rule.Verbs[0] != "patch" {
		t.Errorf("expected verbs [patch], got %v", rule.Verbs)
	}
	wantName := PodName(capsule) // "node0-0"
	if len(rule.ResourceNames) != 1 || rule.ResourceNames[0] != wantName {
		t.Errorf("expected resourceNames [%q] (own pod only), got %v", wantName, rule.ResourceNames)
	}
}

// TestBuildRole_NoUCNoPodStatus verifies the pods/status grant is only present when the
// UC sidecar is enabled — a capsule without the sidecar gets no pod-patch permission.
func TestBuildRole_NoUCNoPodStatus(t *testing.T) {
	capsule := newTestCapsule() // no UC sidecar
	role := (&RoleSubReconciler{}).buildRole(capsule, false)
	if rule := findRule(role.Rules, "pods/status"); rule != nil {
		t.Errorf("expected no pods/status rule without the UC sidecar, got %+v", rule)
	}
}

// TestBuildRole_UCConfigCRGrants verifies the UC sidecar is granted create+get on
// every ConsensusConfig CR kind it stages during the execute phase, in cluster-only mode.
// Without these the provisioner-proxy's config-CR staging is forbidden (see
// internal/uc/config_cr.go). The resource plurals here must stay in lockstep with the CRD
// spec.names.plural and the GVRs in internal/uc/k8s.go.
func TestBuildRole_UCConfigCRGrants(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}

	role := (&RoleSubReconciler{}).buildRole(capsule, true) // cluster-only

	// Every config CR kind the UC creates and polls (Get) via createConfigCR.
	wantResources := []string{
		"log4j2configs",
		"nodesettings",
		"applicationproperties",
		"applicationoverrideproperties",
		"bootstrapproperties",
		"nodepropertiesconfigs",
		"throttlesconfigs",
		"feeschedules",
		"simplefeesschedules",
		"apipermissionproperties",
		"blocknodesconfigs",
	}
	wantVerbs := map[string]bool{"create": true, "get": true}

	for _, res := range wantResources {
		rule := findRule(role.Rules, res)
		if rule == nil {
			t.Errorf("expected a grant for %q, got none", res)
			continue
		}
		got := map[string]bool{}
		for _, v := range rule.Verbs {
			got[v] = true
		}
		for v := range wantVerbs {
			if !got[v] {
				t.Errorf("resource %q: missing verb %q (got %v)", res, v, rule.Verbs)
			}
		}
		// Exactly create+get — no list/watch/update/patch. UC only creates config
		// CRs and polls them (Get); anything more is over-privilege.
		for v := range got {
			if !wantVerbs[v] {
				t.Errorf("resource %q: unexpected verb %q (got %v)", res, v, rule.Verbs)
			}
		}
	}
}

// TestBuildRole_NoUCNoConfigCRGrants verifies the config-CR grants are gated on the UC
// sidecar: a capsule without the sidecar gets none of them.
func TestBuildRole_NoUCNoConfigCRGrants(t *testing.T) {
	capsule := newTestCapsule() // no UC sidecar
	role := (&RoleSubReconciler{}).buildRole(capsule, false)
	if rule := findRule(role.Rules, "apipermissionproperties"); rule != nil {
		t.Errorf("expected no config-CR grant without the UC sidecar, got %+v", rule)
	}
}

// TestBuildRole_MainnetNoConfigCRGrants verifies that with the UC sidecar enabled but in
// mainnet mode (host provisioner daemon), the config-CR grant is withheld — the UC never
// creates config CRs there, so granting them would violate least-privilege. The
// NetworkUpgrade* and pods/status grants remain, since they apply in both modes.
func TestBuildRole_MainnetNoConfigCRGrants(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}

	role := (&RoleSubReconciler{}).buildRole(capsule, false) // mainnet

	if rule := findRule(role.Rules, "apipermissionproperties"); rule != nil {
		t.Errorf("expected no config-CR grant in mainnet mode, got %+v", rule)
	}
	// The network-upgrade grants are mode-independent and must still be present.
	if rule := findRule(role.Rules, "networkupgradeexecutes"); rule == nil {
		t.Error("expected networkupgradeexecutes grant in mainnet mode, got none")
	}
}

func TestBuildRole_NoConfigMapOrSecretRead(t *testing.T) {
	capsule := newTestCapsule()
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}
	for _, clusterOnly := range []bool{true, false} {
		role := (&RoleSubReconciler{}).buildRole(capsule, clusterOnly)
		for _, res := range []string{"secrets", "configmaps"} {
			if rule := findRule(role.Rules, res); rule != nil {
				t.Errorf("clusterOnly=%v: unexpected %s grant %+v", clusterOnly, res, rule)
			}
		}
	}
}
