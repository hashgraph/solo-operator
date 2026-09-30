// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/apipermissionproperties"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type ApiPermissionPropertiesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.ApiPermissionProperties]
}

func NewApiPermissionPropertiesReconciler(c client.Client, scheme *runtime.Scheme) *ApiPermissionPropertiesReconciler {
	return &ApiPermissionPropertiesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=apipermissionproperties,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=apipermissionproperties/status,verbs=get;update;patch

func (r *ApiPermissionPropertiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &operatorv1alpha1.ApiPermissionProperties{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "apipermissionproperties", "ApiPermissionProperties", obj)
	common.Debug(ctx, ReasonReconcileStart).Msg("Starting ApiPermissionProperties reconciliation")
	defer common.Debug(ctx, ReasonReconcileFinish).Msg("Finished ApiPermissionProperties reconciliation")

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

	r.event(ctx, obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "ApiPermissionProperties reconciled")
	return ctrl.Result{}, nil
}

func (r *ApiPermissionPropertiesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.ApiPermissionProperties) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = apipermissionproperties.ConfigMapName(obj)
	return common.WarnOnConflict(ctx, r.Status().Update(ctx, obj))
}

func (r *ApiPermissionPropertiesReconciler) event(ctx context.Context, obj *operatorv1alpha1.ApiPermissionProperties, eventtype string, reason errx.Reason, msg string) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "ApiPermissionProperties", "%s", msg)
}

func (r *ApiPermissionPropertiesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("apipermissionproperties")
	}
	if r.subreconciler == nil {
		r.subreconciler = apipermissionproperties.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ApiPermissionProperties{}).
		Owns(&corev1.ConfigMap{}).
		Named("apipermissionproperties").
		Complete(r)
}
