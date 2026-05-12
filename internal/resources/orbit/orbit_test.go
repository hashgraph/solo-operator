// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestStandardLabels(t *testing.T) {
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
	}

	labels := StandardLabels(orbit)

	if labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("Expected managed-by label %s, got %s", ManagedByValue, labels[LabelManagedBy])
	}
	if labels[LabelOrbit] != "test-orbit" {
		t.Errorf("Expected orbit label 'test-orbit', got %s", labels[LabelOrbit])
	}
}
