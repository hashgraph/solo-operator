// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"maps"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const AnnotationOwnerCreatedAt = "operator.solo.hiero.org/owner-created-at"

const ConditionReasonSuperseded = "Superseded"

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
// a different controller than the caller. Same-kind takeover is allowed so that
// an upgrade-scoped config CR can replace the static CR's ConfigMap (both are
// the same Kind, e.g. ApiPermissionProperties, but different UIDs). A conflict
// is reported only when the existing owner is a different Kind entirely.
func ConfigMapOwnerConflict(existing *corev1.ConfigMap, caller client.Object) string {
	ctrl := metav1.GetControllerOf(existing)
	if ctrl == nil {
		return ""
	}
	if ctrl.UID == caller.GetUID() {
		return ""
	}
	callerGVK := caller.GetObjectKind().GroupVersionKind()
	if ctrl.Kind == callerGVK.Kind && ctrl.APIVersion == callerGVK.GroupVersion().String() {
		return ""
	}
	return fmt.Sprintf("ConfigMap %q is already owned by %s/%s (uid %s)",
		existing.Name, ctrl.Kind, ctrl.Name, ctrl.UID)
}

// ClearControllerRef removes the existing controller owner reference from obj
// so that a new controller of the same Kind can adopt it via SetControllerReference.
func ClearControllerRef(obj metav1.Object) {
	refs := obj.GetOwnerReferences()
	filtered := make([]metav1.OwnerReference, 0, len(refs))
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			continue
		}
		filtered = append(filtered, r)
	}
	obj.SetOwnerReferences(filtered)
}

// ConfigMapSuperseded reports whether the caller CR is older than the CR that
// last wrote the ConfigMap. It compares the caller's creationTimestamp against
// the AnnotationOwnerCreatedAt annotation on the ConfigMap. When the caller is
// older (or equal-age but different UID), it should skip writing so that only
// the most recently created same-Kind CR owns the ConfigMap.
func ConfigMapSuperseded(existing *corev1.ConfigMap, caller client.Object) bool {
	ctrl := metav1.GetControllerOf(existing)
	if ctrl == nil {
		return false
	}
	if ctrl.UID == caller.GetUID() {
		return false
	}
	ann := existing.GetAnnotations()
	if ann == nil {
		return false
	}
	raw, ok := ann[AnnotationOwnerCreatedAt]
	if !ok {
		return false
	}
	ownerCreated, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	return !caller.GetCreationTimestamp().After(ownerCreated)
}

// StampOwnerCreatedAt records the caller's creationTimestamp on the ConfigMap
// so that older same-Kind CRs can detect they have been superseded.
func StampOwnerCreatedAt(cm *corev1.ConfigMap, caller client.Object) {
	ann := cm.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ann[AnnotationOwnerCreatedAt] = caller.GetCreationTimestamp().UTC().Format(time.RFC3339)
	cm.SetAnnotations(ann)
}
