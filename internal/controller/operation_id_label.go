// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
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
// operation-id label is invalid.
func rejectInvalidOperationIDLabel(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	conds *[]operatorv1alpha1.Condition,
	emit func(reason errx.Reason, msg string),
	setPhase func(),
	msg string,
) (ctrl.Result, error) {
	return rejectInvalidLabel(ctx, c, obj, conds, emit, setPhase,
		operatorv1alpha1.EventOperationIDLabelInvalid,
		operatorv1alpha1.CondOperationIDLabelValid,
		operatorv1alpha1.ReasonOperationIDLabelInvalid,
		msg)
}

// rejectInvalidLabel is the shared terminal-rejection tail for the
// NetworkUpgrade* label invariants (operation-id #1066, step #1133): it emits a
// Warning event, sets the terminal phase, records a False condition, and
// persists status. setPhase is a closure because the terminal phase differs per
// CR (Phase vs ExecutePhase).
func rejectInvalidLabel(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	conds *[]operatorv1alpha1.Condition,
	emit func(reason errx.Reason, msg string),
	setPhase func(),
	eventReason errx.Reason,
	condType string,
	condReason errx.Reason,
	msg string,
) (ctrl.Result, error) {
	emit(eventReason, msg)
	setPhase()
	setCondition(conds, metav1.Condition{
		Type:               condType,
		Status:             metav1.ConditionFalse,
		Reason:             condReason.String(),
		Message:            msg,
		ObservedGeneration: obj.GetGeneration(),
	})
	if err := c.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
	}
	return ctrl.Result{}, nil
}
