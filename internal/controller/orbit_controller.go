// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

const OrbitFinalizer = "operator.solo.hiero.org/orbit-finalizer"

type OrbitReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// AllowedNamespaces mirrors --allowed-namespaces; empty means all namespaces.
	AllowedNamespaces []string
}

func NewOrbitReconciler(c client.Client, scheme *runtime.Scheme) *OrbitReconciler {
	return &OrbitReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits,verbs=get;list;watch;update,roleName=manager-role
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits/status,verbs=get;update;patch,roleName=manager-role
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits/finalizers,verbs=update,roleName=manager-role
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=helmcapsules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=envoyproxies,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=log4j2configs,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=nodesettings,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=applicationproperties,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=apipermissionproperties,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=nodepropertiesconfigs,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=throttlesconfigs,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=feeschedules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=simplefeesschedules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=applicationoverrideproperties,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=bootstrapproperties,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkgeneses,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=blocknodesconfigs,verbs=get;list;watch;delete

func (r *OrbitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	orbitObj := &operatorv1alpha1.Orbit{}
	if err := r.Get(ctx, req.NamespacedName, orbitObj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "orbit", "Orbit", orbitObj)
	common.Debug(ctx, ReasonReconcileStart).Msg("Starting Orbit reconciliation")
	defer common.Debug(ctx, ReasonReconcileFinish).Msg("Finished Orbit reconciliation")

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
	if !controllerutil.ContainsFinalizer(orbitObj, OrbitFinalizer) {
		return ctrl.Result{}, nil
	}
	common.Info(ctx, ReasonDeleting).Msg("Deleting Orbit")

	remaining, err := r.deleteOwnedCapsules(ctx, orbitObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	if remaining > 0 {
		common.Info(ctx, ReasonWaitingForCapsuleDeletion).Int("remaining", remaining).Msg("Waiting for owned capsules to be deleted")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	controllerutil.RemoveFinalizer(orbitObj, OrbitFinalizer)
	return ctrl.Result{}, r.Update(ctx, orbitObj)
}

func (r *OrbitReconciler) deleteOwnedCapsules(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) (int, error) {
	kinds := []struct {
		list    client.ObjectList
		orbitOf func(client.Object) string
	}{
		{&operatorv1alpha1.HelmCapsuleList{}, func(o client.Object) string {
			if ref := o.(*operatorv1alpha1.HelmCapsule).Spec.OrbitRef; ref != nil {
				return ref.Name
			}
			return ""
		}},
		{&operatorv1alpha1.ConsensusCapsuleList{}, func(o client.Object) string { return o.(*operatorv1alpha1.ConsensusCapsule).Spec.Orbit }},
		{&operatorv1alpha1.EnvoyProxyList{}, func(o client.Object) string { return o.(*operatorv1alpha1.EnvoyProxy).Spec.Orbit }},
		{&operatorv1alpha1.Log4j2ConfigList{}, func(o client.Object) string { return o.(*operatorv1alpha1.Log4j2Config).Spec.Orbit }},
		{&operatorv1alpha1.NodeSettingsList{}, func(o client.Object) string { return o.(*operatorv1alpha1.NodeSettings).Spec.Orbit }},
		{&operatorv1alpha1.ApplicationPropertiesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.ApplicationProperties).Spec.Orbit }},
		{&operatorv1alpha1.ApiPermissionPropertiesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.ApiPermissionProperties).Spec.Orbit }},
		{&operatorv1alpha1.NodePropertiesConfigList{}, func(o client.Object) string { return o.(*operatorv1alpha1.NodePropertiesConfig).Spec.Orbit }},
		{&operatorv1alpha1.ThrottlesConfigList{}, func(o client.Object) string { return o.(*operatorv1alpha1.ThrottlesConfig).Spec.Orbit }},
		{&operatorv1alpha1.FeeSchedulesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.FeeSchedules).Spec.Orbit }},
		{&operatorv1alpha1.SimpleFeesSchedulesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.SimpleFeesSchedules).Spec.Orbit }},
		{&operatorv1alpha1.ApplicationOverridePropertiesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.ApplicationOverrideProperties).Spec.Orbit }},
		{&operatorv1alpha1.BootstrapPropertiesList{}, func(o client.Object) string { return o.(*operatorv1alpha1.BootstrapProperties).Spec.Orbit }},
		{&operatorv1alpha1.BlockNodesConfigList{}, func(o client.Object) string { return o.(*operatorv1alpha1.BlockNodesConfig).Spec.Orbit }},
		{&operatorv1alpha1.NetworkGenesisList{}, func(o client.Object) string { return o.(*operatorv1alpha1.NetworkGenesis).Spec.Orbit }},
	}

	remaining := 0
	for _, k := range kinds {
		if err := r.List(ctx, k.list); err != nil {
			return remaining, err
		}
		items, err := apimeta.ExtractList(k.list)
		if err != nil {
			return remaining, err
		}
		for _, item := range items {
			obj := item.(client.Object)
			if k.orbitOf(obj) != orbitObj.Name {
				continue
			}
			remaining++
			if obj.GetDeletionTimestamp().IsZero() {
				if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
					return remaining, err
				}
			}
		}
	}
	return remaining, nil
}

