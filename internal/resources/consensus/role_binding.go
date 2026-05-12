// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// RoleBindingSubReconciler manages the RoleBinding for a ConsensusCapsule
type RoleBindingSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewRoleBindingSubReconciler creates a new RoleBindingSubReconciler
func NewRoleBindingSubReconciler(c client.Client, scheme *runtime.Scheme) *RoleBindingSubReconciler {
	return &RoleBindingSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "RoleBinding"),
	}
}

// Reconcile ensures the role binding exists
func (r *RoleBindingSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildRoleBinding(capsule)

	// Set owner reference
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &rbacv1.RoleBinding{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating role binding", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update subjects and role ref if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating role binding", "name", desired.Name, "namespace", desired.Namespace)
		existing.Subjects = desired.Subjects
		existing.RoleRef = desired.RoleRef
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildRoleBinding builds the desired role binding
func (r *RoleBindingSubReconciler) buildRoleBinding(capsule *operatorv1alpha1.ConsensusCapsule) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleBindingName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     RoleName(capsule),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName(capsule),
				Namespace: capsule.Namespace,
			},
		},
	}
}

// needsUpdate checks if the role binding needs to be updated
func (r *RoleBindingSubReconciler) needsUpdate(existing, desired *rbacv1.RoleBinding) bool {
	// Check subjects
	if len(existing.Subjects) != len(desired.Subjects) {
		return true
	}
	for i, subject := range existing.Subjects {
		if subject.Kind != desired.Subjects[i].Kind ||
			subject.Name != desired.Subjects[i].Name ||
			subject.Namespace != desired.Subjects[i].Namespace {
			return true
		}
	}
	// Check role ref
	if existing.RoleRef.APIGroup != desired.RoleRef.APIGroup ||
		existing.RoleRef.Kind != desired.RoleRef.Kind ||
		existing.RoleRef.Name != desired.RoleRef.Name {
		return true
	}
	return false
}
