// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	corev1 "k8s.io/api/core/v1"
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

// ServiceAccountSubReconciler manages the ServiceAccount for a ConsensusCapsule
type ServiceAccountSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewServiceAccountSubReconciler creates a new ServiceAccountSubReconciler
func NewServiceAccountSubReconciler(c client.Client, scheme *runtime.Scheme) *ServiceAccountSubReconciler {
	return &ServiceAccountSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "ServiceAccount"),
	}
}

// Reconcile ensures the ServiceAccount exists and is up to date.
func (r *ServiceAccountSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if saCfg := capsule.Spec.PodProperties.ServiceAccount; saCfg != nil && saCfg.Name != "" {
		return ctrl.Result{}, nil
	}

	desired := r.buildServiceAccount(capsule)

	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ServiceAccount{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating service account", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating service account", "name", desired.Name)
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountSubReconciler) buildServiceAccount(capsule *operatorv1alpha1.ConsensusCapsule) *corev1.ServiceAccount {
	labels := CommonLabels(capsule)
	var annotations map[string]string

	if saCfg := capsule.Spec.PodProperties.ServiceAccount; saCfg != nil {
		for k, v := range saCfg.Labels {
			labels[k] = v
		}
		if len(saCfg.Annotations) > 0 {
			annotations = make(map[string]string, len(saCfg.Annotations))
			for k, v := range saCfg.Annotations {
				annotations[k] = v
			}
		}
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceAccountName(capsule),
			Namespace:   capsule.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func (r *ServiceAccountSubReconciler) needsUpdate(existing, desired *corev1.ServiceAccount) bool {
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	for k, v := range desired.Annotations {
		if existing.Annotations[k] != v {
			return true
		}
	}
	return false
}
