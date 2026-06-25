// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const OrbitFinalizer = "operator.solo.hedera.com/orbit-finalizer"

// OrbitReconciler holds Orbit configuration and cascade-deletes referencing
// Capsules on delete. It does not create or own any namespace-scoped resources;
// each Capsule reconciler owns the resources it needs in its own namespace.
type OrbitReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewOrbitReconciler(c client.Client, scheme *runtime.Scheme) *OrbitReconciler {
	return &OrbitReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=envoyproxies,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=log4j2configs,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=nodesettings,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=applicationproperties,verbs=get;list;watch;delete

func (r *OrbitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting Orbit reconciliation", "name", req.Name)
	defer logger.Info("Finished Orbit reconciliation", "name", req.Name)

	orbitObj := &operatorv1alpha1.Orbit{}
	if err := r.Get(ctx, req.NamespacedName, orbitObj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !orbitObj.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, orbitObj)
	}

	if !controllerutil.ContainsFinalizer(orbitObj, OrbitFinalizer) {
		controllerutil.AddFinalizer(orbitObj, OrbitFinalizer)
		if err := r.Update(ctx, orbitObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, r.updateStatus(ctx, orbitObj)
}

func (r *OrbitReconciler) handleDeletion(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(orbitObj, OrbitFinalizer) {
		return ctrl.Result{}, nil
	}
	logger.Info("Deleting Orbit", "name", orbitObj.Name)

	remaining, err := r.deleteOwnedCapsules(ctx, orbitObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	if remaining > 0 {
		logger.Info("Waiting for owned capsules to be deleted", "remaining", remaining)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	controllerutil.RemoveFinalizer(orbitObj, OrbitFinalizer)
	return ctrl.Result{}, r.Update(ctx, orbitObj)
}

// deleteOwnedCapsules issues Delete for every Capsule that references this Orbit,
// cluster-wide (no namespace filter), and returns how many are still present.
func (r *OrbitReconciler) deleteOwnedCapsules(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) (int, error) {
	remaining := 0

	consensus := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := r.List(ctx, consensus); err != nil {
		return 0, err
	}
	for i := range consensus.Items {
		cc := &consensus.Items[i]
		if cc.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if cc.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, cc); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	helm := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.List(ctx, helm); err != nil {
		return remaining, err
	}
	for i := range helm.Items {
		hc := &helm.Items[i]
		if hc.Spec.OrbitRef == nil || hc.Spec.OrbitRef.Name != orbitObj.Name {
			continue
		}
		remaining++
		if hc.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, hc); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	haproxy := &operatorv1alpha1.HAProxyCapsuleList{}
	if err := r.List(ctx, haproxy); err != nil {
		return remaining, err
	}
	for i := range haproxy.Items {
		hp := &haproxy.Items[i]
		if hp.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if hp.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, hp); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	envoy := &operatorv1alpha1.EnvoyProxyList{}
	if err := r.List(ctx, envoy); err != nil {
		return remaining, err
	}
	for i := range envoy.Items {
		ep := &envoy.Items[i]
		if ep.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if ep.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, ep); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	log4j2 := &operatorv1alpha1.Log4j2ConfigList{}
	if err := r.List(ctx, log4j2); err != nil {
		return remaining, err
	}
	for i := range log4j2.Items {
		lc := &log4j2.Items[i]
		if lc.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if lc.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, lc); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	nodeSettingsList := &operatorv1alpha1.NodeSettingsList{}
	if err := r.List(ctx, nodeSettingsList); err != nil {
		return remaining, err
	}
	for i := range nodeSettingsList.Items {
		ns := &nodeSettingsList.Items[i]
		if ns.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if ns.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	appPropsList := &operatorv1alpha1.ApplicationPropertiesList{}
	if err := r.List(ctx, appPropsList); err != nil {
		return remaining, err
	}
	for i := range appPropsList.Items {
		ap := &appPropsList.Items[i]
		if ap.Spec.Orbit != orbitObj.Name {
			continue
		}
		remaining++
		if ap.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, ap); err != nil && !apierrors.IsNotFound(err) {
				return remaining, err
			}
		}
	}

	return remaining, nil
}

func (r *OrbitReconciler) updateStatus(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) error {
	now := metav1.Now()

	if orbitObj.Status.Phase == "" {
		orbitObj.Status.Phase = operatorv1alpha1.PhaseRunning
	}
	if orbitObj.Status.StartTime == "" {
		orbitObj.Status.StartTime = now.Format(time.RFC3339)
	}
	if orbitObj.Status.ExpirationTime == "" && orbitObj.Spec.Lifetime != "" {
		if dur, err := time.ParseDuration(orbitObj.Spec.Lifetime); err == nil {
			orbitObj.Status.ExpirationTime = now.Add(dur).Format(time.RFC3339)
		}
	}

	ready := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "ReconcileSuccess",
		Message:            "Orbit is ready",
	}
	found := false
	for i, c := range orbitObj.Status.Conditions {
		if c.Type == "Ready" {
			orbitObj.Status.Conditions[i] = ready
			found = true
			break
		}
	}
	if !found {
		orbitObj.Status.Conditions = append(orbitObj.Status.Conditions, ready)
	}
	orbitObj.Status.ObservedGeneration = orbitObj.Generation
	return r.Status().Update(ctx, orbitObj)
}

func (r *OrbitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Orbit{}).
		Watches(&operatorv1alpha1.ConsensusCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapConsensusCapsuleToOrbit)).
		Watches(&operatorv1alpha1.HelmCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapHelmCapsuleToOrbit)).
		Watches(&operatorv1alpha1.HAProxyCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapHAProxyCapsuleToOrbit)).
		Watches(&operatorv1alpha1.EnvoyProxy{}, handler.EnqueueRequestsFromMapFunc(r.mapEnvoyProxyToOrbit)).
		Watches(&operatorv1alpha1.Log4j2Config{}, handler.EnqueueRequestsFromMapFunc(r.mapLog4j2ConfigToOrbit)).
		Watches(&operatorv1alpha1.NodeSettings{}, handler.EnqueueRequestsFromMapFunc(r.mapNodeSettingsToOrbit)).
		Watches(&operatorv1alpha1.ApplicationProperties{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationPropertiesToOrbit)).
		Named("orbit").
		Complete(r)
}

