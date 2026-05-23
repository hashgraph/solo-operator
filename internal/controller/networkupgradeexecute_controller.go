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

// NetworkUpgradeExecuteReconciler is a no-op skeleton; real logic lands in a
// follow-up PR.
type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewNetworkUpgradeExecuteReconciler constructs a NetworkUpgradeExecuteReconciler.
func NewNetworkUpgradeExecuteReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeExecuteReconciler {
	return &NetworkUpgradeExecuteReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/finalizers,verbs=update

// Reconcile is a no-op placeholder.
func (r *NetworkUpgradeExecuteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("NetworkUpgradeExecute reconcile (no-op)",
		"namespace", req.Namespace, "name", req.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeExecuteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeExecute{}).
		Named("networkupgradeexecute").
		Complete(r)
}
