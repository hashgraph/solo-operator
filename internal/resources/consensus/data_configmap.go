// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"
	"maps"

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

<<<<<<< HEAD
||||||| d5b1da1
// DataConfigMapSubReconciler manages the data-config ConfigMap for a ConsensusCapsule.
// It produces <name>-data-config-cm containing:
//   - application.properties — rendered from orbit.Spec.Consensus.ApplicationConfiguration
//   - api-permission.properties — rendered from orbit.Spec.Consensus.PermissionConfiguration
//   - genesis.properties — rendered from orbit.Spec.Consensus.GenesisProperties (omitted when empty)
=======
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

// DataConfigMapSubReconciler manages the data-config ConfigMap for a ConsensusCapsule.
// It produces <name>-data-config-cm containing:
//   - application.properties — rendered from orbit.Spec.Consensus.ApplicationConfiguration
//   - api-permission.properties — rendered from orbit.Spec.Consensus.PermissionConfiguration
//   - genesis.properties — rendered from orbit.Spec.Consensus.GenesisProperties (omitted when empty)
>>>>>>> epica38_3_story2_nodesettings
type DataConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

func NewDataConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *DataConfigMapSubReconciler {
	return &DataConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "DataConfigMap"),
	}
}

func (r *DataConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	orbit, err := fetchOrbit(ctx, r.Client, capsule.Spec.Orbit)
	if err != nil {
		return ctrl.Result{}, err
	}

	appProps, err := r.resolveApplicationProperties(ctx, capsule, orbit)
	if err != nil {
		return ctrl.Result{}, err
	}

	desired := r.buildConfigMap(capsule, orbit, appProps)

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

	if !maps.Equal(existing.Data, desired.Data) {
		logger.V(1).Info("Updating data-config ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DataConfigMapSubReconciler) resolveApplicationProperties(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) (string, error) {
	if capsule.Spec.ApplicationPropertiesRef != "" {
		ref := &operatorv1alpha1.ApplicationProperties{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: capsule.Spec.ApplicationPropertiesRef, Namespace: capsule.Namespace}, ref); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("ApplicationProperties %q not found in namespace %q: will retry once it is created", capsule.Spec.ApplicationPropertiesRef, capsule.Namespace)
			}
			return "", err
		}
		return ref.Spec.Content, nil
	}
	return MergeApplicationProperties(orbit.Spec.Consensus.ApplicationConfiguration), nil
}

func MergeApplicationProperties(overrides map[string]string) string {
	merged := DefaultApplicationProperties()
	for k, v := range overrides {
		merged[k] = v
	}
	return RenderProperties(merged)
}

func (r *DataConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit, appProps string) *corev1.ConfigMap {
	data := map[string]string{
		"application.properties":    appProps,
		"api-permission.properties": RenderProperties(orbit.Spec.Consensus.PermissionConfiguration),
	}
	if len(orbit.Spec.Consensus.GenesisProperties) > 0 {
		data[GenesisPropertiesFileName] = RenderProperties(orbit.Spec.Consensus.GenesisProperties)
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
