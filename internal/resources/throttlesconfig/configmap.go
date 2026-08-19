// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import (
	"context"
	"fmt"
	"strings"

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
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ThrottlesConfig]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ThrottlesConfig](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.ThrottlesConfig) (ctrl.Result, error) {
	if err := common.ValidateJSON(obj.Spec.Content, "throttles.json"); err != nil {
		return r.reject(ctx, obj, "InvalidContent", err.Error())
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ThrottlesConfigConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "ContentValid",
		Message:            "spec.content is valid throttles.json",
	})

	r.checkOperationCoverage(ctx, obj)

	desired := r.buildConfigMap(obj)
	if err := controllerutil.SetControllerReference(obj, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.StampOwnerCreatedAt(desired, obj)
			return ctrl.Result{}, r.Client.Create(ctx, desired)
		}

		return ctrl.Result{}, err
	}

	callerUID := obj.GetUID()
	callerGVK := operatorv1alpha1.GroupVersion.WithKind("ThrottlesConfig")
	if conflict := common.ConfigMapOwnerConflict(existing, callerUID, callerGVK); conflict != "" {
		return r.reject(ctx, obj, "OwnershipConflict", conflict)
	}

	if common.ConfigMapSuperseded(existing, obj) {
		ownerRef := metav1.GetControllerOf(existing)
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ThrottlesConfigConditionTypeValid,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: obj.Generation,
			Reason:             common.ConditionReasonSuperseded,
			Message:            fmt.Sprintf("ConfigMap %s is owned by a newer CR %s (uid %s)", desired.Name, ownerRef.Name, ownerRef.UID),
		})
		return ctrl.Result{}, nil
	}

	if common.ConfigMapNeedsUpdate(existing, desired) {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		common.ClearControllerRef(existing)
		// Re-apply owner reference on update so a pre-existing CM without a controller
		// reference gets cascade-GC wired up correctly.
		if err := controllerutil.SetControllerReference(obj, existing, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		common.StampOwnerCreatedAt(existing, obj)
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) reject(ctx context.Context, obj *operatorv1alpha1.ThrottlesConfig, reason, msg string) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ThrottlesConfigConditionTypeValid,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: obj.Generation,
		Reason:             reason,
		Message:            msg,
	})
	common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, errx.Reason(reason), "ConfigMap", "%s", msg)
	return ctrl.Result{}, nil
}

func (r *ConfigMapSubReconciler) checkOperationCoverage(ctx context.Context, obj *operatorv1alpha1.ThrottlesConfig) {
	missing, err := ValidateThrottleOps(obj.Spec.Content)
	if err != nil {
		msg := fmt.Sprintf("failed to parse throttle operations: %v", err)
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ThrottlesConfigConditionTypeOpsCoverage,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.Generation,
			Reason:             string(common.ReasonInvalidContent),
			Message:            msg,
		})
		common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, common.ReasonInvalidContent, "ConfigMap", "%s", msg)
		return
	}

	if len(missing) > 0 {
		msg := fmt.Sprintf("throttle buckets are missing required operations (network will deny them with BUSY): %s", strings.Join(missing, ", "))
		apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ThrottlesConfigConditionTypeOpsCoverage,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.Generation,
			Reason:             string(ReasonMissingOperations),
			Message:            msg,
		})
		common.Eventf(ctx, r.Recorder, obj, corev1.EventTypeWarning, ReasonMissingOperations, "ConfigMap", "%s", msg)
		return
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ThrottlesConfigConditionTypeOpsCoverage,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             string(ReasonFullCoverage),
		Message:            "all required operations are covered by throttle buckets",
	})
}

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.ThrottlesConfig) *corev1.ConfigMap {
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
