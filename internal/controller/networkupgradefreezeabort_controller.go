// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/upgrade"
)

// NetworkUpgradeFreezeAbortReconciler records a freeze abort, guarding against
// aborts that arrive after the Execute CR has already been created.
type NetworkUpgradeFreezeAbortReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// NewNetworkUpgradeFreezeAbortReconciler constructs a NetworkUpgradeFreezeAbortReconciler.
func NewNetworkUpgradeFreezeAbortReconciler(c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) *NetworkUpgradeFreezeAbortReconciler {
	return &NetworkUpgradeFreezeAbortReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: recorder,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for NetworkUpgradeFreezeAbort.
func (r *NetworkUpgradeFreezeAbortReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cr := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cr.Status.Phase == operatorv1alpha1.PhaseSucceeded || cr.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if cr.Status.StartTime == nil {
		now := metav1.Now()
		cr.Status.StartTime = &now
		cr.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	exec, err := upgrade.FindExecuteForOperation(ctx, r.Client, cr.Namespace, cr.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}

	if exec != nil {
		return r.markTooLate(ctx, cr)
	}
	return r.markProcessed(ctx, cr)
}

func (r *NetworkUpgradeFreezeAbortReconciler) markProcessed(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradeFreezeAbort,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondAbortProcessed, metav1.ConditionTrue,
		operatorv1alpha1.ReasonAbortSuccess,
		"freeze abort processed; prior upgrade steps abandoned", gen)

	if r.Recorder != nil {
		r.Recorder.Event(cr, corev1.EventTypeNormal,
			operatorv1alpha1.EventAbortProcessed,
			"freeze abort processed before Execute CR was created")
	}

	now := metav1.Now()
	cr.Status.CompletionTime = &now
	cr.Status.Phase = operatorv1alpha1.PhaseSucceeded
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeAbortReconciler) markTooLate(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradeFreezeAbort,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondAbortProcessed, metav1.ConditionFalse,
		operatorv1alpha1.ReasonAbortAfterFreeze,
		"NetworkUpgradeExecute already exists; abort cannot reverse an in-progress execute", gen)

	if r.Recorder != nil {
		r.Recorder.Event(cr, corev1.EventTypeWarning,
			operatorv1alpha1.EventAbortAfterFreeze,
			"freeze abort received after NetworkUpgradeExecute was created; no action taken")
	}

	now := metav1.Now()
	cr.Status.CompletionTime = &now
	cr.Status.Phase = operatorv1alpha1.PhaseFailed
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeFreezeAbortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}).
		Named("networkupgradefreezeabort").
		Complete(r)
}
