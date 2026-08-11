// SPDX-License-Identifier: Apache-2.0

package nodesettings

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.NodeSettings]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.NodeSettings](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.NodeSettings) (ctrl.Result, error) {
	if err := ValidateContent(obj.Spec.Content); err != nil {
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.NodeSettingsConditionTypeValid,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.Generation,
			Reason:             "InvalidContent",
			Message:            err.Error(),
		})
		common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, common.ReasonInvalidContent, "ConfigMap", "invalid settings.txt: %s", err.Error())
		return ctrl.Result{}, nil
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.NodeSettingsConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "ContentValid",
		Message:            "spec.content is valid settings.txt",
	})

	desired := r.buildConfigMap(obj)

	if err := controllerutil.SetControllerReference(obj, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonConfigMapCreated).Str("configMap", desired.Name).Msg("Creating NodeSettings ConfigMap")
			return ctrl.Result{}, r.Client.Create(ctx, desired)
		}
		return ctrl.Result{}, err
	}

	if conflict := common.ConfigMapOwnerConflict(existing, obj); conflict != "" {
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.NodeSettingsConditionTypeValid,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.Generation,
			Reason:             "OwnershipConflict",
			Message:            conflict,
		})
		common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, common.ReasonOwnershipConflict, "ConfigMap", "%s", conflict)
		return ctrl.Result{}, nil
	}

	if common.ConfigMapNeedsUpdate(existing, desired) {
		common.Debug(ctx, common.ReasonConfigMapUpdated).Str("configMap", desired.Name).Msg("Updating NodeSettings ConfigMap")
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		common.ClearControllerRef(existing)
		// Re-apply owner reference on update so a pre-existing CM without a controller
		// reference gets cascade-GC wired up correctly.
		if err := controllerutil.SetControllerReference(obj, existing, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.NodeSettings) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(obj),
			Namespace: obj.Namespace,
			Labels:    CommonLabels(obj),
		},
		Data: map[string]string{
			ConfigFileName: obj.Spec.Content,
		},
	}
}
