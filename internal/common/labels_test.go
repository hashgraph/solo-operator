// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStandardLabels(t *testing.T) {
	labels := StandardLabels("test-config", "test-orbit")

	expected := map[string]string{
		"app.kubernetes.io/name":       "test-config",
		"app.kubernetes.io/instance":   "test-orbit",
		"app.kubernetes.io/managed-by": "solo-operator",
		"app.kubernetes.io/part-of":    "consensus-node",
		"app.kubernetes.io/component":  "config-map",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("label %q = %q, want %q", k, labels[k], v)
		}
	}
}

func TestPropagateOperationID(t *testing.T) {
	t.Run("copies label when present", func(t *testing.T) {
		src := &metav1.ObjectMeta{Labels: map[string]string{LabelOperationID: "op-42"}}
		dst := map[string]string{"existing": "value"}
		PropagateOperationID(dst, src)
		if dst[LabelOperationID] != "op-42" {
			t.Errorf("got %q, want %q", dst[LabelOperationID], "op-42")
		}
		if dst["existing"] != "value" {
			t.Error("existing label was clobbered")
		}
	})

	t.Run("no-op when label absent", func(t *testing.T) {
		src := &metav1.ObjectMeta{Labels: map[string]string{"other": "x"}}
		dst := map[string]string{"existing": "value"}
		PropagateOperationID(dst, src)
		if _, ok := dst[LabelOperationID]; ok {
			t.Error("operation-id label should not be set when absent on source")
		}
	})

	t.Run("no-op when source has nil labels", func(t *testing.T) {
		src := &metav1.ObjectMeta{}
		dst := map[string]string{}
		PropagateOperationID(dst, src)
		if _, ok := dst[LabelOperationID]; ok {
			t.Error("operation-id label should not be set when source labels are nil")
		}
	})
}
