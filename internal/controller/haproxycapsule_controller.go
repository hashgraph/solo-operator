// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/haproxy"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	// HAProxyCapsuleFinalizer is the finalizer for HAProxyCapsule resources
	HAProxyCapsuleFinalizer = "operator.solo.hiero.org/haproxycapsule-finalizer"
)

// HAProxyCapsuleReconciler reconciles a HAProxyCapsule object
type HAProxyCapsuleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	subreconciler *subreconciler.Chain[*operatorv1alpha1.HAProxyCapsule]
}

// NewHAProxyCapsuleReconciler creates a new HAProxyCapsuleReconciler
func NewHAProxyCapsuleReconciler(c client.Client, scheme *runtime.Scheme) *HAProxyCapsuleReconciler {
	return &HAProxyCapsuleReconciler{
		Client:        c,
		Scheme:        scheme,
		subreconciler: haproxy.NewSubReconcilers(c, scheme),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=haproxycapsules,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=haproxycapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=haproxycapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is the main reconciliation loop for HAProxyCapsule resources
func (r *HAProxyCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = common.RequestContext(ctx, req)
	haproxyObj := &operatorv1alpha1.HAProxyCapsule{}
	if err := r.Get(ctx, req.NamespacedName, haproxyObj); err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, ReasonResourceNotFound).Msg("HAProxyCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		common.Err(ctx, err, ReasonGetFailed).Msg("Failed to get HAProxyCapsule")
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "haproxycapsule", "HAProxyCapsule", haproxyObj)
	common.Debug(ctx, ReasonReconcileStart).Msg("Starting HAProxyCapsule reconciliation")
	defer common.Debug(ctx, ReasonReconcileFinish).Msg("Finished HAProxyCapsule reconciliation")

	// Handle deletion
	if !haproxyObj.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, haproxyObj)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(haproxyObj, HAProxyCapsuleFinalizer) {
		controllerutil.AddFinalizer(haproxyObj, HAProxyCapsuleFinalizer)
		if err := r.Update(ctx, haproxyObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := validateProxyDigestPolicy(ctx, r.Client, haproxyObj.Spec.Orbit, "HAProxyCapsule", haproxyObj.Name, haproxy.Images(haproxyObj)...); err != nil {
		return ctrl.Result{}, err
	}

	// Run subreconcilers
	result, err := r.subreconciler.Reconcile(ctx, haproxyObj)
	if err != nil {
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	// Update status
	if err := r.updateStatus(ctx, haproxyObj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a HAProxyCapsule resource
func (r *HAProxyCapsuleReconciler) handleDeletion(ctx context.Context, haproxyObj *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(haproxyObj, HAProxyCapsuleFinalizer) {
		common.Info(ctx, ReasonDeleting).Msg("Deleting HAProxyCapsule")

		// Remove finalizer
		controllerutil.RemoveFinalizer(haproxyObj, HAProxyCapsuleFinalizer)
		if err := r.Update(ctx, haproxyObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the HAProxyCapsule status
func (r *HAProxyCapsuleReconciler) updateStatus(ctx context.Context, haproxyObj *operatorv1alpha1.HAProxyCapsule) error {
	// Check deployment status
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: haproxyObj.Name, Namespace: haproxyObj.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		haproxyObj.Status.Phase = operatorv1alpha1.PhasePending
	} else {
		if deploy.Status.ReadyReplicas > 0 {
			haproxyObj.Status.Phase = operatorv1alpha1.PhaseRunning
		} else {
			haproxyObj.Status.Phase = operatorv1alpha1.PhasePending
		}
	}

	haproxyObj.Status.ObservedGeneration = haproxyObj.Generation

	return common.WarnOnConflict(ctx, r.Status().Update(ctx, haproxyObj))
}

const (
	fieldHAProxySecretRefs    = "haproxy.secretRefs"
	fieldHAProxyConfigMapRefs = "haproxy.configMapRefs"
)

// SetupWithManager sets up the controller with the Manager
func (r *HAProxyCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	indexer := mgr.GetFieldIndexer()
	if err := indexer.IndexField(ctx, &operatorv1alpha1.HAProxyCapsule{}, fieldHAProxySecretRefs, func(o client.Object) []string {
		return haproxy.ReferencedSecrets(o.(*operatorv1alpha1.HAProxyCapsule))
	}); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &operatorv1alpha1.HAProxyCapsule{}, fieldHAProxyConfigMapRefs, func(o client.Object) []string {
		return haproxy.ReferencedConfigMaps(o.(*operatorv1alpha1.HAProxyCapsule))
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HAProxyCapsule{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapByIndex(fieldHAProxySecretRefs))).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.mapByIndex(fieldHAProxyConfigMapRefs))).
		Named("haproxycapsule").
		Complete(r)
}

func (r *HAProxyCapsuleReconciler) mapByIndex(field string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		list := &operatorv1alpha1.HAProxyCapsuleList{}
		if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace()), client.MatchingFields{field: obj.GetName()}); err != nil {
			common.Err(ctx, err, ReasonListFailed).Str("index", field).Msg("Failed to list HAProxyCapsules for watched source")
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(list.Items))
		for _, item := range list.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: item.Name, Namespace: item.Namespace}})
		}
		return reqs
	}
}
