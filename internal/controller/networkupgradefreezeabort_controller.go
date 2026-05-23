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

// NetworkUpgradeFreezeAbortReconciler is a no-op skeleton; real logic lands in
// a follow-up PR.
type NetworkUpgradeFreezeAbortReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewNetworkUpgradeFreezeAbortReconciler constructs a NetworkUpgradeFreezeAbortReconciler.
func NewNetworkUpgradeFreezeAbortReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeFreezeAbortReconciler {
	return &NetworkUpgradeFreezeAbortReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezeaborts/finalizers,verbs=update

// Reconcile is a no-op placeholder.
func (r *NetworkUpgradeFreezeAbortReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("NetworkUpgradeFreezeAbort reconcile (no-op)",
		"namespace", req.Namespace, "name", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeFreezeAbortReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreezeAbort{}).
		Named("networkupgradefreezeabort").
		Complete(r)
}
