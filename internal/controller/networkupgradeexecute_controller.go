// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
	consensusresources "github.com/hashgraph/solo-operator/internal/resources/consensus"
)

const (
	executePrereqRequeueAfter     = 30 * time.Second
	executeDaemonRequeueAfter     = 10 * time.Second
	executeReadyRequeueAfter      = 10 * time.Second
	executeDefaultPodReadyTimeout = 10 * time.Minute
)

type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	StrategyName    string
	PodReadyTimeout time.Duration
}

func NewNetworkUpgradeExecuteReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeExecuteReconciler {
	return &NetworkUpgradeExecuteReconciler{
		Client:          c,
		Scheme:          scheme,
		Checker:         registry.OCIChecker{},
		PodReadyTimeout: executeDefaultPodReadyTimeout,
	}
}

func (r *NetworkUpgradeExecuteReconciler) WithStrategy(name string) *NetworkUpgradeExecuteReconciler {
	r.StrategyName = name
	return r
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch

func (r *NetworkUpgradeExecuteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var execute operatorv1alpha1.NetworkUpgradeExecute
	if err := r.Get(ctx, req.NamespacedName, &execute); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if execute.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded ||
		execute.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		return ctrl.Result{}, nil
	}

	// Initialize status on first reconcile.
	if execute.Status.Phase == "" || execute.Status.Phase == operatorv1alpha1.ExecutePhasePending {
		now := metav1.Now()
		execute.Status.StartTime = &now
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, &execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing status: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	switch execute.Status.Phase {
	case operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon,
		operatorv1alpha1.ExecutePhasePendingInfraUpgrade:
		return r.reconcileWaitingForDaemon(ctx, &execute)

	case operatorv1alpha1.ExecutePhasePendingNodeUpgrade:
		return r.reconcilePendingNodeUpgrade(ctx, &execute)

	default:
		logger.Info("unhandled execute phase", "phase", execute.Status.Phase)
		return ctrl.Result{}, nil
	}
}

// reconcileWaitingForDaemon handles the ReadyForProvisionerDaemon and PendingInfraUpgrade phases.
// It first ensures prerequisites are met and the capsule is scaled down, then polls for DaemonResult.
func (r *NetworkUpgradeExecuteReconciler) reconcileWaitingForDaemon(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check prerequisites: Prepare and Freeze must both be Succeeded.
	prepare, err := r.findPrepare(ctx, execute.Namespace, execute.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradePrepare to succeed", "operationId", execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}

	freeze, err := r.findFreeze(ctx, execute.Namespace, execute.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if freeze == nil || freeze.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradeFreeze to succeed", "operationId", execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}

	// Re-validate registries every time (tampered image at execute time corrupts nodes).
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	skipUC := map[operatorv1alpha1.ComponentName]bool{operatorv1alpha1.ComponentUC: true}
	updated, registryOK := registry.RevalidateComponents(
		ctx, r.Checker,
		execute.Spec.Images, execute.Spec.SelectedSources,
		checkTimeout, rng, r.StrategyName,
		skipUC,
		func(name operatorv1alpha1.ComponentName, res registry.FailoverResult) {
			r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryFailover,
				"%s: registry %s unreachable, failed over to %s (digest: %s)",
				name, res.FailedImage, res.Selected.Image, res.Selected.Digest)
		},
		func(name operatorv1alpha1.ComponentName, err error) {
			r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: %v", name, err)
		},
	)
	if !registryOK {
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
		execute.Status.CompletionTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistrySelected,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid,
			Message: "no valid registry available for one or more components",
		})
		if err := r.Status().Update(ctx, execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status to failed: %w", err)
		}
		r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
			"registry validation failed for operationId %q", execute.Spec.OperationID)
		return ctrl.Result{}, fmt.Errorf("no valid registry for one or more components")
	}
	execute.Status.SelectedSources = updated
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondRegistrySelected,
		Status:  metav1.ConditionTrue,
		Reason:  operatorv1alpha1.ReasonSelectionComplete,
		Message: "all selected registries healthy",
	})

	// Scale down the capsule if not already done.
	capsules, err := r.listCapsules(ctx, execute.Namespace, execute.Spec.Orbit)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.ensureCapsuleOffline(ctx, execute, capsules); err != nil {
		return ctrl.Result{}, err
	}

	// Check if DaemonResult condition has been written by the provisioner daemon.
	daemonCond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondDaemonResult)

	if daemonCond == nil {
		// Daemon has not responded yet — stay in ReadyForProvisionerDaemon and poll.
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
		}
		return ctrl.Result{RequeueAfter: executeDaemonRequeueAfter}, nil
	}

	if daemonCond.Status == metav1.ConditionFalse {
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
		execute.Status.CompletionTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status to failed: %w", err)
		}
		r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
			"provisioner daemon reported failure: %s", daemonCond.Reason)
		return ctrl.Result{}, nil
	}

	// DaemonResult=True: advance to PendingNodeUpgrade.
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.ObservedGeneration = execute.Generation
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("advancing to PendingNodeUpgrade: %w", err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventPendingNodeUpgrade,
		"provisioner daemon succeeded; advancing to PendingNodeUpgrade for operationId %q", execute.Spec.OperationID)
	return ctrl.Result{Requeue: true}, nil
}

