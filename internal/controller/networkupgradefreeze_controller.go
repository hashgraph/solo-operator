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

// NetworkUpgradeFreezeReconciler is a no-op skeleton; real logic lands in a
// follow-up PR.
type NetworkUpgradeFreezeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewNetworkUpgradeFreezeReconciler constructs a NetworkUpgradeFreezeReconciler.
func NewNetworkUpgradeFreezeReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeFreezeReconciler {
	return &NetworkUpgradeFreezeReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/finalizers,verbs=update

// Reconcile is a no-op placeholder.
func (r *NetworkUpgradeFreezeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("NetworkUpgradeFreeze reconcile (no-op)",
		"namespace", req.Namespace, "name", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeFreezeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreeze{}).
		Named("networkupgradefreeze").
		Complete(r)
}
