// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/resources/feeschedules"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type FeeSchedulesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.FeeSchedules]
}

func NewFeeSchedulesReconciler(c client.Client, scheme *runtime.Scheme) *FeeSchedulesReconciler {
	return &FeeSchedulesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=feeschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=feeschedules/status,verbs=get;update;patch

func (r *FeeSchedulesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &operatorv1alpha1.FeeSchedules{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "feeschedules", "FeeSchedules", obj)
	common.Info(ctx, ReasonReconcileStart).Msg("Starting FeeSchedules reconciliation")
	defer common.Info(ctx, ReasonReconcileFinish).Msg("Finished FeeSchedules reconciliation")

	result, err := r.subreconciler.Reconcile(ctx, obj)
	if err != nil {
		r.event(ctx, obj, corev1.EventTypeWarning, operatorv1alpha1.EventConfigCRFailed, err.Error())
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	if err := r.updateStatus(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	r.event(ctx, obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "FeeSchedules reconciled")
	return ctrl.Result{}, nil
}

func (r *FeeSchedulesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.FeeSchedules) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = feeschedules.ConfigMapName(obj)
	return common.WarnOnConflict(ctx, r.Status().Update(ctx, obj))
}

func (r *FeeSchedulesReconciler) event(ctx context.Context, obj *operatorv1alpha1.FeeSchedules, eventtype string, reason errx.Reason, msg string) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "FeeSchedules", "%s", msg)
}

func (r *FeeSchedulesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("feeschedules")
	}
	if r.subreconciler == nil {
		r.subreconciler = feeschedules.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.FeeSchedules{}).
		Owns(&corev1.ConfigMap{}).
		Named("feeschedules").
		Complete(r)
}
