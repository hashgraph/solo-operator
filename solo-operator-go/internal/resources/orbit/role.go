// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// RoleSubReconciler manages the consensus capsule role for an Orbit
type RoleSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Orbit]
}

// NewRoleSubReconciler creates a new RoleSubReconciler
func NewRoleSubReconciler(c client.Client, scheme *runtime.Scheme) *RoleSubReconciler {
	return &RoleSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Orbit](c, scheme, "ConsensusCapsuleRole"),
	}
}

// Reconcile ensures the consensus capsule role exists
func (r *RoleSubReconciler) Reconcile(ctx context.Context, orbit *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildRole(orbit)

	// Check if exists
	existing := &rbacv1.Role{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating consensus capsule role", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update rules if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating consensus capsule role", "name", desired.Name, "namespace", desired.Namespace)
		existing.Rules = desired.Rules
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildRole builds the desired role
func (r *RoleSubReconciler) buildRole(orbit *operatorv1alpha1.Orbit) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConsensusCapsuleRoleName,
			Namespace: orbit.Name,
			Labels:    StandardLabels(orbit),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

// needsUpdate checks if the role needs to be updated
func (r *RoleSubReconciler) needsUpdate(existing, desired *rbacv1.Role) bool {
	// Check rules equality
	if len(existing.Rules) != len(desired.Rules) {
		return true
	}
	for i, rule := range desired.Rules {
		if !rulesEqual(existing.Rules[i], rule) {
			return true
		}
	}
	// Check labels
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}

// rulesEqual checks if two policy rules are equal
func rulesEqual(a, b rbacv1.PolicyRule) bool {
	if !stringSliceEqual(a.APIGroups, b.APIGroups) {
		return false
	}
	if !stringSliceEqual(a.Resources, b.Resources) {
		return false
	}
	if !stringSliceEqual(a.Verbs, b.Verbs) {
		return false
	}
	return true
}

// stringSliceEqual checks if two string slices are equal
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
