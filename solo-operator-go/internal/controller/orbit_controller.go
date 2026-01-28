// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/orbit"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	// OrbitFinalizer is the finalizer for Orbit resources
	OrbitFinalizer = "operator.solo.hedera.com/orbit-finalizer"
)

// OrbitReconciler reconciles an Orbit object
type OrbitReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	subreconcilers *subreconciler.Chain[*operatorv1alpha1.Orbit]
}

// NewOrbitReconciler creates a new OrbitReconciler with subreconcilers initialized
func NewOrbitReconciler(c client.Client, scheme *runtime.Scheme) *OrbitReconciler {
	return &OrbitReconciler{
		Client:         c,
		Scheme:         scheme,
		subreconcilers: orbit.NewSubReconcilers(c, scheme),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for Orbit resources
func (r *OrbitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting Orbit reconciliation", "name", req.Name)
	defer logger.Info("Finished Orbit reconciliation", "name", req.Name)

	// Fetch the Orbit instance
	orbitObj := &operatorv1alpha1.Orbit{}
	if err := r.Get(ctx, req.NamespacedName, orbitObj); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Orbit resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Orbit")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !orbitObj.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, orbitObj)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(orbitObj, OrbitFinalizer) {
		controllerutil.AddFinalizer(orbitObj, OrbitFinalizer)
		if err := r.Update(ctx, orbitObj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize subreconcilers if not done (for backward compatibility)
	if r.subreconcilers == nil {
		r.subreconcilers = orbit.NewSubReconcilers(r.Client, r.Scheme)
	}

	// Reconcile all dependent resources using subreconciler chain
	result, err := r.subreconcilers.Reconcile(ctx, orbitObj)
	if err != nil {
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	// Update status
	if err := r.updateStatus(ctx, orbitObj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of an Orbit resource
func (r *OrbitReconciler) handleDeletion(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(orbitObj, OrbitFinalizer) {
		logger.Info("Deleting Orbit", "name", orbitObj.Name)

		// Delete the namespace (this cascades to all namespaced resources)
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, client.ObjectKey{Name: orbitObj.Name}, ns); err == nil {
			logger.Info("Deleting orbit namespace", "name", orbitObj.Name)
			if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(orbitObj, OrbitFinalizer)
		if err := r.Update(ctx, orbitObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the Orbit status
func (r *OrbitReconciler) updateStatus(ctx context.Context, orbitObj *operatorv1alpha1.Orbit) error {
	now := metav1.Now()

	// Initialize status if needed
	if orbitObj.Status.Phase == "" {
		orbitObj.Status.Phase = operatorv1alpha1.PhaseRunning
	}

	if orbitObj.Status.StartTime == "" {
		orbitObj.Status.StartTime = now.Format(time.RFC3339)
	}

	// Calculate expiration time if lifetime is set
	if orbitObj.Status.ExpirationTime == "" && orbitObj.Spec.Lifetime != "" {
		dur, err := time.ParseDuration(orbitObj.Spec.Lifetime)
		if err == nil {
			expiration := now.Add(dur)
			orbitObj.Status.ExpirationTime = expiration.Format(time.RFC3339)
		}
	}

	// Update conditions
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "ReconcileSuccess",
		Message:            "Orbit is ready",
	}

	// Update or add the condition
	found := false
	for i, c := range orbitObj.Status.Conditions {
		if c.Type == "Ready" {
			orbitObj.Status.Conditions[i] = readyCondition
			found = true
			break
		}
	}
	if !found {
		orbitObj.Status.Conditions = append(orbitObj.Status.Conditions, readyCondition)
	}

	orbitObj.Status.ObservedGeneration = orbitObj.Generation

	return r.Status().Update(ctx, orbitObj)
}

// SetupWithManager sets up the controller with the Manager
func (r *OrbitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize subreconcilers if not done
	if r.subreconcilers == nil {
		r.subreconcilers = orbit.NewSubReconcilers(r.Client, r.Scheme)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Orbit{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&rbacv1.Role{}).
		Named("orbit").
		Complete(r)
}
