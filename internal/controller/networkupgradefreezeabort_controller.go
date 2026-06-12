// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

type NetworkUpgradeFreezeAbortReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

func NewNetworkUpgradeFreezeAbortReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeFreezeAbortReconciler {
	return &NetworkUpgradeFreezeAbortReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch

func (r *NetworkUpgradeFreezeAbortReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var abort operatorv1alpha1.NetworkUpgradeFreezeAbort
	if err := r.Get(ctx, req.NamespacedName, &abort); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if abort.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		abort.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if abort.Status.Phase == "" || abort.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		abort.Status.StartTime = &now
		abort.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &abort); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing status: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	execute, err := r.findExecute(ctx, abort.Namespace, abort.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}

	completionTime := metav1.Now()
	abort.Status.CompletionTime = &completionTime
	abort.Status.ObservedGeneration = abort.Generation

	if execute != nil {
		log.FromContext(ctx).Info("NetworkUpgradeExecute CR exists; abort arrived after freeze", "operationId", abort.Spec.OperationID)
		r.eventf(&abort, corev1.EventTypeWarning, operatorv1alpha1.EventAbortAfterFreeze,
			"NetworkUpgradeExecute CR already exists for operationId %q; abort has no effect after freeze", abort.Spec.OperationID)
		abort.Status.Phase = operatorv1alpha1.PhaseFailed
		setCondition(&abort.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondAbortProcessed,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonAbortAfterFreeze,
			Message: "abort received after NetworkUpgradeExecute was created; freeze window already closed",
		})
	} else {
		r.eventf(&abort, corev1.EventTypeNormal, operatorv1alpha1.EventAbortProcessed,
			"abort processed for operationId %q; upgrade cancelled before freeze", abort.Spec.OperationID)
		abort.Status.Phase = operatorv1alpha1.PhaseSucceeded
		setCondition(&abort.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondAbortProcessed,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonAbortSuccess,
			Message: "upgrade abort processed; no Execute CR found, freeze window was still open",
		})
	}

	if err := r.Status().Update(ctx, &abort); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeAbortReconciler) findExecute(ctx context.Context, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradeExecute, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return nil, fmt.Errorf("listing NetworkUpgradeExecute: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradeExecute for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *NetworkUpgradeFreezeAbortReconciler) eventf(obj *operatorv1alpha1.NetworkUpgradeFreezeAbort, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "FreezeAbort", format, args...)
	}
}

func (r *NetworkUpgradeFreezeAbortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradefreezeabort")
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradeExecute{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeExecute).Spec.OperationID}
		},
	); err != nil {
		return fmt.Errorf("registering operationId index on NetworkUpgradeExecute: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}).
		Named("networkupgradefreezeabort").
		Complete(r)
}
