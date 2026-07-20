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

	role := (&RoleSubReconciler{}).buildRole(capsule)

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
	role := (&RoleSubReconciler{}).buildRole(capsule)
	if rule := findRule(role.Rules, "pods/status"); rule != nil {
		t.Errorf("expected no pods/status rule without the UC sidecar, got %+v", rule)
	}
}
