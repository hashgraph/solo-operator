// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// NetworkUpgradePrepareReconciler is a no-op skeleton; real logic lands in a
// follow-up PR. It registers the CRD with the manager so the cluster can
// accept NetworkUpgradePrepare objects today.
type NetworkUpgradePrepareReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewNetworkUpgradePrepareReconciler constructs a NetworkUpgradePrepareReconciler.
func NewNetworkUpgradePrepareReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradePrepareReconciler {
	return &NetworkUpgradePrepareReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/finalizers,verbs=update

// Reconcile is a no-op placeholder.
func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("NetworkUpgradePrepare reconcile (no-op)",
		"namespace", req.Namespace, "name", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradePrepareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradePrepare{}).
		Named("networkupgradeprepare").
		Complete(r)
}
