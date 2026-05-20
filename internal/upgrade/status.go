// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition writes a condition onto the slice, replacing any existing entry
// with the same Type. Returns true when the condition was added or any of its
// fields changed (other than LastTransitionTime), so callers can elide
// no-op Status().Update calls.
func SetCondition(conditions *[]metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string, observedGeneration int64) bool {
	return apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: observedGeneration,
		LastTransitionTime: metav1.Now(),
	})
}
