// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// NamespaceSubReconciler manages the namespace for an Orbit
type NamespaceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Orbit]
}

// NewNamespaceSubReconciler creates a new NamespaceSubReconciler
func NewNamespaceSubReconciler(c client.Client, scheme *runtime.Scheme) *NamespaceSubReconciler {
	return &NamespaceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Orbit](c, scheme, "OrbitNamespace"),
	}
}

// Reconcile ensures the orbit namespace exists
func (r *NamespaceSubReconciler) Reconcile(ctx context.Context, orbit *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildNamespace(orbit)

	// Check if namespace exists
	existing := &corev1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: orbit.Name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating namespace", "name", orbit.Name)
			// Note: We don't set owner reference for cluster-scoped resources
			// The namespace lifecycle is managed by the Orbit finalizer
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update labels if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating namespace labels", "name", orbit.Name)
		if existing.Labels == nil {
			existing.Labels = make(map[string]string)
		}
		for k, v := range desired.Labels {
			existing.Labels[k] = v
		}
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildNamespace builds the desired namespace
func (r *NamespaceSubReconciler) buildNamespace(orbit *operatorv1alpha1.Orbit) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   orbit.Name,
			Labels: StandardLabels(orbit),
		},
	}
}

// needsUpdate checks if the namespace needs to be updated
func (r *NamespaceSubReconciler) needsUpdate(existing, desired *corev1.Namespace) bool {
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}
