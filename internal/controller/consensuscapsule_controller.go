// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	// ConsensusCapsuleFinalizer is the finalizer for ConsensusCapsule resources
	ConsensusCapsuleFinalizer = "operator.solo.hedera.com/consensuscapsule-finalizer"

	// LabelComponent is the component label
	LabelComponent = "app.kubernetes.io/component"

	// LabelNodeId is the node ID label
	LabelNodeId = "operator.solo.hedera.com/node-id"
)

// ConsensusCapsuleReconciler reconciles a ConsensusCapsule object
type ConsensusCapsuleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	subreconciler *subreconciler.Chain[*operatorv1alpha1.ConsensusCapsule]
}

// NewConsensusCapsuleReconciler creates a new ConsensusCapsuleReconciler
func NewConsensusCapsuleReconciler(c client.Client, scheme *runtime.Scheme) *ConsensusCapsuleReconciler {
	return &ConsensusCapsuleReconciler{
		Client:        c,
		Scheme:        scheme,
		subreconciler: consensus.NewSubReconcilers(c, scheme),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for ConsensusCapsule resources
func (r *ConsensusCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting ConsensusCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished ConsensusCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Fetch the ConsensusCapsule instance
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	if err := r.Get(ctx, req.NamespacedName, capsule); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ConsensusCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ConsensusCapsule")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !capsule.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, capsule)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(capsule, ConsensusCapsuleFinalizer) {
		controllerutil.AddFinalizer(capsule, ConsensusCapsuleFinalizer)
		if err := r.Update(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Run subreconcilers
	result, err := r.subreconciler.Reconcile(ctx, capsule)
	if err != nil {
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	// Update status based on pod state
	if err := r.updateStatus(ctx, capsule); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a ConsensusCapsule resource
func (r *ConsensusCapsuleReconciler) handleDeletion(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(capsule, ConsensusCapsuleFinalizer) {
		logger.Info("Deleting ConsensusCapsule",
			"namespace", capsule.Namespace,
			"name", capsule.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(capsule, ConsensusCapsuleFinalizer)
		if err := r.Update(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// updateStatus updates the ConsensusCapsule status based on pod state
func (r *ConsensusCapsuleReconciler) updateStatus(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	// List pods matching the deployment
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(capsule.Namespace),
		client.MatchingLabels(consensus.PodLabels(capsule))); err != nil {
		return err
	}

	// Determine phase from pod status
	phase := operatorv1alpha1.PhasePending
	if len(podList.Items) > 0 {
		pod := podList.Items[0]
		switch pod.Status.Phase {
		case corev1.PodRunning:
			phase = operatorv1alpha1.PhaseRunning
		case corev1.PodSucceeded:
			phase = operatorv1alpha1.PhaseSucceeded
		case corev1.PodFailed:
			phase = operatorv1alpha1.PhaseFailed
		case corev1.PodPending:
			phase = operatorv1alpha1.PhasePending
		default:
			phase = operatorv1alpha1.PhaseUnknown
		}
	}

	// Check if status needs update
	if capsule.Status.Phase == phase && capsule.Status.ObservedGeneration == capsule.Generation {
		return nil
	}

	capsule.Status.Phase = phase
	capsule.Status.ObservedGeneration = capsule.Generation

	// Update address book entry
	capsule.Status.AddressBookEntry = &operatorv1alpha1.AddressBookEntry{
		NodeId:           capsule.Spec.NodeId,
		NickName:         capsule.Name,
		SelfName:         capsule.Name,
		Weight:           capsule.Spec.Weight,
		InternalHostname: consensus.InternalHostname(capsule),
		InternalPort:     consensus.GossipPort,
		ExternalHostname: capsule.Spec.ExternalAddress,
		ExternalPort:     consensus.GossipPort,
		NodeAccount:      capsule.Spec.AccountId,
	}

	return r.Status().Update(ctx, capsule)
}

// SetupWithManager sets up the controller with the Manager
func (r *ConsensusCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ConsensusCapsule{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.Deployment{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("consensuscapsule").
		Complete(r)
}
