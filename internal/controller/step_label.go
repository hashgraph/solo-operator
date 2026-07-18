// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// validateStepLabel enforces the step label invariant: it must be present and
// equal the reconciler's canonical step for the CR kind (prepare/freeze/
// execute/abort). Unlike the operation-id label there is no spec field to
// cross-check against — each reconciler knows its own canonical step and passes
// it as expectedStep.
func validateStepLabel(labels map[string]string, expectedStep string) (ok bool, msg string) {
	label, present := labels[operatorv1alpha1.LabelStep]
	if !present || label == "" {
		return false, fmt.Sprintf("step label %q is required but missing", operatorv1alpha1.LabelStep)
	}
	if label != expectedStep {
		return false, fmt.Sprintf(
			"step label %q=%q does not match expected step %q",
			operatorv1alpha1.LabelStep, label, expectedStep,
		)
	}
	return true, ""
}

// rejectInvalidStepLabel terminally rejects a NetworkUpgrade* CR whose step
// label is invalid, mirroring rejectInvalidOperationIDLabel (#1066): it emits a
// StepLabelInvalid Warning event, sets the terminal phase via setPhase, and
// records a StepLabelValid=False condition.
func rejectInvalidStepLabel(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	conds *[]operatorv1alpha1.Condition,
	emit func(reason errx.Reason, msg string),
	setPhase func(),
	msg string,
) (ctrl.Result, error) {
	return rejectInvalidLabel(ctx, c, obj, conds, emit, setPhase,
		operatorv1alpha1.EventStepLabelInvalid,
		operatorv1alpha1.CondStepLabelValid,
		operatorv1alpha1.ReasonStepLabelInvalid,
		msg)
}
