// SPDX-License-Identifier: Apache-2.0

package envoy

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

// ConfigMapSubReconciler manages the ConfigMap for an EnvoyProxy
type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

// NewConfigMapSubReconciler creates a new ConfigMapSubReconciler
func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "ConfigMap"),
	}
}

// Reconcile ensures the config map exists and is up to date
func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired, err := r.buildConfigMap(envoy)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy config map", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update data if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating Envoy config map", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap builds the desired config map
func (r *ConfigMapSubReconciler) buildConfigMap(envoy *operatorv1alpha1.EnvoyProxy) (*corev1.ConfigMap, error) {
	files := envoy.Spec.Files
	if len(files) == 0 {
		var err error
		files, err = DefaultEnvoyConfig(envoy)
		if err != nil {
			return nil, err
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(envoy),
			Namespace: envoy.Namespace,
			Labels:    CommonLabels(envoy),
		},
		Data: files,
	}, nil
}

// needsUpdate checks if the config map needs to be updated
func (r *ConfigMapSubReconciler) needsUpdate(existing, desired *corev1.ConfigMap) bool {
	if len(existing.Data) != len(desired.Data) {
		return true
	}
	for k, v := range desired.Data {
		if existing.Data[k] != v {
			return true
		}
	}
	return false
}
