// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// ConfigMapSubReconciler manages the ConfigMap for an HAProxyCapsule
type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HAProxyCapsule]
}

// NewConfigMapSubReconciler creates a new ConfigMapSubReconciler
func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HAProxyCapsule](c, scheme, "ConfigMap"),
	}
}

// Reconcile ensures the config map exists and is up to date
func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	desired, err := r.buildConfigMap(haproxy)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonConfigMapCreated).Str("configMap", desired.Name).Msg("Creating HAProxy config map")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update data if needed
	if r.needsUpdate(existing, desired) {
		common.Debug(ctx, common.ReasonConfigMapUpdated).Str("configMap", desired.Name).Msg("Updating HAProxy config map")
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

// buildConfigMap builds the desired config map
func (r *ConfigMapSubReconciler) buildConfigMap(haproxy *operatorv1alpha1.HAProxyCapsule) (*corev1.ConfigMap, error) {
	cfg, err := HAProxyConfig(haproxy)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(haproxy),
			Namespace: haproxy.Namespace,
			Labels:    CommonLabels(haproxy),
		},
		Data: map[string]string{
			ConfigFileName: cfg,
		},
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
