// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// ConfigMapOwnerConflict checks whether an existing ConfigMap is already owned by
// a controller of a different kind than the caller. Returns a human-readable error
// message if there is a conflict, or "" if the ConfigMap is safe to adopt.
func ConfigMapOwnerConflict(existing *corev1.ConfigMap, caller client.Object) string {
	ctrl := metav1.GetControllerOf(existing)
	if ctrl == nil {
		return ""
	}
	callerKind := caller.GetObjectKind().GroupVersionKind().Kind
	if ctrl.Kind == callerKind && ctrl.UID == caller.GetUID() {
		return ""
	}
	if ctrl.Kind != callerKind {
		return fmt.Sprintf("ConfigMap %q is already owned by %s/%s", existing.Name, ctrl.Kind, ctrl.Name)
	}
	return ""
}