// reconcilePendingNodeUpgrade patches the ConsensusCapsule and monitors pod readiness.
func (r *NetworkUpgradeExecuteReconciler) reconcilePendingNodeUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsules, err := r.listCapsules(ctx, execute.Namespace, execute.Spec.Orbit)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Patch capsules that haven't been patched yet (idempotent: skip already-patched ones).
	capsulePatched := findCondition(execute.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if capsulePatched == nil || capsulePatched.Status != metav1.ConditionTrue {
		if err := r.patchCapsules(ctx, execute, capsules); err != nil {
			return ctrl.Result{}, err
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondConsensusCapsulePatched,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPatchApplied,
			Message: "ConsensusCapsule images updated and brought online",
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("updating status after capsule patch: %w", err)
		}
		r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulePatched,
			"ConsensusCapsule patched with new images for operationId %q", execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	// Capsule already patched — check if StatefulSet pods are ready.
	allReady, timedOut, err := r.checkPodsReady(ctx, execute, capsules)
	if err != nil {
		return ctrl.Result{}, err
	}

	if timedOut {
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
		execute.Status.CompletionTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondNodesUpgraded,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonUpgradeFailed,
			Message: "timed out waiting for consensus pods to become ready",
		})
		if err := r.Status().Update(ctx, execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status to failed: %w", err)
		}
		r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
			"timed out waiting for consensus pods after upgrade for operationId %q", execute.Spec.OperationID)
		return ctrl.Result{}, nil
	}

	if !allReady {
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	// All pods ready — upgrade succeeded.
	now := metav1.Now()
	execute.Status.Phase = operatorv1alpha1.ExecutePhaseSucceeded
	execute.Status.CompletionTime = &now
	execute.Status.ObservedGeneration = execute.Generation
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondNodesUpgraded,
		Status:  metav1.ConditionTrue,
		Reason:  operatorv1alpha1.ReasonUpgradeSucceeded,
		Message: "all consensus pods ready with upgraded images",
	})
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("updating status to succeeded: %w", err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventUpgradeSucceeded,
		"upgrade succeeded for operationId %q", execute.Spec.OperationID)
	return ctrl.Result{}, nil
}

// ensureCapsuleOffline sets Offline=true on any capsule in the orbit that is not already offline.
func (r *NetworkUpgradeExecuteReconciler) ensureCapsuleOffline(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsules []operatorv1alpha1.ConsensusCapsule,
) error {
	for i := range capsules {
		c := &capsules[i]
		if c.Spec.Offline {
			continue
		}
		patch := client.MergeFrom(c.DeepCopy())
		c.Spec.Offline = true
		if err := r.Patch(ctx, c, patch); err != nil {
			return fmt.Errorf("scaling down capsule %s: %w", c.Name, err)
		}
		r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventPodScaledDown,
			"scaled down ConsensusCapsule %s/%s for operationId %q", c.Namespace, c.Name, execute.Spec.OperationID)
	}
	return nil
}

// patchCapsules applies the image upgrade (three-rule) and sets Offline=false atomically.
func (r *NetworkUpgradeExecuteReconciler) patchCapsules(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsules []operatorv1alpha1.ConsensusCapsule,
) error {
	for i := range capsules {
		c := &capsules[i]
		patch := client.MergeFrom(c.DeepCopy())

		r.applyImagePatch(execute, c)
		c.Spec.Offline = false

		if err := r.Patch(ctx, c, patch); err != nil {
			return fmt.Errorf("patching capsule %s: %w", c.Name, err)
		}
	}
	return nil
}

