// SPDX-License-Identifier: Apache-2.0

package common

import "testing"

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
