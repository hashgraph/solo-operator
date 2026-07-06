// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// validateOperationIDLabel enforces the HIP-required operation-id label
// invariant: it must be present and equal spec.operationId, which is the source
// of truth.
func validateOperationIDLabel(labels map[string]string, specOperationID string) (ok bool, msg string) {
	label, present := labels[operatorv1alpha1.LabelOperationID]
	if !present || label == "" {
		return false, fmt.Sprintf("operation-id label %q is required but missing", operatorv1alpha1.LabelOperationID)
	}
	if label != specOperationID {
		return false, fmt.Sprintf(
			"operation-id label %q=%q does not match spec.operationId %q",
			operatorv1alpha1.LabelOperationID, label, specOperationID,
		)
	}
	return true, ""
}

// rejectInvalidOperationIDLabel terminally rejects a NetworkUpgrade* CR whose
// operation-id label is invalid. setPhase is a closure because the terminal
// phase differs per CR (Phase vs ExecutePhase).
func rejectInvalidOperationIDLabel(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	conds *[]operatorv1alpha1.Condition,
	emit func(reason, msg string),
	setPhase func(),
	msg string,
) (ctrl.Result, error) {
	emit(operatorv1alpha1.EventOperationIDLabelInvalid, msg)
	setPhase()
	setCondition(conds, metav1.Condition{
		Type:               operatorv1alpha1.CondOperationIDLabelValid,
		Status:             metav1.ConditionFalse,
		Reason:             operatorv1alpha1.ReasonOperationIDLabelInvalid,
		Message:            msg,
		ObservedGeneration: obj.GetGeneration(),
	})
	if err := c.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}
	return ctrl.Result{}, nil
}
