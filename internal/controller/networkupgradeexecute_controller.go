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
	"k8s.io/apimachinery/pkg/types"
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

	if ok, msg := validateOperationIDLabel(execute.Labels, execute.Spec.OperationID); !ok {
		return rejectInvalidOperationIDLabel(ctx, r.Client, &execute, &execute.Status.Conditions,
			func(reason, m string) { r.eventf(&execute, corev1.EventTypeWarning, reason, "%s", m) },
			func() { execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed },
			msg)
	}

	if execute.Status.Phase == "" || execute.Status.Phase == operatorv1alpha1.ExecutePhasePending {
		sibling, err := executeSiblingExists(ctx, r.Client, execute.Namespace, string(execute.UID), execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if sibling {
			msg := fmt.Sprintf("operationId %q already used by another NetworkUpgradeExecute", execute.Spec.OperationID)
			r.eventf(&execute, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
			execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
			execute.Status.ObservedGeneration = execute.Generation
			setCondition(&execute.Status.Conditions, metav1.Condition{
				Type:               operatorv1alpha1.CondManifestValidated,
				Status:             metav1.ConditionFalse,
				Reason:             operatorv1alpha1.ReasonDuplicateOpID,
				Message:            msg,
				ObservedGeneration: execute.Generation,
			})
			if err := r.Status().Update(ctx, &execute); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
			}
			return ctrl.Result{}, fmt.Errorf("%s", msg)
		}

		prepare, err := findPrepare(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			logger.Info("waiting for NetworkUpgradePrepare to succeed", "operationId", execute.Spec.OperationID)
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		freeze, err := findFreeze(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if freeze == nil || freeze.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			logger.Info("waiting for NetworkUpgradeFreeze to succeed", "operationId", execute.Spec.OperationID)
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		capsule, err := r.getCapsule(ctx, &execute)
		if err != nil {
			return ctrl.Result{}, err
		}
		if capsule == nil {
			logger.Info("ConsensusCapsule not found", "capsule", execute.Spec.ConsensusNode)
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		if err := r.ensureCapsuleOffline(ctx, &execute, capsule); err != nil {
			return ctrl.Result{}, err
		}

		now := metav1.Now()
		execute.Status.StartTime = &now
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, &execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing status: %w", err)
		}
		r.eventf(&execute, corev1.EventTypeNormal, operatorv1alpha1.EventReadyForDaemon,
			"waiting for provisioner daemon for operationId %q", execute.Spec.OperationID)
		return ctrl.Result{Requeue: true}, nil
	}

	switch execute.Status.Phase {
	case operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon:
		return r.reconcileWaitingForDaemon(ctx, &execute)

	case operatorv1alpha1.ExecutePhasePendingInfraUpgrade:
		return r.reconcilePendingInfraUpgrade(ctx, &execute)

	case operatorv1alpha1.ExecutePhasePendingNodeUpgrade:
		return r.reconcilePendingNodeUpgrade(ctx, &execute)

	default:
		logger.Info("unhandled execute phase", "phase", execute.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *NetworkUpgradeExecuteReconciler) reconcileWaitingForDaemon(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	daemonCond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondDaemonResult)

	if daemonCond == nil {
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

	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingInfraUpgrade
	execute.Status.ObservedGeneration = execute.Generation
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("advancing to PendingInfraUpgrade: %w", err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventPendingNodeUpgrade,
		"provisioner daemon succeeded; advancing to PendingInfraUpgrade for operationId %q", execute.Spec.OperationID)
	return ctrl.Result{Requeue: true}, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcilePendingInfraUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	// TODO(#943): populate entries from spec.configCRs once that field is added to
	// NetworkUpgradeExecuteSpec. Until then the map is empty and this phase is a
	// no-op pass-through, preserving the phase transition skeleton so future work
	// only needs to populate the map.
	entries := make(map[string]configCREntry)

	done, err := r.reconcileConfigCRs(ctx, execute, entries)
	if err != nil {
		r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventConfigCRFailed,
			"config CR reconciliation failed for operationId %q: %v", execute.Spec.OperationID, err)
		return ctrl.Result{}, err
	}
	if !done {
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}

	now := metav1.Now()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.NodeUpgradeStartTime = &now
	execute.Status.ObservedGeneration = execute.Generation
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondConfigCRsApplied,
		Status:             metav1.ConditionTrue,
		Reason:             "ConfigCRsApplied",
		Message:            "all config CRs applied for this operation",
		ObservedGeneration: execute.Generation,
	})
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("advancing to PendingNodeUpgrade: %w", err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventPendingNodeUpgrade,
		"config CRs applied; advancing to PendingNodeUpgrade for operationId %q", execute.Spec.OperationID)
	return ctrl.Result{Requeue: true}, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcilePendingNodeUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsule, err := r.getCapsule(ctx, execute)
	if err != nil {
		return ctrl.Result{}, err
	}
	if capsule == nil {
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	capsulePatched := findCondition(execute.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if capsulePatched == nil || capsulePatched.Status != metav1.ConditionTrue {
		allOffline, err := r.allPeersOffline(ctx, execute)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !allOffline {
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

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

		patch := client.MergeFrom(capsule.DeepCopy())
		r.applyImagePatch(execute, capsule)
		if err := r.Patch(ctx, capsule, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching capsule %s: %w", capsule.Name, err)
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondConsensusCapsulePatched,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPatchApplied,
			Message: "ConsensusCapsule images updated",
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("updating status after capsule patch: %w", err)
		}
		r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulePatched,
			"ConsensusCapsule %s patched with new images for operationId %q", capsule.Name, execute.Spec.OperationID)
		return ctrl.Result{Requeue: true}, nil
	}

	capsulesOnline := findCondition(execute.Status.Conditions, operatorv1alpha1.CondCapsulesOnline)
	if capsulesOnline == nil || capsulesOnline.Status != metav1.ConditionTrue {
		allPatched, err := r.allPeersPatched(ctx, execute)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !allPatched {
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		if capsule.Spec.Offline {
			patch := client.MergeFrom(capsule.DeepCopy())
			capsule.Spec.Offline = false
			if err := r.Patch(ctx, capsule, patch); err != nil {
				return ctrl.Result{}, fmt.Errorf("bringing capsule %s online: %w", capsule.Name, err)
			}
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondCapsulesOnline,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonCapsulesOnline,
			Message: "ConsensusCapsule brought online",
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("updating status after bringing capsule online: %w", err)
		}
		r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulesOnline,
			"ConsensusCapsule %s brought online for operationId %q", capsule.Name, execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	ready, timedOut, err := r.checkPodReady(ctx, execute, capsule)
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
			Message: "timed out waiting for consensus pod to become ready",
		})
		if err := r.Status().Update(ctx, execute); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status to failed: %w", err)
		}
		r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
			"timed out waiting for consensus pod after upgrade for operationId %q", execute.Spec.OperationID)
		return ctrl.Result{}, nil
	}

	if !ready {
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	now := metav1.Now()
	execute.Status.Phase = operatorv1alpha1.ExecutePhaseSucceeded
	execute.Status.CompletionTime = &now
	execute.Status.ObservedGeneration = execute.Generation
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondNodesUpgraded,
		Status:  metav1.ConditionTrue,
		Reason:  operatorv1alpha1.ReasonUpgradeSucceeded,
		Message: "consensus pod ready with upgraded images",
	})
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("updating status to succeeded: %w", err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventUpgradeSucceeded,
		"upgrade succeeded for operationId %q capsule %s", execute.Spec.OperationID, capsule.Name)
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeExecuteReconciler) allPeersOffline(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := r.List(ctx, &list,
		client.InNamespace(execute.Namespace),
		client.MatchingFields{fieldOperationID: execute.Spec.OperationID},
	); err != nil {
		return false, fmt.Errorf("listing Execute CRs for operationId %q: %w", execute.Spec.OperationID, err)
	}
	for i := range list.Items {
		sibling := &list.Items[i]
		if sibling.Spec.ConsensusNode == "" {
			continue
		}
		var capsule operatorv1alpha1.ConsensusCapsule
		err := r.Get(ctx, types.NamespacedName{Name: sibling.Spec.ConsensusNode, Namespace: sibling.Namespace}, &capsule)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("getting capsule %s: %w", sibling.Spec.ConsensusNode, err)
		}
		if !capsule.Spec.Offline {
			return false, nil
		}
	}
	return true, nil
}

