// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
	hc.Status.Phase = operatorv1alpha1.PhaseRunning
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

// recordFailure sets phase Failed and a Ready=False condition with an appropriate reason.
// Returns without propagating so controller-runtime handles retry backoff.
func (r *HelmCapsuleReconciler) recordFailure(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, cause error) {
	reason := operatorv1alpha1.HelmCapsuleReasonHelmFailed
	switch {
	case errors.Is(cause, helm.ErrChartFetch):
		reason = operatorv1alpha1.HelmCapsuleReasonChartFetchFailed
	case errors.Is(cause, helm.ErrValuesBuild):
		reason = operatorv1alpha1.HelmCapsuleReasonValuesBuildFailed
	}

	hc.Status.Phase = operatorv1alpha1.PhaseFailed
	hc.Status.ObservedGeneration = hc.Generation
	setCondition(&hc.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: hc.Generation,
		Reason:             reason,
		Message:            cause.Error(),
	})

	if err := r.Status().Update(ctx, hc); err != nil {
		log.FromContext(ctx).Error(err, "Failed to record failure condition")
	}
}

func (r *HelmCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HelmCapsule{}).
		Named("helmcapsule").
		Complete(r)
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
