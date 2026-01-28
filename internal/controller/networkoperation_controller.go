// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/operations/check"
	"github.com/hashgraph/solo-operator/internal/operations/genesis"
	"github.com/hashgraph/solo-operator/internal/operations/node"
	"github.com/hashgraph/solo-operator/internal/operations/upgrade"
	"github.com/hashgraph/solo-operator/internal/sequencer"
)

const (
	// NetworkOperationFinalizer is the finalizer for NetworkOperation resources
	NetworkOperationFinalizer = "operator.solo.hedera.com/networkoperation-finalizer"
)

// NetworkOperationReconciler reconciles a NetworkOperation object
type NetworkOperationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Sequencer for ordering operations within a namespace
	sequencer *sequencer.Sequencer[*operatorv1alpha1.NetworkOperation]

	// Processors for different operation types
	genesisProcessor *genesis.Task
	checkProcessor   *check.Task
	upgradeProcessor *upgrade.Processor
	nodeProcessor    *node.Processor
}

// NewNetworkOperationReconciler creates a new NetworkOperationReconciler with all processors initialized
func NewNetworkOperationReconciler(c client.Client, scheme *runtime.Scheme) *NetworkOperationReconciler {
	return &NetworkOperationReconciler{
		Client:           c,
		Scheme:           scheme,
		genesisProcessor: genesis.NewTask(c),
		checkProcessor:   check.NewTask(c),
		upgradeProcessor: upgrade.NewProcessor(c),
		nodeProcessor:    node.NewProcessor(c),
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkoperations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkoperations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkoperations/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for NetworkOperation resources
func (r *NetworkOperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting NetworkOperation reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished NetworkOperation reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Initialize sequencer if not done
	if r.sequencer == nil {
		r.sequencer = sequencer.NewSequencer(func() []*operatorv1alpha1.NetworkOperation {
			return r.listActiveOperations(ctx)
		})
	}

	// Fetch the NetworkOperation instance
	netOp := &operatorv1alpha1.NetworkOperation{}
	if err := r.Get(ctx, req.NamespacedName, netOp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("NetworkOperation resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get NetworkOperation")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !netOp.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, netOp)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(netOp, NetworkOperationFinalizer) {
		controllerutil.AddFinalizer(netOp, NetworkOperationFinalizer)
		if err := r.Update(ctx, netOp); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip if already completed or failed
	if netOp.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		netOp.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	// Check sequencer - only process if first in queue
	seqRequest := sequencer.NewRequest(netOp)
	seqResponse := r.sequencer.CheckNext(seqRequest)

	if !seqResponse.FirstInQueue {
		logger.Info("Not first in queue, requeuing",
			"namespace", netOp.Namespace,
			"name", netOp.Name,
			"delay", seqResponse.ReconcileDelay)
		return ctrl.Result{RequeueAfter: seqResponse.ReconcileDelay}, nil
	}

	// Set phase to running if pending
	if netOp.Status.Phase == "" || netOp.Status.Phase == operatorv1alpha1.PhasePending {
		netOp.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, netOp); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Process based on mode
	var result ctrl.Result
	var err error

	switch netOp.Spec.Mode {
	case operatorv1alpha1.NetworkOperationModeNetworkGenesis, operatorv1alpha1.NetworkOperationModeFirstInitialization:
		result, err = r.reconcileGenesis(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeNetworkCheck:
		result, err = r.reconcileCheck(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeNetworkUpgrade:
		result, err = r.reconcileUpgrade(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeNodeAdd:
		result, err = r.reconcileNodeAdd(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeNodeDelete:
		result, err = r.reconcileNodeDelete(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeNodeUpdate:
		result, err = r.reconcileNodeUpdate(ctx, netOp)
	case operatorv1alpha1.NetworkOperationModeProxyUpgrade:
		result, err = r.reconcileProxyUpgrade(ctx, netOp)
	default:
		logger.Error(nil, "Unknown operation mode", "mode", netOp.Spec.Mode)
		return r.markFailed(ctx, netOp, fmt.Sprintf("Unknown operation mode: %s", netOp.Spec.Mode))
	}

	if err != nil {
		return result, err
	}

	// If completed, mark as completed in sequencer
	if netOp.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		if _, err := r.sequencer.MarkAsCompleted(seqRequest); err != nil {
			logger.Error(err, "Failed to mark as completed in sequencer")
		}
	}

	return result, nil
}

// handleDeletion handles the deletion of a NetworkOperation resource
func (r *NetworkOperationReconciler) handleDeletion(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(netOp, NetworkOperationFinalizer) {
		logger.Info("Deleting NetworkOperation",
			"namespace", netOp.Namespace,
			"name", netOp.Name)

		// Mark as deleted in sequencer
		if r.sequencer != nil {
			r.sequencer.MarkAsDeleted(sequencer.NewRequest(netOp))
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(netOp, NetworkOperationFinalizer)
		if err := r.Update(ctx, netOp); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileGenesis handles the NetworkGenesis operation
func (r *NetworkOperationReconciler) reconcileGenesis(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing NetworkGenesis operation")

	// Execute genesis processor
	result, err := r.genesisProcessor.Execute(ctx, netOp)
	if err != nil {
		logger.Error(err, "Genesis processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("Genesis operation failed: %v", err))
	}

	// Update status from processor result
	if err := r.updateStatusFromGenesis(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	// Check if all steps completed
	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, "Genesis operation failed")
	}

	// Continue processing
	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// updateStatusFromGenesis updates the NetworkOperation status from genesis processor result
func (r *NetworkOperationReconciler) updateStatusFromGenesis(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation, status *genesis.GenesisStatus) error {
	// Convert genesis conditions to metav1.Condition
	conditions := make([]metav1.Condition, len(status.Conditions))
	for i, c := range status.Conditions {
		conditions[i] = metav1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		}
	}
	netOp.Status.Conditions = conditions
	netOp.Status.Phase = status.Phase
	netOp.Status.ConsensusCapsules = status.ConsensusCapsules

	return r.Status().Update(ctx, netOp)
}

// reconcileCheck handles the NetworkCheck operation
func (r *NetworkOperationReconciler) reconcileCheck(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing NetworkCheck operation")

	// Execute check processor
	result, err := r.checkProcessor.Execute(ctx, netOp)
	if err != nil {
		logger.Error(err, "Check processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("Check operation failed: %v", err))
	}

	// Update status from processor result
	if err := r.updateStatusFromCheck(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	// Check if completed
	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, "Check operation failed")
	}

	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// updateStatusFromCheck updates the NetworkOperation status from check processor result
func (r *NetworkOperationReconciler) updateStatusFromCheck(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation, status *check.CheckStatus) error {
	conditions := make([]metav1.Condition, len(status.Conditions))
	for i, c := range status.Conditions {
		conditions[i] = metav1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		}
	}
	netOp.Status.Conditions = conditions
	netOp.Status.Phase = status.Phase

	return r.Status().Update(ctx, netOp)
}

// reconcileUpgrade handles the NetworkUpgrade operation
func (r *NetworkOperationReconciler) reconcileUpgrade(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if netOp.Spec.NetworkUpgrade == nil {
		return r.markFailed(ctx, netOp, "NetworkUpgrade spec is required for upgrade operations")
	}

	logger.Info("Processing NetworkUpgrade operation",
		"workflowStep", netOp.Spec.NetworkUpgrade.WorkflowStep)

	// Execute upgrade processor based on workflow step
	result, err := r.upgradeProcessor.Process(ctx, netOp)
	if err != nil {
		logger.Error(err, "Upgrade processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("Upgrade operation failed: %v", err))
	}

	// Update status from processor result
	if err := r.updateStatusFromUpgrade(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	// Check if completed
	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, "Upgrade operation failed")
	}

	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// updateStatusFromUpgrade updates the NetworkOperation status from upgrade processor result
func (r *NetworkOperationReconciler) updateStatusFromUpgrade(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation, status *upgrade.UpgradeStatus) error {
	conditions := make([]metav1.Condition, len(status.Conditions))
	for i, c := range status.Conditions {
		conditions[i] = metav1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		}
	}
	netOp.Status.Conditions = conditions
	netOp.Status.Phase = status.Phase
	netOp.Status.SoftwareSourceStatus = status.SoftwareSourceStatus
	netOp.Status.UpgradedNodes = status.UpgradedNodes

	return r.Status().Update(ctx, netOp)
}

// reconcileNodeAdd handles the NodeAdd operation
func (r *NetworkOperationReconciler) reconcileNodeAdd(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing NodeAdd operation")

	result, err := r.nodeProcessor.ProcessAdd(netOp)
	if err != nil {
		logger.Error(err, "NodeAdd processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("NodeAdd operation failed: %v", err))
	}

	if err := r.updateStatusFromNode(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, result.Status.Message)
	}

	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// reconcileNodeDelete handles the NodeDelete operation
func (r *NetworkOperationReconciler) reconcileNodeDelete(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing NodeDelete operation")

	result, err := r.nodeProcessor.ProcessDelete(netOp)
	if err != nil {
		logger.Error(err, "NodeDelete processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("NodeDelete operation failed: %v", err))
	}

	if err := r.updateStatusFromNode(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, result.Status.Message)
	}

	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// reconcileNodeUpdate handles the NodeUpdate operation
func (r *NetworkOperationReconciler) reconcileNodeUpdate(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing NodeUpdate operation")

	result, err := r.nodeProcessor.ProcessUpdate(netOp)
	if err != nil {
		logger.Error(err, "NodeUpdate processor execution failed")
		return r.markFailed(ctx, netOp, fmt.Sprintf("NodeUpdate operation failed: %v", err))
	}

	if err := r.updateStatusFromNode(ctx, netOp, result.Status); err != nil {
		return ctrl.Result{}, err
	}

	if result.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.markSucceeded(ctx, netOp)
	}

	if result.Status.Phase == operatorv1alpha1.PhaseFailed {
		return r.markFailed(ctx, netOp, result.Status.Message)
	}

	return ctrl.Result{RequeueAfter: result.ReconcileDelay}, nil
}

// updateStatusFromNode updates the NetworkOperation status from node processor result
func (r *NetworkOperationReconciler) updateStatusFromNode(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation, status *node.NodeStatus) error {
	conditions := make([]metav1.Condition, len(status.Conditions))
	for i, c := range status.Conditions {
		conditions[i] = metav1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		}
	}
	netOp.Status.Conditions = conditions
	netOp.Status.Phase = status.Phase

	return r.Status().Update(ctx, netOp)
}

// reconcileProxyUpgrade handles the ProxyUpgrade operation
func (r *NetworkOperationReconciler) reconcileProxyUpgrade(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Processing ProxyUpgrade operation")

	// TODO: Implement proxy upgrade processor
	// - Update HAProxy/Envoy images

	return r.markSucceeded(ctx, netOp)
}

// markSucceeded marks the operation as succeeded
func (r *NetworkOperationReconciler) markSucceeded(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation) (ctrl.Result, error) {
	netOp.Status.Phase = operatorv1alpha1.PhaseSucceeded
	netOp.Status.ObservedGeneration = netOp.Generation

	now := metav1.Now()
	// Preserve existing conditions and add/update the Succeeded condition
	succeeded := metav1.Condition{
		Type:               "Succeeded",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "OperationCompleted",
		Message:            "Network operation completed successfully",
		ObservedGeneration: netOp.Generation,
	}
	r.setCondition(netOp, succeeded)

	if err := r.Status().Update(ctx, netOp); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// markFailed marks the operation as failed
func (r *NetworkOperationReconciler) markFailed(ctx context.Context, netOp *operatorv1alpha1.NetworkOperation, message string) (ctrl.Result, error) {
	netOp.Status.Phase = operatorv1alpha1.PhaseFailed
	netOp.Status.ObservedGeneration = netOp.Generation

	now := metav1.Now()
	failed := metav1.Condition{
		Type:               "Failed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "OperationFailed",
		Message:            message,
		ObservedGeneration: netOp.Generation,
	}
	r.setCondition(netOp, failed)

	if err := r.Status().Update(ctx, netOp); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setCondition adds or updates a condition in the NetworkOperation status
func (r *NetworkOperationReconciler) setCondition(netOp *operatorv1alpha1.NetworkOperation, condition metav1.Condition) {
	for i, c := range netOp.Status.Conditions {
		if c.Type == condition.Type {
			netOp.Status.Conditions[i] = condition
			return
		}
	}
	netOp.Status.Conditions = append(netOp.Status.Conditions, condition)
}

// listActiveOperations lists all active (non-completed) network operations
func (r *NetworkOperationReconciler) listActiveOperations(ctx context.Context) []*operatorv1alpha1.NetworkOperation {
	list := &operatorv1alpha1.NetworkOperationList{}
	if err := r.List(ctx, list); err != nil {
		return nil
	}

	active := make([]*operatorv1alpha1.NetworkOperation, 0)
	for i := range list.Items {
		op := &list.Items[i]
		if op.Status.Phase != operatorv1alpha1.PhaseSucceeded &&
			op.Status.Phase != operatorv1alpha1.PhaseFailed {
			active = append(active, op)
		}
	}

	return active
}

// SetupWithManager sets up the controller with the Manager
func (r *NetworkOperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize processors if not already done
	if r.genesisProcessor == nil {
		r.genesisProcessor = genesis.NewTask(r.Client)
	}
	if r.checkProcessor == nil {
		r.checkProcessor = check.NewTask(r.Client)
	}
	if r.upgradeProcessor == nil {
		r.upgradeProcessor = upgrade.NewProcessor(r.Client)
	}
	if r.nodeProcessor == nil {
		r.nodeProcessor = node.NewProcessor(r.Client)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkOperation{}).
		Named("networkoperation").
		Complete(r)
}

// Default reconcile delay when no specific delay is needed
const defaultReconcileDelay = 5 * time.Second
