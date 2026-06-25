// SPDX-License-Identifier: Apache-2.0

package simplefeesschedules

import (
	"context"
	"maps"

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
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.SimpleFeesSchedules]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.SimpleFeesSchedules](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.SimpleFeesSchedules) (ctrl.Result, error) {
	if err := ValidateContent(obj.Spec.Content); err != nil {
		return r.reject(obj, "InvalidContent", err.Error())
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.SimpleFeesSchedulesConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "ContentValid",
		Message:            "spec.content is valid simpleFeesSchedules.json",
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

	if needsUpdate(existing, desired) {
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

func needsUpdate(existing, desired *corev1.ConfigMap) bool {
	return !maps.Equal(existing.Data, desired.Data) ||
		!maps.Equal(existing.Labels, desired.Labels) ||
		!sameController(existing, desired)
}

// sameController reports whether existing and desired share the same controller owner
// reference by UID. A missing controller ref on either side counts as a difference.
func sameController(existing, desired *corev1.ConfigMap) bool {
	ec := metav1.GetControllerOf(existing)
	dc := metav1.GetControllerOf(desired)
	if ec == nil || dc == nil {
		return ec == nil && dc == nil
	}
	return ec.UID == dc.UID
}

func (r *ConfigMapSubReconciler) reject(obj *operatorv1alpha1.SimpleFeesSchedules, reason, msg string) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.SimpleFeesSchedulesConditionTypeValid,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: obj.Generation,
		Reason:             reason,
		Message:            msg,
	})
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, corev1.EventTypeWarning, reason, "ConfigMap", "%s", msg)
	}
	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.SimpleFeesSchedules) *corev1.ConfigMap {
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
