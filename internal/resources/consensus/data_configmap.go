// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

func fetchOrbit(ctx context.Context, c client.Client, orbitName string) (*operatorv1alpha1.Orbit, error) {
	if orbitName == "" {
		return nil, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("capsule has no orbit name set"), ReasonOrbitNameMissing),
			"Set spec.orbit on the ConsensusCapsule",
		)
	}
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errx.WithHints(
				errx.WithReason(errOrbitNotFound.New("orbit %q not found: will retry once Orbit is created", orbitName), ReasonOrbitNotFound),
				"Create the referenced Orbit resource",
			)
		}
		return nil, err
	}
	return orbit, nil
}

type DataConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

func NewDataConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *DataConfigMapSubReconciler {
	return &DataConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "DataConfigMap"),
	}
}

func (r *DataConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
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
			common.Info(ctx, common.ReasonConfigMapCreated).Str("configMap", desired.Name).Msg("Creating genesis-properties ConfigMap")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !maps.Equal(existing.Data, desired.Data) {
		common.Debug(ctx, common.ReasonConfigMapUpdated).Str("configMap", desired.Name).Msg("Updating genesis-properties ConfigMap")
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func MergeApplicationProperties(overrides map[string]string) string {
	merged := DefaultApplicationProperties()
	for k, v := range overrides {
		merged[k] = v
	}
	return RenderProperties(merged)
}

func (r *DataConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, _ *operatorv1alpha1.Orbit) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DataConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: map[string]string{},
	}
}