func (r *OrbitReconciler) updateStatus(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) error {
	before := orbitObj.Status.DeepCopy()
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

	apimeta.SetStatusCondition(&orbitObj.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  ReasonReconcileSuccess.String(),
		Message: "Orbit is ready",
	})
	r.setNamespaceAllowed(ctx, orbitObj)
	orbitObj.Status.ObservedGeneration = orbitObj.Generation
	if equality.Semantic.DeepEqual(before, &orbitObj.Status) {
		return nil
	}
	return common.WarnOnConflict(ctx, r.Status().Update(ctx, orbitObj))
}

// setNamespaceAllowed relies on the admission-enforced invariant that an Orbit's CRs live in the namespace named after it.
func (r *OrbitReconciler) setNamespaceAllowed(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) {
	if len(r.AllowedNamespaces) == 0 {
		apimeta.RemoveStatusCondition(&orbitObj.Status.Conditions, operatorv1alpha1.OrbitConditionTypeNamespaceAllowed)
		return
	}
	cond := metav1.Condition{
		Type:    operatorv1alpha1.OrbitConditionTypeNamespaceAllowed,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonNamespaceAllowed.String(),
		Message: fmt.Sprintf("Namespace %q is in the operator's allowedNamespaces", orbitObj.Name),
	}
	if !slices.Contains(r.AllowedNamespaces, orbitObj.Name) {
		cond.Status = metav1.ConditionFalse
		cond.Reason = ReasonNamespaceNotAllowed.String()
		cond.Message = fmt.Sprintf("Namespace %q is not in the operator's allowedNamespaces; its resources are not reconciled. "+
			"Create the namespace and add it to the chart's allowedNamespaces (helm upgrade).", orbitObj.Name)
		common.Warn(ctx, ReasonNamespaceNotAllowed).Strs("allowed_namespaces", r.AllowedNamespaces).Msg(cond.Message)
	}
	apimeta.SetStatusCondition(&orbitObj.Status.Conditions, cond)
}

func (r *OrbitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Orbit{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&operatorv1alpha1.ConsensusCapsule{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.ConsensusCapsule) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.HelmCapsule{}, handler.EnqueueRequestsFromMapFunc(r.mapHelmCapsuleToOrbit)).
		Watches(&operatorv1alpha1.EnvoyProxy{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.EnvoyProxy) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.Log4j2Config{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.Log4j2Config) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.NodeSettings{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.NodeSettings) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.ApplicationProperties{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.ApplicationProperties) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.ApiPermissionProperties{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.ApiPermissionProperties) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.NodePropertiesConfig{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.NodePropertiesConfig) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.ThrottlesConfig{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.ThrottlesConfig) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.FeeSchedules{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.FeeSchedules) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.SimpleFeesSchedules{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.SimpleFeesSchedules) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.ApplicationOverrideProperties{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.ApplicationOverrideProperties) string { return o.Spec.Orbit }))).
		Watches(&operatorv1alpha1.BootstrapProperties{}, handler.EnqueueRequestsFromMapFunc(mapOrbitRef(func(o *operatorv1alpha1.BootstrapProperties) string { return o.Spec.Orbit }))).
		Named("orbit").
		Complete(r)
}

// mapOrbitRef returns a handler.MapFunc that enqueues the Orbit named by extract(obj).
// Use it in Watches() for any CR type with a spec.orbit string field.
func mapOrbitRef[T client.Object](extract func(T) string) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		o, ok := obj.(T)
		if !ok {
			return nil
		}
		name := extract(o)
		if name == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
	}
}

func (r *OrbitReconciler) mapHelmCapsuleToOrbit(_ context.Context, obj client.Object) []reconcile.Request {
	hc, ok := obj.(*operatorv1alpha1.HelmCapsule)
	if !ok || hc.Spec.OrbitRef == nil || hc.Spec.OrbitRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: hc.Spec.OrbitRef.Name}}}
}
