// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"encoding/json"
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
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// BlockNodesConfigMapSubReconciler manages the block-nodes ConfigMap for a ConsensusCapsule.
// It produces <name>-block-nodes-cm containing block-nodes.json and is only active when
// capsule.Spec.BlockNodes is non-nil.
type BlockNodesConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewBlockNodesConfigMapSubReconciler creates a new BlockNodesConfigMapSubReconciler.
func NewBlockNodesConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *BlockNodesConfigMapSubReconciler {
	return &BlockNodesConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "BlockNodesConfigMap"),
	}
}

// ShouldReconcile returns true only when block-node configuration is present.
func (r *BlockNodesConfigMapSubReconciler) ShouldReconcile(_ context.Context, capsule *operatorv1alpha1.ConsensusCapsule) bool {
	return capsule.Spec.BlockNodes != nil
}

// Reconcile ensures the block-nodes ConfigMap exists and is up to date.
func (r *BlockNodesConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	desired, err := r.buildConfigMap(capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonConfigMapCreated).Str("configMap", desired.Name).Msg("Creating block-nodes ConfigMap")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !maps.Equal(existing.Data, desired.Data) {
		common.Debug(ctx, common.ReasonConfigMapUpdated).Str("configMap", desired.Name).Msg("Updating block-nodes ConfigMap")
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := common.WarnOnConflict(ctx, r.Client.Update(ctx, existing)); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap marshals the BlockNodesConfig to JSON and wraps it in a ConfigMap.
func (r *BlockNodesConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule) (*corev1.ConfigMap, error) {
	jsonBytes, err := json.Marshal(capsule.Spec.BlockNodes)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BlockNodesConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: map[string]string{
			"block-nodes.json": string(jsonBytes),
		},
	}, nil
}
