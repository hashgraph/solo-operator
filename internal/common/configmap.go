// SPDX-License-Identifier: Apache-2.0

package common

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapNeedsUpdate reports whether an existing ConfigMap differs from the desired one.
func ConfigMapNeedsUpdate(existing, desired *corev1.ConfigMap) bool {
	return !maps.Equal(existing.Data, desired.Data) ||
		!maps.Equal(existing.Labels, desired.Labels) ||
		!SameController(existing, desired)
}

// SameController reports whether existing and desired share the same controller owner
// reference by UID. A missing controller ref on either side counts as a difference.
func SameController(existing, desired *corev1.ConfigMap) bool {
	ec := metav1.GetControllerOf(existing)
	dc := metav1.GetControllerOf(desired)
	if ec == nil || dc == nil {
		return ec == nil && dc == nil
	}
	return ec.UID == dc.UID
}
