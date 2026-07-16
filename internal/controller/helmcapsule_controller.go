// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	HelmCapsuleFinalizer = "operator.solo.hiero.org/helmcapsule-finalizer"
	conditionReady       = "Ready"
	// SkipUninstallAnnotation can be set to "true" on a HelmCapsule to bypass
	// Helm uninstall during deletion. Useful when a release is stuck and an
	// admin needs to unblock the finalizer without running helm uninstall.
	SkipUninstallAnnotation = "operator.solo.hiero.org/skip-uninstall"
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

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=helmcapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=helmcapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=helmcapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// Secrets: read for ValuesFrom/CredentialsRef; full CRUD for Helm release storage (driver=secrets).
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// Core resources created/managed by Helm charts (services, serviceaccounts, pvcs, pods).
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts;persistentvolumeclaims;endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// Workload resources created by Helm charts.
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;replicasets;daemonsets,verbs=get;list;watch;create;update;patch;delete
// Batch resources created by Helm charts.
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// RBAC resources that charts may create (e.g. mirror importer role).
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// Network resources created by Helm charts.
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies;ingresses,verbs=get;list;watch;create;update;patch;delete
// PodDisruptionBudgets (policy API group) created by charts such as mirror/redis.
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// HorizontalPodAutoscalers created by charts.
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// Flux HelmRelease CRD: mirror-monitor Role grants GET on helmreleases; operator must hold this to avoid RBAC escalation.
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch

func (r *HelmCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	hc := &operatorv1alpha1.HelmCapsule{}
	if err := r.Get(ctx, req.NamespacedName, hc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "helmcapsule", "HelmCapsule", hc)
	common.Info(ctx, ReasonReconcileStart).Msg("Reconciling HelmCapsule")

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

	if hc.Annotations[SkipUninstallAnnotation] == "true" {
		common.Info(ctx, ReasonSkippingUninstall).
			Str("annotation", SkipUninstallAnnotation).
			Msg("Skipping Helm uninstall — escape-hatch annotation is set")
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
	// Re-fetch on each retry to pick up the latest ResourceVersion and avoid
	// silently dropping phase/condition updates on concurrent spec changes.
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, hc); err != nil {
			return err
		}

		oldPhase := hc.Status.Phase
		oldGen := hc.Status.ObservedGeneration
		oldConds := make([]operatorv1alpha1.Condition, len(hc.Status.Conditions))
		copy(oldConds, hc.Status.Conditions)

		var readyCondition metav1.Condition
		switch hc.Status.ReleaseStatus {
		case "deployed":
			hc.Status.Phase = operatorv1alpha1.PhaseSucceeded
			readyCondition = metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: hc.Generation,
				Reason:             operatorv1alpha1.HelmCapsuleReasonDeployed.String(),
				Message:            "Helm release deployed",
			}
		case "pending-install", "pending-upgrade", "pending-rollback":
			hc.Status.Phase = operatorv1alpha1.PhaseRunning
			readyCondition = metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: hc.Generation,
				Reason:             operatorv1alpha1.HelmCapsuleReasonProgressing.String(),
				Message:            fmt.Sprintf("Helm release is %s", hc.Status.ReleaseStatus),
			}
		case "failed", "uninstalling", "uninstalled", "superseded":
			hc.Status.Phase = operatorv1alpha1.PhaseFailed
			readyCondition = metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: hc.Generation,
				Reason:             operatorv1alpha1.HelmCapsuleReasonReleaseNotReady.String(),
				Message:            fmt.Sprintf("Helm release status: %s", hc.Status.ReleaseStatus),
			}
		default:
			hc.Status.Phase = operatorv1alpha1.PhaseFailed
			readyCondition = metav1.Condition{
				Type:               conditionReady,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: hc.Generation,
				Reason:             operatorv1alpha1.HelmCapsuleReasonUnknownReleaseStatus.String(),
				Message:            fmt.Sprintf("Helm release status is unrecognised: %q", hc.Status.ReleaseStatus),
			}
		}
		hc.Status.ObservedGeneration = hc.Generation
		setCondition(&hc.Status.Conditions, readyCondition)

		if hc.Status.Phase == oldPhase && hc.Status.ObservedGeneration == oldGen && reflect.DeepEqual(hc.Status.Conditions, oldConds) {
			return nil
		}
		return r.Status().Update(ctx, hc)
	})
}

// recordFailure sets phase Failed and a Ready=False condition, retrying on
// conflict. The condition Reason is the errx reason carried by cause (HelmFailed
// if undecorated).
func (r *HelmCapsuleReconciler) recordFailure(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, cause error) {
	reason := common.ReasonFrom(cause, operatorv1alpha1.HelmCapsuleReasonHelmFailed)

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
			Message:            common.MessageWithHints(cause),
		})
		return r.Status().Update(ctx, latest)
	}); err != nil {
		common.Err(ctx, err, ReasonFailureConditionRecordFailed).Msg("Failed to record failure condition")
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

// mapOrbitToHelmCapsules re-enqueues HelmCapsules that reference the changed Orbit.
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
	reqs := r.mapValuesSourceToHelmCapsules(ctx, obj, "Secret")

	// Also cover chart.credentialsRef so credential rotation triggers a reconcile.
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		return reqs
	}
	seen := make(map[types.NamespacedName]bool, len(reqs))
	for _, r := range reqs {
		seen[r.NamespacedName] = true
	}
	for _, hc := range list.Items {
		if hc.Spec.Chart.CredentialsRef != nil && hc.Spec.Chart.CredentialsRef.Name == obj.GetName() {
			nn := types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}
			if !seen[nn] {
				reqs = append(reqs, reconcile.Request{NamespacedName: nn})
			}
		}
	}
	return reqs
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

// mapHelmCapsuleToPeers re-enqueues peers so release-name conflicts self-heal when the
// conflicting capsule is renamed or deleted. Each reconcile triggers N-1 extra reconciles;
// acceptable at single-digit-capsule scale but can be replaced with a releaseName indexer.
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

// findCondition returns the condition with the given type, or nil.
func findCondition(conditions []operatorv1alpha1.Condition, condType string) *operatorv1alpha1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
