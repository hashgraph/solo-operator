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
	"github.com/hashgraph/solo-operator/internal/resources/bootstrapproperties"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type BootstrapPropertiesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.BootstrapProperties]
}

func NewBootstrapPropertiesReconciler(c client.Client, scheme *runtime.Scheme) *BootstrapPropertiesReconciler {
	return &BootstrapPropertiesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=bootstrapproperties,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=bootstrapproperties/status,verbs=get;update;patch

func (r *BootstrapPropertiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &operatorv1alpha1.BootstrapProperties{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "bootstrapproperties", "BootstrapProperties", obj)
	common.Info(ctx, ReasonReconcileStart).Msg("Starting BootstrapProperties reconciliation")
	defer common.Info(ctx, ReasonReconcileFinish).Msg("Finished BootstrapProperties reconciliation")

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

	r.event(ctx, obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "BootstrapProperties reconciled")
	return ctrl.Result{}, nil
}

func (r *BootstrapPropertiesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.BootstrapProperties) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = bootstrapproperties.ConfigMapName(obj)
	return r.Status().Update(ctx, obj)
}

func (r *BootstrapPropertiesReconciler) event(ctx context.Context, obj *operatorv1alpha1.BootstrapProperties, eventtype string, reason errx.Reason, msg string) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "BootstrapProperties", "%s", msg)
}

func (r *BootstrapPropertiesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("bootstrapproperties")
	}
	if r.subreconciler == nil {
		r.subreconciler = bootstrapproperties.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.BootstrapProperties{}).
		Owns(&corev1.ConfigMap{}).
		Named("bootstrapproperties").
		Complete(r)
}
