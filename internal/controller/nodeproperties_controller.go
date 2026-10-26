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
	"github.com/hashgraph/solo-operator/internal/resources/nodeproperties"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

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

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=nodepropertiesconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=nodepropertiesconfigs/status,verbs=get;update;patch

func (r *NodePropertiesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &operatorv1alpha1.NodePropertiesConfig{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "nodeproperties", "NodePropertiesConfig", obj)
	common.Debug(ctx, ReasonReconcileStart).Msg("Starting NodeProperties reconciliation")
	defer common.Debug(ctx, ReasonReconcileFinish).Msg("Finished NodeProperties reconciliation")

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

	r.event(ctx, obj, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled, "NodeProperties reconciled")
	return ctrl.Result{}, nil
}

func (r *NodePropertiesReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.NodePropertiesConfig) error {
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = nodeproperties.ConfigMapName(obj)
	return common.WarnOnConflict(ctx, r.Status().Update(ctx, obj))
}

func (r *NodePropertiesReconciler) event(ctx context.Context, obj *operatorv1alpha1.NodePropertiesConfig, eventtype string, reason errx.Reason, msg string) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "NodeProperties", "%s", msg)
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
