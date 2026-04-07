// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/haproxy"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	// HAProxyCapsuleFinalizer is the finalizer for HAProxyCapsule resources
	HAProxyCapsuleFinalizer = "operator.solo.hedera.com/haproxycapsule-finalizer"
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

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for HAProxyCapsule resources
func (r *HAProxyCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting HAProxyCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished HAProxyCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Fetch the HAProxyCapsule instance
	haproxyObj := &operatorv1alpha1.HAProxyCapsule{}
	if err := r.Get(ctx, req.NamespacedName, haproxyObj); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("HAProxyCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get HAProxyCapsule")
		return ctrl.Result{}, err
	}

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
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(haproxyObj, HAProxyCapsuleFinalizer) {
		logger.Info("Deleting HAProxyCapsule",
			"namespace", haproxyObj.Namespace,
			"name", haproxyObj.Name)

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
	// Capture current status — all fields are value types so this is a full copy.
	oldStatus := haproxyObj.Status

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

	if haproxyObj.Status.Phase == oldStatus.Phase && haproxyObj.Status.ObservedGeneration == oldStatus.ObservedGeneration {
		return nil
	}
	return r.Status().Update(ctx, haproxyObj)
}

// SetupWithManager sets up the controller with the Manager
func (r *HAProxyCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HAProxyCapsule{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Named("haproxycapsule").
		Complete(r)
}