func (r *OrbitReconciler) mapConsensusCapsuleToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	cc, ok := obj.(*operatorv1alpha1.ConsensusCapsule)
	if !ok || cc.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: cc.Spec.Orbit}}}
}

func (r *OrbitReconciler) mapHelmCapsuleToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	hc, ok := obj.(*operatorv1alpha1.HelmCapsule)
	if !ok || hc.Spec.OrbitRef == nil || hc.Spec.OrbitRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: hc.Spec.OrbitRef.Name}}}
}

func (r *OrbitReconciler) mapHAProxyCapsuleToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	hp, ok := obj.(*operatorv1alpha1.HAProxyCapsule)
	if !ok || hp.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: hp.Spec.Orbit}}}
}

func (r *OrbitReconciler) mapEnvoyProxyToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	ep, ok := obj.(*operatorv1alpha1.EnvoyProxy)
	if !ok || ep.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ep.Spec.Orbit}}}
}

func (r *OrbitReconciler) mapLog4j2ConfigToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	lc, ok := obj.(*operatorv1alpha1.Log4j2Config)
	if !ok || lc.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: lc.Spec.Orbit}}}
}

func (r *OrbitReconciler) mapNodeSettingsToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	ns, ok := obj.(*operatorv1alpha1.NodeSettings)
	if !ok || ns.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ns.Spec.Orbit}}}
}

func (r *OrbitReconciler) mapApplicationPropertiesToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	ap, ok := obj.(*operatorv1alpha1.ApplicationProperties)
	if !ok || ap.Spec.Orbit == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ap.Spec.Orbit}}}
}
