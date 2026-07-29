// SPDX-License-Identifier: Apache-2.0

package applicationoverrideproperties

import (
	"context"

	"github.com/automa-saga/errx"
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
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ApplicationOverrideProperties]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ApplicationOverrideProperties](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.ApplicationOverrideProperties) (ctrl.Result, error) {
	if err := common.ValidateJavaProperties(obj.Spec.Content, "application-override.properties"); err != nil {
		return r.reject(ctx, obj, "InvalidContent", err.Error())
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ApplicationOverridePropertiesConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "ContentValid",
		Message:            "spec.content is valid application-override.properties",
	})

	desired := r.buildConfigMap(obj)
	if err := controllerutil.SetControllerReference(obj, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.Client.Create(ctx, desired)
		}
		return ctrl.Result{}, err
	}

	if common.ConfigMapNeedsUpdate(existing, desired) {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		// Re-apply owner reference on update so a pre-existing CM without a controller
		// reference gets cascade-GC wired up correctly.
		if err := controllerutil.SetControllerReference(obj, existing, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.Client.Update(ctx, existing)
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) reject(ctx context.Context, obj *operatorv1alpha1.ApplicationOverrideProperties, reason, msg string) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ApplicationOverridePropertiesConditionTypeValid,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: obj.Generation,
		Reason:             reason,
		Message:            msg,
	})
	common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, errx.Reason(reason), "ConfigMap", "%s", msg)
	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.ApplicationOverrideProperties) *corev1.ConfigMap {
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
