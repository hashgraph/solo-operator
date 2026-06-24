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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/applicationproperties"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	ApplicationPropertiesFinalizer = "operator.solo.hedera.com/applicationproperties-finalizer"
)

type ApplicationPropertiesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.ApplicationProperties]
}

func NewApplicationPropertiesReconciler(c client.Client, scheme *runtime.Scheme) *ApplicationPropertiesReconciler {
	return &ApplicationPropertiesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=applicationproperties,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=applicationproperties/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=applicationproperties/finalizers,verbs=update

func (r *ApplicationPropertiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting ApplicationProperties reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished ApplicationProperties reconciliation", "namespace", req.Namespace, "name", req.Name)

	obj := &operatorv1alpha1.ApplicationProperties{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !obj.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(obj, ApplicationPropertiesFinalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, ApplicationPropertiesFinalizer) {
		controllerutil.AddFinalizer(obj, ApplicationPropertiesFinalizer)
		if err := r.Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
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

	r.event(obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "ApplicationProperties reconciled")
	return ctrl.Result{}, nil
}

func (r *ApplicationPropertiesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.ApplicationProperties) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = applicationproperties.ConfigMapName(obj)
	return r.Status().Update(ctx, obj)
}

func (r *ApplicationPropertiesReconciler) event(obj *operatorv1alpha1.ApplicationProperties, eventtype, reason, msg string) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "ApplicationProperties", "%s", msg)
	}
}

func (r *ApplicationPropertiesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("applicationproperties")
	}
	if r.subreconciler == nil {
		r.subreconciler = applicationproperties.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ApplicationProperties{}).
		Owns(&corev1.ConfigMap{}).
		Named("applicationproperties").
		Complete(r)
}
