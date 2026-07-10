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
	"github.com/hashgraph/solo-operator/internal/resources/log4j2config"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type Log4j2ConfigReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.Log4j2Config]
}

func NewLog4j2ConfigReconciler(c client.Client, scheme *runtime.Scheme) *Log4j2ConfigReconciler {
	return &Log4j2ConfigReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=log4j2configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=log4j2configs/status,verbs=get;update;patch

func (r *Log4j2ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting Log4j2Config reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished Log4j2Config reconciliation", "namespace", req.Namespace, "name", req.Name)

	obj := &operatorv1alpha1.Log4j2Config{}
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

	r.event(obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "Log4j2Config reconciled")
	return ctrl.Result{}, nil
}

func (r *Log4j2ConfigReconciler) event(obj *operatorv1alpha1.Log4j2Config, eventtype, reason, msg string) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "Log4j2Config", "%s", msg)
	}
}

func (r *Log4j2ConfigReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.Log4j2Config) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = log4j2config.ConfigMapName(obj)
	return r.Status().Update(ctx, obj)
}

func (r *Log4j2ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("log4j2config")
	}
	if r.subreconciler == nil {
		r.subreconciler = log4j2config.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Log4j2Config{}).
		Owns(&corev1.ConfigMap{}).
		Named("log4j2config").
		Complete(r)
}
