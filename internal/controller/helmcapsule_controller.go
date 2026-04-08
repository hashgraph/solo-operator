// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"reflect"

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
	// SkipUninstallAnnotation can be set to "true" on a HelmCapsule to bypass
	// Helm uninstall during deletion. Useful when a release is stuck and an
	// admin needs to unblock the finalizer without running helm uninstall.
	SkipUninstallAnnotation = "operator.solo.hedera.com/skip-uninstall"
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
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// Secrets: read for ValuesFrom/CredentialsRef; full CRUD for Helm release storage (driver=secrets).
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

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
		// Surface Pending so observers know the resource was registered
		// before any Helm work has started.
		hc.Status.Phase = operatorv1alpha1.PhasePending
		if err := r.Status().Update(ctx, hc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	result, err := r.subreconciler.Reconcile(ctx, hc)
	if err != nil {
		r.recordFailure(ctx, hc, err)
		return result, err
	}

	return result, r.updateStatus(ctx, hc)
}

func (r *HelmCapsuleReconciler) handleDeletion(ctx context.Context, hc *operatorv1alpha1.HelmCapsule) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(hc, HelmCapsuleFinalizer) {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	if hc.Annotations[SkipUninstallAnnotation] == "true" {
		logger.Info("Skipping Helm uninstall — escape-hatch annotation is set",
			"annotation", SkipUninstallAnnotation)
	} else {
		releaseName := hc.Spec.ReleaseName
		if releaseName == "" {
			releaseName = hc.Name
		}
		timeout := helm.DefaultTimeout
		if hc.Spec.Timeout != nil && hc.Spec.Timeout.Duration > 0 {
			timeout = hc.Spec.Timeout.Duration
		}
		opts := helm.UninstallOptions{
			Timeout:      timeout,
			KeepHistory:  hc.Spec.KeepHistory,
			DisableHooks: hc.Spec.DisableHooks,
			Wait:         hc.Spec.Wait,
		}
		if err := r.helmClient.Uninstall(ctx, releaseName, hc.Namespace, opts); err != nil {
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(hc, HelmCapsuleFinalizer)
	return ctrl.Result{}, r.Update(ctx, hc)
}

func (r *HelmCapsuleReconciler) updateStatus(ctx context.Context, hc *operatorv1alpha1.HelmCapsule) error {
	// Capture current status before mutation. setCondition mutates Conditions in-place,
	// so copy the slice to get an independent snapshot for change detection.
	oldPhase := hc.Status.Phase
	oldGen := hc.Status.ObservedGeneration
	oldConds := make([]operatorv1alpha1.Condition, len(hc.Status.Conditions))
	copy(oldConds, hc.Status.Conditions)

	// Reflect the actual Helm release state in Phase.
	// pending-* statuses mean the release is in-flight; everything else (or no
	// prior release) is treated as the operator having done its job successfully.
	switch hc.Status.ReleaseStatus {
	case "pending-install", "pending-upgrade", "pending-rollback":
		hc.Status.Phase = operatorv1alpha1.PhaseRunning
	default:
		hc.Status.Phase = operatorv1alpha1.PhaseSucceeded
	}
	hc.Status.ObservedGeneration = hc.Generation
	setCondition(&hc.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: hc.Generation,
		Reason:             operatorv1alpha1.HelmCapsuleReasonDeployed,
		Message:            "Helm release deployed",
	})

	if hc.Status.Phase == oldPhase && hc.Status.ObservedGeneration == oldGen && reflect.DeepEqual(hc.Status.Conditions, oldConds) {
		return nil
	}
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
	case errors.Is(cause, helm.ErrReleaseNameConflict):
		reason = operatorv1alpha1.HelmCapsuleReasonReleaseNameConflict
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
		Watches(&operatorv1alpha1.HelmCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapHelmCapsuleToPeers)).
		Complete(r)
}

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

func (r *HelmCapsuleReconciler) mapConfigMapToHelmCapsules(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapValuesSourceToHelmCapsules(ctx, obj, "ConfigMap")
}

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

// mapHelmCapsuleToPeers re-enqueues peers so release-name conflicts self-heal
// promptly when the conflicting capsule is renamed or deleted.
func (r *HelmCapsuleReconciler) mapHelmCapsuleToPeers(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, hc := range list.Items {
		if hc.UID == obj.GetUID() {
			continue
		}
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace},
		})
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
