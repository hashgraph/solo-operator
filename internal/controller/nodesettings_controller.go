// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/nodesettings"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	NodeSettingsFinalizer = "operator.solo.hedera.com/nodesettings-finalizer"
)

type NodeSettingsReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	subreconciler *subreconciler.Chain[*operatorv1alpha1.NodeSettings]
}

func NewNodeSettingsReconciler(c client.Client, scheme *runtime.Scheme) *NodeSettingsReconciler {
	return &NodeSettingsReconciler{
		Client:        c,
		Scheme:        scheme,
		subreconciler: nodesettings.NewSubReconcilers(c, scheme),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodesettings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodesettings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodesettings/finalizers,verbs=update

func (r *NodeSettingsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting NodeSettings reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished NodeSettings reconciliation", "namespace", req.Namespace, "name", req.Name)

	obj := &operatorv1alpha1.NodeSettings{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !obj.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(obj, NodeSettingsFinalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, NodeSettingsFinalizer) {
		controllerutil.AddFinalizer(obj, NodeSettingsFinalizer)
		if err := r.Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	result, err := r.subreconciler.Reconcile(ctx, obj)
	if err != nil {
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	if err := r.updateStatus(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NodeSettingsReconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.NodeSettings) error {
	desired := nodesettings.ConfigMapName(obj)
	if obj.Status.ObservedGeneration == obj.Generation && obj.Status.ConfigMapName == desired {
		return nil
	}
	obj.Status.ObservedGeneration = obj.Generation
	obj.Status.ConfigMapName = desired
	return r.Status().Update(ctx, obj)
}

func (r *NodeSettingsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NodeSettings{}).
		Owns(&corev1.ConfigMap{}).
		Named("nodesettings").
		Complete(r)
}
