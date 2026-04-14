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

// DataConfigMapSubReconciler manages the data-config ConfigMap for a ConsensusCapsule.
// It produces <name>-data-config-cm containing:
//   - application.properties — rendered from orbit.Spec.Consensus.ApplicationConfiguration
//   - api-permission.properties — rendered from orbit.Spec.Consensus.PermissionConfiguration
type DataConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewDataConfigMapSubReconciler creates a new DataConfigMapSubReconciler.
func NewDataConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *DataConfigMapSubReconciler {
	return &DataConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "DataConfigMap"),
	}
}

// Reconcile ensures the data-config ConfigMap exists and is up to date.
func (r *DataConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	orbit, err := fetchOrbit(ctx, r.Client, capsule.Spec.Orbit)
	if err != nil {
		return ctrl.Result{}, err
	}

	desired := r.buildConfigMap(capsule, orbit)

	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating data-config ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if needsConfigMapUpdate(existing.Data, desired.Data) {
		logger.V(1).Info("Updating data-config ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap constructs the desired data-config ConfigMap.
func (r *DataConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) *corev1.ConfigMap {
	data := map[string]string{
		"application.properties":    RenderProperties(orbit.Spec.Consensus.ApplicationConfiguration),
		"api-permission.properties": RenderProperties(orbit.Spec.Consensus.PermissionConfiguration),
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DataConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: data,
	}
}
