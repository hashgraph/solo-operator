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

// PVCSubReconciler manages PersistentVolumeClaims for a ConsensusCapsule
type PVCSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewPVCSubReconciler creates a new PVCSubReconciler
func NewPVCSubReconciler(c client.Client, scheme *runtime.Scheme) *PVCSubReconciler {
	return &PVCSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "PersistentVolumeClaims"),
	}
}

// ShouldReconcile returns true if PVCs are configured
func (r *PVCSubReconciler) ShouldReconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) bool {
	return capsule.Spec.PersistentVolumeClaims != nil
}

// Reconcile ensures all configured PVCs exist
func (r *PVCSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	pvcConfig := capsule.Spec.PersistentVolumeClaims
	if pvcConfig == nil {
		return ctrl.Result{}, nil
	}

	// Create PVCs for each configured volume
	pvcSpecs := map[string]*corev1.PersistentVolumeClaimSpec{
		VolumeTypeLogs:    pvcConfig.Logs,
		VolumeTypeStats:   pvcConfig.Stats,
		VolumeTypeStates:  pvcConfig.States,
		VolumeTypeUpgrade: pvcConfig.Upgrade,
	}

	if pvcConfig.Streams != nil {
		pvcSpecs[VolumeTypeBlocks] = pvcConfig.Streams.Block
		pvcSpecs[VolumeTypeRecords] = pvcConfig.Streams.Record
		pvcSpecs[VolumeTypeEvents] = pvcConfig.Streams.Events
	}

	for volumeType, spec := range pvcSpecs {
		if spec != nil {
			if err := r.reconcilePVC(ctx, capsule, volumeType, spec); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePVC ensures a specific PVC exists
func (r *PVCSubReconciler) reconcilePVC(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, volumeType string, spec *corev1.PersistentVolumeClaimSpec) error {
	logger := log.FromContext(ctx)

	desired := r.buildPVC(capsule, volumeType, spec)

	// Set owner reference
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return err
	}

	// Check if exists
	existing := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PVC", "name", desired.Name, "namespace", desired.Namespace, "volumeType", volumeType)
			return r.Client.Create(ctx, desired)
		}
		return err
	}

	// PVCs are immutable after creation, so we don't update them
	return nil
}

// buildPVC builds the desired PVC
func (r *PVCSubReconciler) buildPVC(capsule *operatorv1alpha1.ConsensusCapsule, volumeType string, spec *corev1.PersistentVolumeClaimSpec) *corev1.PersistentVolumeClaim {
	labels := CommonLabels(capsule)
	labels["volume-type"] = volumeType

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCName(capsule, volumeType),
			Namespace: capsule.Namespace,
			Labels:    labels,
		},
		Spec: *spec,
	}
}

// GetPVCNames returns a list of all PVC names for a capsule
func GetPVCNames(capsule *operatorv1alpha1.ConsensusCapsule) []string {
	if capsule.Spec.PersistentVolumeClaims == nil {
		return nil
	}

	pvcConfig := capsule.Spec.PersistentVolumeClaims
	names := make([]string, 0)

	if pvcConfig.Logs != nil {
		names = append(names, PVCName(capsule, VolumeTypeLogs))
	}
	if pvcConfig.Stats != nil {
		names = append(names, PVCName(capsule, VolumeTypeStats))
	}
	if pvcConfig.States != nil {
		names = append(names, PVCName(capsule, VolumeTypeStates))
	}
	if pvcConfig.Upgrade != nil {
		names = append(names, PVCName(capsule, VolumeTypeUpgrade))
	}
	if pvcConfig.Streams != nil {
		if pvcConfig.Streams.Block != nil {
			names = append(names, PVCName(capsule, VolumeTypeBlocks))
		}
		if pvcConfig.Streams.Record != nil {
			names = append(names, PVCName(capsule, VolumeTypeRecords))
		}
		if pvcConfig.Streams.Events != nil {
			names = append(names, PVCName(capsule, VolumeTypeEvents))
		}
	}

	return names
}
