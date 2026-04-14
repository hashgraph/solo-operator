// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"

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

// HapiAppConfigMapSubReconciler manages the hapi-app ConfigMap for a ConsensusCapsule.
// It produces <name>-hapi-app-cm containing:
//   - log4j2.xml — from capsule.Spec.Log4j2XML
//   - settings.txt — rendered from orbit.Spec.Consensus.NetworkSettings
type HapiAppConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewHapiAppConfigMapSubReconciler creates a new HapiAppConfigMapSubReconciler.
func NewHapiAppConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *HapiAppConfigMapSubReconciler {
	return &HapiAppConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "HapiAppConfigMap"),
	}
}

// Reconcile ensures the hapi-app ConfigMap exists and is up to date.
func (r *HapiAppConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
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
			logger.Info("Creating hapi-app ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if needsConfigMapUpdate(existing.Data, desired.Data) {
		logger.V(1).Info("Updating hapi-app ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap constructs the desired hapi-app ConfigMap.
func (r *HapiAppConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) *corev1.ConfigMap {
	data := map[string]string{
		"log4j2.xml":   capsule.Spec.Log4j2XML,
		"settings.txt": RenderSettingsTxt(orbit.Spec.Consensus.NetworkSettings),
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HapiAppConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: data,
	}
}

// fetchOrbit fetches the Orbit CR by name. Returns an error when the orbit does not exist
// so the reconciler requeues and the ConfigMap is only created once the orbit is ready.
func fetchOrbit(ctx context.Context, c client.Client, orbitName string) (*operatorv1alpha1.Orbit, error) {
	if orbitName == "" {
		return nil, fmt.Errorf("capsule has no orbit name set")
	}
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("orbit %q not found: will retry once Orbit is created", orbitName)
		}
		return nil, err
	}
	return orbit, nil
}

// needsConfigMapUpdate returns true if the ConfigMap data has changed.
func needsConfigMapUpdate(existing, desired map[string]string) bool {
	if len(existing) != len(desired) {
		return true
	}
	for k, v := range desired {
		if existing[k] != v {
			return true
		}
	}
	return false
}
