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
	"github.com/hashgraph/solo-operator/internal/resources/nodeproperties"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const NodePropertiesFinalizer = "operator.solo.hedera.com/nodeproperties-finalizer"

type NodePropertiesReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.NodePropertiesConfig]
}

func NewNodePropertiesReconciler(c client.Client, scheme *runtime.Scheme) *NodePropertiesReconciler {
	return &NodePropertiesReconciler{
		Client: c,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodeproperties,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodeproperties/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodeproperties/finalizers,verbs=update

func (r *NodePropertiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting NodeProperties reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished NodeProperties reconciliation", "namespace", req.Namespace, "name", req.Name)

	obj := &operatorv1alpha1.NodePropertiesConfig{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !obj.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(obj, NodePropertiesFinalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, NodePropertiesFinalizer) {
		controllerutil.AddFinalizer(obj, NodePropertiesFinalizer)
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

	r.event(obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "NodeProperties reconciled")
	return ctrl.Result{}, nil
}

func (r *NodePropertiesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.NodePropertiesConfig) error {
	obj.Status.ObservedGeneration = obj.Generation
	// status.configMapName is set unconditionally even when Valid=False (no ConfigMap was created).
	// This is intentional: operators and users can always resolve the expected ConfigMap name from
	// the CR name, and the Valid condition already signals whether the ConfigMap actually exists.
	obj.Status.ConfigMapName = nodeproperties.ConfigMapName(obj)
	return r.Status().Update(ctx, obj)
}

func (r *NodePropertiesReconciler) event(obj *operatorv1alpha1.NodePropertiesConfig, eventtype, reason, msg string) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "NodeProperties", "%s", msg)
	}
}

func (r *NodePropertiesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("nodeproperties")
	}
	if r.subreconciler == nil {
		r.subreconciler = nodeproperties.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NodePropertiesConfig{}).
		Owns(&corev1.ConfigMap{}).
		Named("nodeproperties").
		Complete(r)
}