func (r *NetworkUpgradeExecuteReconciler) allPeersPatched(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := r.List(ctx, &list,
		client.InNamespace(execute.Namespace),
		client.MatchingFields{fieldOperationID: execute.Spec.OperationID},
	); err != nil {
		return false, fmt.Errorf("listing Execute CRs for operationId %q: %w", execute.Spec.OperationID, err)
	}
	for i := range list.Items {
		sibling := &list.Items[i]
		if sibling.UID == execute.UID {
			continue
		}
		cond := findCondition(sibling.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
		if cond == nil || cond.Status != metav1.ConditionTrue {
			return false, nil
		}
	}
	return true, nil
}

func (r *NetworkUpgradeExecuteReconciler) getCapsule(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (*operatorv1alpha1.ConsensusCapsule, error) {
	var capsule operatorv1alpha1.ConsensusCapsule
	err := r.Get(ctx, types.NamespacedName{Name: execute.Spec.ConsensusNode, Namespace: execute.Namespace}, &capsule)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting ConsensusCapsule %s: %w", execute.Spec.ConsensusNode, err)
	}
	return &capsule, nil
}

func (r *NetworkUpgradeExecuteReconciler) ensureCapsuleOffline(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsule *operatorv1alpha1.ConsensusCapsule,
) error {
	if capsule.Spec.Offline {
		return nil
	}
	patch := client.MergeFrom(capsule.DeepCopy())
	capsule.Spec.Offline = true
	if err := r.Patch(ctx, capsule, patch); err != nil {
		return fmt.Errorf("scaling down capsule %s: %w", capsule.Name, err)
	}
	r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventPodScaledDown,
		"scaled down ConsensusCapsule %s/%s for operationId %q", capsule.Namespace, capsule.Name, execute.Spec.OperationID)
	return nil
}

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

	if img, _, upgrade := applyComponent(operatorv1alpha1.ComponentConsensusNode); upgrade {
		setSoftwareVersion(&capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion, img)
	}

	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentRecordStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.RecordsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.RecordsUploader, img)
	}

	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentEventStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.EventsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.EventsUploader, img)
	}

	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBlockStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.BlocksUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.BlocksUploader, img)
	}

	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBackupUploader); remove {
		capsule.Spec.PodProperties.Containers.BackupUploader = nil
	} else if upgrade {
		ensureStream(&capsule.Spec.PodProperties.Containers.BackupUploader, img)
	}

	if img, remove, upgrade := applyComponent(operatorv1alpha1.ComponentUC); remove {
		capsule.Spec.PodProperties.Containers.UC = nil
	} else if upgrade {
		ensureUC(&capsule.Spec.PodProperties.Containers.UC, img)
	}

}

