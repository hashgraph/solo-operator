// SPDX-License-Identifier: Apache-2.0

package log4j2config

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
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Log4j2Config]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Log4j2Config](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.Log4j2Config) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := ValidateContent(obj.Spec.Content); err != nil {
		logger.Error(err, "invalid log4j2 XML — fix spec.content to recover", "name", obj.Name, "namespace", obj.Namespace)
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ConditionTypeValid,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.Generation,
			Reason:             "InvalidXML",
			Message:            err.Error(),
		})
		if r.Recorder != nil {
			r.Recorder.Eventf(obj, nil, corev1.EventTypeWarning, "InvalidContent", "ConfigMap", "invalid log4j2 XML: %s", err.Error())
		}
		return ctrl.Result{}, nil
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "XMLValid",
		Message:            "spec.content is valid log4j2 XML",
	})

	desired := r.buildConfigMap(obj)

	if err := controllerutil.SetControllerReference(obj, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Log4j2 ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
			return ctrl.Result{}, r.Client.Create(ctx, desired)
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating Log4j2 ConfigMap", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return ctrl.Result{}, r.Client.Update(ctx, existing)
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.Log4j2Config) *corev1.ConfigMap {
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

func (r *ConfigMapSubReconciler) needsUpdate(existing, desired *corev1.ConfigMap) bool {
	return !maps.Equal(existing.Data, desired.Data) || !maps.Equal(existing.Labels, desired.Labels)
}
