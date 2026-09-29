// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

type NetworkUpgradeFreezeAbortReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

func NewNetworkUpgradeFreezeAbortReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeFreezeAbortReconciler {
	return &NetworkUpgradeFreezeAbortReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezeaborts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezeaborts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezeaborts/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes,verbs=get;list;watch

func (r *NetworkUpgradeFreezeAbortReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var abort operatorv1alpha1.NetworkUpgradeFreezeAbort
	if err := r.Get(ctx, req.NamespacedName, &abort); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradefreezeabort", "NetworkUpgradeFreezeAbort", &abort)
	common.Info(ctx, ReasonReconcileStart).Msg("Starting NetworkUpgradeFreezeAbort reconciliation")
	defer common.Info(ctx, ReasonReconcileFinish).Msg("Finished NetworkUpgradeFreezeAbort reconciliation")

	if res, err := ensureFinalizer(ctx, r.Client, &abort, FinalizerNetworkUpgradeFreezeAbort); res != nil {
		return *res, err
	}

	if abort.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		abort.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if ok, msg := validateOperationIDLabel(abort.Labels, abort.Spec.OperationID); !ok {
		return rejectInvalidOperationIDLabel(ctx, r.Client, &abort, &abort.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &abort, corev1.EventTypeWarning, reason, "%s", m) },
			func() { abort.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	if ok, msg := validateStepLabel(abort.Labels, operatorv1alpha1.StepAbort); !ok {
		return rejectInvalidStepLabel(ctx, r.Client, &abort, &abort.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &abort, corev1.EventTypeWarning, reason, "%s", m) },
			func() { abort.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	sibling, err := abortSiblingExists(ctx, r.Client, abort.Namespace, string(abort.UID), abort.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if sibling {
		msg := fmt.Sprintf("Duplicate operationId %q; already used by another NetworkUpgradeFreezeAbort", abort.Spec.OperationID)
		r.eventf(ctx, &abort, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
		abort.Status.Phase = operatorv1alpha1.PhaseFailed
		abort.Status.ObservedGeneration = abort.Generation
		setCondition(&abort.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondManifestValidated,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonDuplicateOpID.String(),
			Message:            msg,
			ObservedGeneration: abort.Generation,
		})
		if err := r.Status().Update(ctx, &abort); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
		}
		return ctrl.Result{}, errx.WithReason(errReconcile.New("%s", msg), ReasonManifestInvalid)
	}

	if abort.Status.Phase == "" || abort.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		abort.Status.StartTime = &now
		abort.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &abort); err != nil {
			return ctrl.Result{}, wrapStatusInit(err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	executeExists, err := r.anyExecuteExists(ctx, abort.Namespace, abort.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}

	completionTime := metav1.Now()
	abort.Status.CompletionTime = &completionTime
	abort.Status.ObservedGeneration = abort.Generation

	if executeExists {
		r.eventf(ctx, &abort, corev1.EventTypeWarning, operatorv1alpha1.EventAbortAfterFreeze,
			"NetworkUpgradeExecute CR already exists for operationId %q; abort has no effect after freeze", abort.Spec.OperationID)
		abort.Status.Phase = operatorv1alpha1.PhaseFailed
		setCondition(&abort.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondAbortProcessed,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonAbortAfterFreeze.String(),
			Message: "abort received after NetworkUpgradeExecute was created; freeze window already closed",
		})
	} else {
		r.eventf(ctx, &abort, corev1.EventTypeNormal, operatorv1alpha1.EventAbortProcessed,
			"Abort processed for operationId %q; upgrade cancelled before freeze", abort.Spec.OperationID)
		abort.Status.Phase = operatorv1alpha1.PhaseSucceeded
		setCondition(&abort.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondAbortProcessed,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonAbortSuccess.String(),
			Message: "upgrade abort processed; no Execute CR found, freeze window was still open",
		})
	}

	if err := r.Status().Update(ctx, &abort); err != nil {
		return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeAbortReconciler) anyExecuteExists(ctx context.Context, namespace, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing NetworkUpgradeExecute"), ReasonListFailed),
			"Confirm the operator has list permission on networkupgradeexecutes.operator.solo.hiero.org")
	}
	return len(list.Items) > 0, nil
}

func (r *NetworkUpgradeFreezeAbortReconciler) eventf(ctx context.Context, obj *operatorv1alpha1.NetworkUpgradeFreezeAbort, eventtype string, reason errx.Reason, format string, args ...any) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "FreezeAbort", format, args...)
}

func RegisterFreezeAbortIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradeFreezeAbort{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreezeAbort).Spec.OperationID}
		},
	)
}

func (r *NetworkUpgradeFreezeAbortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradefreezeabort")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}).
		Named("networkupgradefreezeabort").
		Complete(r)
}
