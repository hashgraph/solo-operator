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

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// CheckpointSubReconciler creates [capsule]-checkpoint once and never overwrites it.
// The controller patches it to mark genesis done; the init container reads it at pod start.
type CheckpointSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

func NewCheckpointSubReconciler(c client.Client, scheme *runtime.Scheme) *CheckpointSubReconciler {
	return &CheckpointSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "Checkpoint"),
	}
}

func (r *CheckpointSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	desired := r.buildCheckpoint(capsule)
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonConfigMapCreated).Str("configMap", desired.Name).Msg("Creating checkpoint ConfigMap")
			return ctrl.Result{}, r.Client.Create(ctx, desired)
		}
		return ctrl.Result{}, err
	}

	// Never overwrite — state is managed by the controller.
	return ctrl.Result{}, nil
}

func (r *CheckpointSubReconciler) buildCheckpoint(capsule *operatorv1alpha1.ConsensusCapsule) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CheckpointConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: map[string]string{
			CheckpointKeyGenesisInitialized: CheckpointValueFalse,
		},
	}
}
