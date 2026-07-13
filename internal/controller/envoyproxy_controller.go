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
	"github.com/hashgraph/solo-operator/internal/resources/envoy"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	// EnvoyProxyFinalizer is the finalizer for EnvoyProxy resources
	EnvoyProxyFinalizer = "operator.solo.hiero.org/envoyproxy-finalizer"
)

// EnvoyProxyReconciler reconciles an EnvoyProxy object
type EnvoyProxyReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	subreconciler *subreconciler.Chain[*operatorv1alpha1.EnvoyProxy]
}

// NewEnvoyProxyReconciler creates a new EnvoyProxyReconciler
func NewEnvoyProxyReconciler(c client.Client, scheme *runtime.Scheme) *EnvoyProxyReconciler {
	return &EnvoyProxyReconciler{
		Client:        c,
		Scheme:        scheme,
		subreconciler: envoy.NewSubReconcilers(c, scheme),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=envoyproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=envoyproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=envoyproxies/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for EnvoyProxy resources
func (r *EnvoyProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting EnvoyProxy reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished EnvoyProxy reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Fetch the EnvoyProxy instance
	envoyObj := &operatorv1alpha1.EnvoyProxy{}
	if err := r.Get(ctx, req.NamespacedName, envoyObj); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("EnvoyProxy resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get EnvoyProxy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !envoyObj.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, envoyObj)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(envoyObj, EnvoyProxyFinalizer) {
		controllerutil.AddFinalizer(envoyObj, EnvoyProxyFinalizer)
		if err := r.Update(ctx, envoyObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Run subreconcilers
	result, err := r.subreconciler.Reconcile(ctx, envoyObj)
	if err != nil {
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	// Update status
	if err := r.updateStatus(ctx, envoyObj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of an EnvoyProxy resource
func (r *EnvoyProxyReconciler) handleDeletion(ctx context.Context, envoyObj *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(envoyObj, EnvoyProxyFinalizer) {
		logger.Info("Deleting EnvoyProxy",
			"namespace", envoyObj.Namespace,
			"name", envoyObj.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(envoyObj, EnvoyProxyFinalizer)
		if err := r.Update(ctx, envoyObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the EnvoyProxy status
func (r *EnvoyProxyReconciler) updateStatus(ctx context.Context, envoyObj *operatorv1alpha1.EnvoyProxy) error {
	// Get deployment status
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: envoyObj.Name, Namespace: envoyObj.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Deployment not found
		envoyObj.Status.Replicas = 0
		envoyObj.Status.ReadyReplicas = 0
		envoyObj.Status.AvailableReplicas = 0
		envoyObj.Status.UnavailableReplicas = 0
		envoyObj.Status.UpdatedReplicas = 0
	} else {
		envoyObj.Status.Replicas = deploy.Status.Replicas
		envoyObj.Status.ReadyReplicas = deploy.Status.ReadyReplicas
		envoyObj.Status.AvailableReplicas = deploy.Status.AvailableReplicas
		envoyObj.Status.UnavailableReplicas = deploy.Status.UnavailableReplicas
		envoyObj.Status.UpdatedReplicas = deploy.Status.UpdatedReplicas
		envoyObj.Status.Conditions = deploy.Status.Conditions
	}

	envoyObj.Status.ObservedGeneration = envoyObj.Generation

	return r.Status().Update(ctx, envoyObj)
}

// SetupWithManager sets up the controller with the Manager
func (r *EnvoyProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.EnvoyProxy{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Named("envoyproxy").
		Complete(r)
}