// applyImagePatch mutates the capsule in place using the three-rule logic:
//   - present + enabled  → upgrade image
//   - omitted            → no-change
//   - present + disabled → remove sidecar (nil pointer)
func (r *NetworkUpgradeExecuteReconciler) applyImagePatch(
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsule *operatorv1alpha1.ConsensusCapsule,
) {
	images := execute.Spec.Images
	selected := execute.Status.SelectedSources

	applyComponent := func(name operatorv1alpha1.ComponentName) (image string, remove bool, upgrade bool) {
		comp, present := images.Component(name)
		if !present {
			return "", false, false
		}
		if comp.Enabled != nil && !*comp.Enabled {
			return "", true, false
		}
		src := selected.Get(name)
		if src == nil {
			return "", false, false
		}
		return src.Image, false, true
	}

	// consensusNode — always present, never a pointer.
	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentConsensusNode); upgrade && !remove {
		setSoftwareVersion(&capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion, img)
	}

	// recordStreamUploader
	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentRecordStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.RecordsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.RecordsUploader, img)
	}

	// eventStreamUploader
	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentEventStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.EventsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.EventsUploader, img)
	}

	// blockStreamUploader
	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBlockStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.BlocksUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.BlocksUploader, img)
	}

	// backupUploader
	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBackupUploader); remove {
		capsule.Spec.PodProperties.Containers.BackupUploader = nil
	} else if upgrade {
		ensureStream(&capsule.Spec.PodProperties.Containers.BackupUploader, img)
	}

	// ComponentUC — not yet wired into ConsensusContainers; skip until added.
}

// checkPodsReady inspects StatefulSet.Status.ReadyReplicas for all capsules.
func (r *NetworkUpgradeExecuteReconciler) checkPodsReady(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsules []operatorv1alpha1.ConsensusCapsule,
) (allReady, timedOut bool, err error) {
	timeout := r.PodReadyTimeout
	if timeout == 0 {
		timeout = executeDefaultPodReadyTimeout
	}

	if execute.Status.StartTime != nil && time.Since(execute.Status.StartTime.Time) > timeout {
		return false, true, nil
	}

	for i := range capsules {
		c := &capsules[i]
		var sts appsv1.StatefulSet
		if err := r.Get(ctx, client.ObjectKey{
			Name:      consensusresources.StatefulSetName(c),
			Namespace: c.Namespace,
		}, &sts); err != nil {
			if apierrors.IsNotFound(err) {
				return false, false, nil
			}
			return false, false, fmt.Errorf("getting StatefulSet for capsule %s: %w", c.Name, err)
		}
		if sts.Status.ReadyReplicas < 1 {
			return false, false, nil
		}
	}
	return true, false, nil
}

func (r *NetworkUpgradeExecuteReconciler) listCapsules(ctx context.Context, namespace, orbit string) ([]operatorv1alpha1.ConsensusCapsule, error) {
	var list operatorv1alpha1.ConsensusCapsuleList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels{common.LabelOrbit: orbit},
	); err != nil {
		return nil, fmt.Errorf("listing ConsensusCapsules for orbit %q: %w", orbit, err)
	}
	return list.Items, nil
}

func (r *NetworkUpgradeExecuteReconciler) findPrepare(ctx context.Context, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradePrepare, error) {
	var list operatorv1alpha1.NetworkUpgradePrepareList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return nil, fmt.Errorf("listing NetworkUpgradePrepare: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradePrepare for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *NetworkUpgradeExecuteReconciler) findFreeze(ctx context.Context, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradeFreeze, error) {
	var list operatorv1alpha1.NetworkUpgradeFreezeList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return nil, fmt.Errorf("listing NetworkUpgradeFreeze: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradeFreeze for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}

func (r *NetworkUpgradeExecuteReconciler) eventf(obj *operatorv1alpha1.NetworkUpgradeExecute, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "NetworkUpgradeExecute", format, args...)
	}
}

func (r *NetworkUpgradeExecuteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradeexecute")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	// Register field index on NetworkUpgradeFreeze — not yet registered by any other controller.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradeFreeze{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreeze).Spec.OperationID}
		},
	); err != nil {
		return fmt.Errorf("registering operationId index on NetworkUpgradeFreeze: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeExecute{}).
		Named("networkupgradeexecute").
		Complete(r)
}

// ensureCheetah sets the image on a *CheetahUploader, allocating if nil.
func ensureCheetah(ptr **operatorv1alpha1.CheetahUploader, image string) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.CheetahUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image)
}

// ensureStream sets the image on a *StreamUploader, allocating if nil.
func ensureStream(ptr **operatorv1alpha1.StreamUploader, image string) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.StreamUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image)
}

// setSoftwareVersion allocates and populates a SoftwareVersion from a full OCI image reference.
func setSoftwareVersion(sv **operatorv1alpha1.SoftwareVersion, image string) {
	if *sv == nil {
		*sv = &operatorv1alpha1.SoftwareVersion{}
	}
	(*sv).Repository, (*sv).ImageName, (*sv).ImageTag = common.ParseImage(image)
}