func (r *NetworkUpgradeExecuteReconciler) checkPodReady(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsule *operatorv1alpha1.ConsensusCapsule,
) (ready, timedOut bool, err error) {
	timeout := r.PodReadyTimeout
	if timeout == 0 {
		timeout = executeDefaultPodReadyTimeout
	}
	if execute.Status.NodeUpgradeStartTime != nil && time.Since(execute.Status.NodeUpgradeStartTime.Time) > timeout {
		return false, true, nil
	}

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKey{
		Name:      consensusresources.StatefulSetName(capsule),
		Namespace: capsule.Namespace,
	}, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("getting StatefulSet for capsule %s: %w", capsule.Name, err)
	}
	return sts.Status.ReadyReplicas >= 1, false, nil
}

func (r *NetworkUpgradeExecuteReconciler) eventf(obj *operatorv1alpha1.NetworkUpgradeExecute, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "NetworkUpgradeExecute", format, args...)
	}
}

func RegisterFreezeIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradeFreeze{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeFreeze).Spec.OperationID}
		},
	)
}

func RegisterExecuteIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradeExecute{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradeExecute).Spec.OperationID}
		},
	)
}

func (r *NetworkUpgradeExecuteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradeexecute")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeExecute{}).
		Named("networkupgradeexecute").
		Complete(r)
}

func ensureCheetah(ptr **operatorv1alpha1.CheetahUploader, image string) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.CheetahUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image)
}

func ensureStream(ptr **operatorv1alpha1.StreamUploader, image string) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.StreamUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image)
}

func ensureUC(ptr **operatorv1alpha1.UcSidecar, image string) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.UcSidecar{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image)
}

func setSoftwareVersion(sv **operatorv1alpha1.SoftwareVersion, image string) {
	if *sv == nil {
		*sv = &operatorv1alpha1.SoftwareVersion{}
	}
	(*sv).Repository, (*sv).ImageName, (*sv).ImageTag = common.ParseImage(image)
}
