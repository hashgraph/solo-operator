// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"context"
	"fmt"

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
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ApplicationProperties]
	Recorder events.EventRecorder
}

func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ApplicationProperties](c, scheme, "ConfigMap"),
		Recorder:          recorder,
	}
}

func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, obj *operatorv1alpha1.ApplicationProperties) (ctrl.Result, error) {
	if err := ValidateContent(obj.Spec.Content); err != nil {
		return r.reject(obj, "InvalidContent", err.Error())
	}

	ledgerID, _ := ParseLedgerID(obj.Spec.Content)

	if obj.Spec.Orbit != "" && ledgerID != "" {
		orbit := &operatorv1alpha1.Orbit{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: obj.Spec.Orbit}, orbit); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("orbit %q not found: will retry once Orbit is created", obj.Spec.Orbit)
			}
			return ctrl.Result{}, err
		}
		if orbitLedgerID := orbitLedgerID(orbit); orbitLedgerID != "" && ledgerID != orbitLedgerID {
			return r.reject(obj, "LedgerIDMismatch", fmt.Sprintf("spec.content ledger.id %q does not match Orbit %q ledger.id %q", ledgerID, obj.Spec.Orbit, orbitLedgerID))
		}
	}

	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ApplicationPropertiesConditionTypeValid,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: obj.Generation,
		Reason:             "ContentValid",
		Message:            "spec.content is valid application.properties",
	})
	obj.Status.LedgerID = ledgerID

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

func (r *ConfigMapSubReconciler) reject(obj *operatorv1alpha1.ApplicationProperties, reason, msg string) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ApplicationPropertiesConditionTypeValid,
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

func (r *ConfigMapSubReconciler) buildConfigMap(obj *operatorv1alpha1.ApplicationProperties) *corev1.ConfigMap {
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

func orbitLedgerID(orbit *operatorv1alpha1.Orbit) string {
	return orbit.Spec.Consensus.Genesis.AddressBook.LedgerId
}
