// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	HelmCapsuleFinalizer = "operator.solo.hedera.com/helmcapsule-finalizer"
	conditionReady       = "Ready"
)

type HelmCapsuleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	helmClient    helm.Client
	subreconciler *subreconciler.Chain[*operatorv1alpha1.HelmCapsule]
}

func NewHelmCapsuleReconciler(c client.Client, scheme *runtime.Scheme, helmClient helm.Client) *HelmCapsuleReconciler {
	return &HelmCapsuleReconciler{
		Client:        c,
		Scheme:        scheme,
		helmClient:    helmClient,
		subreconciler: helm.NewSubReconcilers(c, scheme, helmClient),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch

func (r *HelmCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling HelmCapsule", "namespace", req.Namespace, "name", req.Name)

	hc := &operatorv1alpha1.HelmCapsule{}
	if err := r.Get(ctx, req.NamespacedName, hc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !hc.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, hc)
	}

	if !controllerutil.ContainsFinalizer(hc, HelmCapsuleFinalizer) {
		controllerutil.AddFinalizer(hc, HelmCapsuleFinalizer)
		if err := r.Update(ctx, hc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	result, err := r.subreconciler.Reconcile(ctx, hc)
	if err != nil {
		r.recordFailure(ctx, hc, err)
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	return ctrl.Result{}, r.updateStatus(ctx, hc)
}

func (r *HelmCapsuleReconciler) handleDeletion(ctx context.Context, hc *operatorv1alpha1.HelmCapsule) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(hc, HelmCapsuleFinalizer) {
		return ctrl.Result{}, nil
	}
	if err := r.helmClient.Uninstall(ctx, hc.Name, hc.Namespace); err != nil {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(hc, HelmCapsuleFinalizer)
	return ctrl.Result{}, r.Update(ctx, hc)
}

func (r *HelmCapsuleReconciler) updateStatus(ctx context.Context, hc *operatorv1alpha1.HelmCapsule) error {
	hc.Status.Phase = operatorv1alpha1.PhaseSucceeded
	hc.Status.ObservedGeneration = hc.Generation
	setCondition(&hc.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: hc.Generation,
		Reason:             operatorv1alpha1.HelmCapsuleReasonDeployed,
		Message:            "Helm release deployed",
	})
	return r.Status().Update(ctx, hc)
}

// recordFailure sets phase Failed and a Ready=False condition. Retries on conflict
// by re-fetching the object, so the condition is reliably persisted.
func (r *HelmCapsuleReconciler) recordFailure(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, cause error) {
	reason := operatorv1alpha1.HelmCapsuleReasonHelmFailed
	switch {
	case errors.Is(cause, helm.ErrChartFetch):
		reason = operatorv1alpha1.HelmCapsuleReasonChartFetchFailed
	case errors.Is(cause, helm.ErrValuesBuild):
		reason = operatorv1alpha1.HelmCapsuleReasonValuesBuildFailed
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &operatorv1alpha1.HelmCapsule{}
		if err := r.Get(ctx, types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, latest); err != nil {
			return err
		}
		latest.Status.Phase = operatorv1alpha1.PhaseFailed
		latest.Status.ObservedGeneration = latest.Generation
		setCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: latest.Generation,
			Reason:             reason,
			Message:            cause.Error(),
		})
		return r.Status().Update(ctx, latest)
	}); err != nil {
		log.FromContext(ctx).Error(err, "Failed to record failure condition")
	}
}

func (r *HelmCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HelmCapsule{}).
		Named("helmcapsule").
		Watches(&operatorv1alpha1.Orbit{}, handler.EnqueueRequestsFromMapFunc(r.mapOrbitToHelmCapsules)).
		Watches(&operatorv1alpha1.ConsensusCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapCapsuleToHelmCapsules)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.mapConfigMapToHelmCapsules)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapSecretToHelmCapsules)).
		Complete(r)
}

// mapOrbitToHelmCapsules enqueues all HelmCapsules that reference the changed Orbit.
func (r *HelmCapsuleReconciler) mapOrbitToHelmCapsules(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, list); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, hc := range list.Items {
		if hc.Spec.OrbitRef != nil && hc.Spec.OrbitRef.Name == obj.GetName() {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace},
			})
		}
	}
	return reqs
}

// mapCapsuleToHelmCapsules enqueues HelmCapsules in the same namespace whose OrbitRef
// matches the ConsensusCapsule's Orbit.
func (r *HelmCapsuleReconciler) mapCapsuleToHelmCapsules(ctx context.Context, obj client.Object) []reconcile.Request {
	cc, ok := obj.(*operatorv1alpha1.ConsensusCapsule)
	if !ok {
		return nil
	}
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, list, client.InNamespace(cc.Namespace)); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, hc := range list.Items {
		if hc.Spec.OrbitRef != nil && hc.Spec.OrbitRef.Name == cc.Spec.Orbit {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace},
			})
		}
	}
	return reqs
}

// mapConfigMapToHelmCapsules enqueues HelmCapsules that reference the changed ConfigMap
// via ValuesFrom.
func (r *HelmCapsuleReconciler) mapConfigMapToHelmCapsules(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapValuesSourceToHelmCapsules(ctx, obj, "ConfigMap")
}

// mapSecretToHelmCapsules enqueues HelmCapsules that reference the changed Secret
// via ValuesFrom.
func (r *HelmCapsuleReconciler) mapSecretToHelmCapsules(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapValuesSourceToHelmCapsules(ctx, obj, "Secret")
}

func (r *HelmCapsuleReconciler) mapValuesSourceToHelmCapsules(ctx context.Context, obj client.Object, kind string) []reconcile.Request {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, hc := range list.Items {
		for _, ref := range hc.Spec.ValuesFrom {
			if ref.Kind == kind && ref.Name == obj.GetName() {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace},
				})
				break
			}
		}
	}
	return reqs
}

// setCondition upserts a condition by type, only updating LastTransitionTime when Status changes.
func setCondition(conditions *[]operatorv1alpha1.Condition, newCond metav1.Condition) {
	now := metav1.Now()
	for i, existing := range *conditions {
		if existing.Type != newCond.Type {
			continue
		}
		if existing.Status != newCond.Status {
			newCond.LastTransitionTime = now
		} else {
			newCond.LastTransitionTime = existing.LastTransitionTime
		}
		(*conditions)[i] = newCond
		return
	}
	newCond.LastTransitionTime = now
	*conditions = append(*conditions, newCond)
}
