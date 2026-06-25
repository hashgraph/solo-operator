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
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/simplefeesschedules"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type SimpleFeesSchedulesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.SimpleFeesSchedules]
}

func NewSimpleFeesSchedulesReconciler(c client.Client, scheme *runtime.Scheme) *SimpleFeesSchedulesReconciler {
	return &SimpleFeesSchedulesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=simplefeesschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=simplefeesschedules/status,verbs=get;update;patch

func (r *SimpleFeesSchedulesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting SimpleFeesSchedules reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished SimpleFeesSchedules reconciliation", "namespace", req.Namespace, "name", req.Name)

	obj := &operatorv1alpha1.SimpleFeesSchedules{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	result, err := r.subreconciler.Reconcile(ctx, obj)
	if err != nil {
		r.event(obj, corev1.EventTypeWarning, operatorv1alpha1.EventConfigCRFailed, err.Error())
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	if err := r.updateStatus(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	r.event(obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "SimpleFeesSchedules reconciled")
	return ctrl.Result{}, nil
}

func (r *SimpleFeesSchedulesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.SimpleFeesSchedules) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = simplefeesschedules.ConfigMapName(obj)
	return r.Status().Update(ctx, obj)
}

func (r *SimpleFeesSchedulesReconciler) event(obj *operatorv1alpha1.SimpleFeesSchedules, eventtype, reason, msg string) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "SimpleFeesSchedules", "%s", msg)
	}
}

func (r *SimpleFeesSchedulesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("simplefeesschedules")
	}
	if r.subreconciler == nil {
		r.subreconciler = simplefeesschedules.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.SimpleFeesSchedules{}).
		Owns(&corev1.ConfigMap{}).
		Named("simplefeesschedules").
		Complete(r)
}
