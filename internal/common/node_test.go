// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestInternalHostname(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	if got, want := InternalHostname(capsule, "consensus"), "node0-0.consensus.test-ns.svc.cluster.local"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestInternalHostname_Override(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
		Spec:       operatorv1alpha1.ConsensusCapsuleSpec{InternalAddress: "10.0.0.5"},
	}
	if got := InternalHostname(capsule, "consensus"); got != "10.0.0.5" {
		t.Errorf("expected override, got %q", got)
	}
}

func TestNodeAlias(t *testing.T) {
	tests := []struct {
		nodeId   int64
		expected string
	}{
		{0, "node0"},
		{1, "node1"},
		{42, "node42"},
	}
	for _, tc := range tests {
		if got := NodeAlias(tc.nodeId); got != tc.expected {
			t.Errorf("NodeAlias(%d) = %q, want %q", tc.nodeId, got, tc.expected)
		}
	}
}

func TestAccountIdForNode(t *testing.T) {
	tests := []struct {
		nodeId   int64
		expected string
	}{
		{0, "0.0.3"},
		{1, "0.0.4"},
		{10, "0.0.13"},
	}
	for _, tc := range tests {
		if got := AccountIdForNode(tc.nodeId); got != tc.expected {
			t.Errorf("AccountIdForNode(%d) = %q, want %q", tc.nodeId, got, tc.expected)
		}
	}
}

func TestAccountNum(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0.0.3", "3"},
		{"0.0.100", "100"},
		{"1.2.999", "999"},
		{"notdotted", "notdotted"},
		{"only.two", "only.two"},
	}
	for _, tc := range tests {
		if got := AccountNum(tc.input); got != tc.expected {
			t.Errorf("AccountNum(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
