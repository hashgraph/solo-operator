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

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/registry"
	consensusresources "github.com/hashgraph/solo-operator/internal/resources/consensus"
)

const (
	executePrereqRequeueAfter = 30 * time.Second
	executeDaemonRequeueAfter = 10 * time.Second
	executeReadyRequeueAfter  = 10 * time.Second
	// executeDefaultNodeActiveTimeout is the backstop for a node to reach platform
	// status ACTIVE after an upgrade, anchored to nodeUpgradeStartTime. It covers a
	// dead/missing UC that never advances the capsule phase. Reaching ACTIVE can
	// legitimately take much longer than pod-readiness (event replay, reconnect,
	// state loading), so it defaults generously and is operator-tunable via
	// --node-active-timeout.
	executeDefaultNodeActiveTimeout = 30 * time.Minute
)

type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	StrategyName string
	// NodeActiveTimeout bounds how long to wait for the consensus node to reach
	// platform status ACTIVE after an upgrade before failing (the backstop).
	NodeActiveTimeout time.Duration
}

func NewNetworkUpgradeExecuteReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeExecuteReconciler {
	return &NetworkUpgradeExecuteReconciler{
		Client:            c,
		Scheme:            scheme,
		Checker:           registry.OCIChecker{},
		NodeActiveTimeout: executeDefaultNodeActiveTimeout,
	}
}

func (r *NetworkUpgradeExecuteReconciler) WithStrategy(name string) *NetworkUpgradeExecuteReconciler {
	r.StrategyName = name
	return r
}

// WithNodeActiveTimeout overrides the backstop deadline for a node to reach
// platform status ACTIVE after an upgrade. A non-positive value keeps the default.
func (r *NetworkUpgradeExecuteReconciler) WithNodeActiveTimeout(d time.Duration) *NetworkUpgradeExecuteReconciler {
	if d > 0 {
		r.NodeActiveTimeout = d
	}
	return r
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezes,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch

func (r *NetworkUpgradeExecuteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var execute operatorv1alpha1.NetworkUpgradeExecute
	if err := r.Get(ctx, req.NamespacedName, &execute); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradeexecute", "NetworkUpgradeExecute", &execute)

	if res, err := ensureFinalizer(ctx, r.Client, &execute, FinalizerNetworkUpgradeExecute); res != nil {
		return *res, err
	}

	if execute.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded ||
		execute.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		return ctrl.Result{}, nil
	}

	if ok, msg := validateOperationIDLabel(execute.Labels, execute.Spec.OperationID); !ok {
		return rejectInvalidOperationIDLabel(ctx, r.Client, &execute, &execute.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &execute, corev1.EventTypeWarning, reason, "%s", m) },
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
			r.eventf(ctx, &execute, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
			execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
			execute.Status.ObservedGeneration = execute.Generation
			setCondition(&execute.Status.Conditions, metav1.Condition{
				Type:               operatorv1alpha1.CondManifestValidated,
				Status:             metav1.ConditionFalse,
				Reason:             operatorv1alpha1.ReasonDuplicateOpID.String(),
				Message:            msg,
				ObservedGeneration: execute.Generation,
			})
			if err := r.Status().Update(ctx, &execute); err != nil {
				return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
			}
			return ctrl.Result{}, errx.WithReason(errReconcile.New("%s", msg), ReasonManifestInvalid)
		}

		prepare, err := findPrepare(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			common.Info(ctx, ReasonWaitingForPrepare).Msg("waiting for NetworkUpgradePrepare to succeed")
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		freeze, err := findFreeze(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if freeze == nil || freeze.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			common.Info(ctx, ReasonWaitingForFreeze).Msg("waiting for NetworkUpgradeFreeze to succeed")
			return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
		}

		capsule, err := r.getCapsule(ctx, &execute)
		if err != nil {
			return ctrl.Result{}, err
		}
		if capsule == nil {
			common.Info(ctx, ReasonCapsuleNotFound).Str("capsule", execute.Spec.ConsensusNode).Msg("ConsensusCapsule not found")
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
			return ctrl.Result{}, wrapStatusInit(err)
		}
		r.eventf(ctx, &execute, corev1.EventTypeNormal, operatorv1alpha1.EventReadyForDaemon,
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
		common.Info(ctx, ReasonUnhandledPhase).Str("phase", string(execute.Status.Phase)).Msg("unhandled execute phase")
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
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
		}
		return ctrl.Result{RequeueAfter: executeDaemonRequeueAfter}, nil
	}

	if daemonCond.Status == metav1.ConditionFalse {
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
		execute.Status.CompletionTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status to failed")
		}
		r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
			"provisioner daemon reported failure: %s", daemonCond.Reason)
		return ctrl.Result{}, nil
	}

	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingInfraUpgrade
	execute.Status.ObservedGeneration = execute.Generation
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, wrapStatusUpdate(err, "advancing to PendingInfraUpgrade")
	}
	r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventPendingInfraUpgrade,
		"provisioner daemon succeeded; advancing to PendingInfraUpgrade for operationId %q", execute.Spec.OperationID)
	return ctrl.Result{Requeue: true}, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcilePendingInfraUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	cond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondConfigCRsApplied)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		common.Info(ctx, ReasonWaitingForConfigCRsApplied).Msg("waiting for daemon to set CondConfigCRsApplied=True")
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}

	now := metav1.Now()
	execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
	execute.Status.NodeUpgradeStartTime = &now
	execute.Status.ObservedGeneration = execute.Generation
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, wrapStatusUpdate(err, "advancing to PendingNodeUpgrade")
	}
	r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventPendingNodeUpgrade,
		"daemon applied config CRs; advancing to PendingNodeUpgrade for operationId %q", execute.Spec.OperationID)
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
				r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryFailover,
					"%s: registry %s unreachable, failed over to %s (digest: %s)",
					name, res.FailedImage, res.Selected.Image, res.Selected.Digest)
			},
			func(name operatorv1alpha1.ComponentName, err error) {
				r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
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
				Reason:  operatorv1alpha1.ReasonRegistryInvalid.String(),
				Message: "no valid registry available for one or more components",
			})
			if err := r.Status().Update(ctx, execute); err != nil {
				return ctrl.Result{}, wrapStatusUpdate(err, "updating status to failed")
			}
			r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed,
				"registry validation failed for operationId %q", execute.Spec.OperationID)
			return ctrl.Result{}, errx.WithHints(
				errx.WithReason(errReconcile.New("no valid registry for one or more components"), ReasonNoValidRegistry),
				"Confirm at least one configured registry serves a valid image for each component")
		}
		execute.Status.SelectedSources = updated
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistrySelected,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonSelectionComplete.String(),
			Message: "all selected registries healthy",
		})

		patch := client.MergeFrom(capsule.DeepCopy())
		r.applyImagePatch(execute, capsule)
		if err := r.Patch(ctx, capsule, patch); err != nil {
			return ctrl.Result{}, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "patching capsule %s", capsule.Name), ReasonPatchFailed),
				"Confirm the operator has patch permission on consensuscapsules")
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondConsensusCapsulePatched,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPatchApplied.String(),
			Message: "ConsensusCapsule images updated",
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status after capsule patch")
		}
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulePatched,
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
				return ctrl.Result{}, errx.WithHints(
					errx.WithReason(errK8sAPI.Wrap(err, "bringing capsule %s online", capsule.Name), ReasonScaleFailed),
					"Confirm the operator has patch permission on consensuscapsules")
			}
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondCapsulesOnline,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonCapsulesOnline.String(),
			Message: "ConsensusCapsule brought online",
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status after bringing capsule online")
		}
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulesOnline,
			"ConsensusCapsule %s brought online for operationId %q", capsule.Name, execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}

	outcome, err := r.checkUpgradeGate(ctx, execute, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch outcome {
	case upgradeTimedOut:
		return r.failUpgrade(ctx, execute,
			"timed out waiting for consensus node to reach platform status ACTIVE",
			fmt.Sprintf("timed out waiting for consensus node ACTIVE after upgrade for operationId %q", execute.Spec.OperationID))

	case upgradeCatastrophic:
		return r.failUpgrade(ctx, execute,
			fmt.Sprintf("consensus node reported platform status %s after upgrade", operatorv1alpha1.PlatformStatusCatastrophicFailure),
			fmt.Sprintf("consensus node %s reported CATASTROPHIC_FAILURE after upgrade for operationId %q", capsule.Name, execute.Spec.OperationID))

	case upgradeSucceeded:
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseSucceeded
		execute.Status.CompletionTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondNodesUpgraded,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonUpgradeSucceeded.String(),
			Message: "consensus node reached platform status ACTIVE on the post-upgrade revision",
		})
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status to succeeded")
		}
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventUpgradeSucceeded,
			"upgrade succeeded for operationId %q capsule %s", execute.Spec.OperationID, capsule.Name)
		return ctrl.Result{}, nil

	default: // upgradeWaiting — the consensus node has not yet reached ACTIVE.
		return ctrl.Result{RequeueAfter: executeReadyRequeueAfter}, nil
	}
}

// failUpgrade drives the Execute CR to a terminal Failed phase with a
// NodesUpgraded=False condition and a UpgradeFailed warning event. The reconciler
// is the sole writer of Execute.status.phase.
func (r *NetworkUpgradeExecuteReconciler) failUpgrade(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	condMsg, eventMsg string,
) (ctrl.Result, error) {
	now := metav1.Now()
	execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
	execute.Status.CompletionTime = &now
	execute.Status.ObservedGeneration = execute.Generation
	setCondition(&execute.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondNodesUpgraded,
		Status:  metav1.ConditionFalse,
		Reason:  operatorv1alpha1.ReasonUpgradeFailed.String(),
		Message: condMsg,
	})
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, wrapStatusUpdate(err, "updating status to failed")
	}
	r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventUpgradeFailed, "%s", eventMsg)
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeExecuteReconciler) allPeersOffline(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := r.List(ctx, &list,
		client.InNamespace(execute.Namespace),
		client.MatchingFields{fieldOperationID: execute.Spec.OperationID},
	); err != nil {
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing Execute CRs for operationId %q", execute.Spec.OperationID), ReasonListFailed),
			"Confirm the operator has list permission on networkupgradeexecutes.operator.solo.hiero.org")
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
			return false, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "getting capsule %s", sibling.Spec.ConsensusNode), ReasonGetFailed),
				"Confirm the operator has get permission on consensuscapsules")
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
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing Execute CRs for operationId %q", execute.Spec.OperationID), ReasonListFailed),
			"Confirm the operator has list permission on networkupgradeexecutes.operator.solo.hiero.org")
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
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting ConsensusCapsule %s", execute.Spec.ConsensusNode), ReasonGetFailed),
			"Confirm the operator has get permission on consensuscapsules")
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
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "scaling down capsule %s", capsule.Name), ReasonScaleFailed),
			"Confirm the operator has patch permission on consensuscapsules")
	}
	r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventPodScaledDown,
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

// upgradeOutcome is the terminal-state evaluation of the post-upgrade consensus
// node produced by checkUpgradeGate.
type upgradeOutcome int

const (
	// upgradeWaiting means the node has not yet reached a terminal state.
	upgradeWaiting upgradeOutcome = iota
	// upgradeSucceeded means the node reached ACTIVE on the post-upgrade revision.
	upgradeSucceeded
	// upgradeCatastrophic means the node reported CATASTROPHIC_FAILURE on the
	// post-upgrade revision (fast fail).
	upgradeCatastrophic
	// upgradeTimedOut means the backstop deadline elapsed (covers a dead/missing UC).
	upgradeTimedOut
)

// checkUpgradeGate evaluates whether the post-upgrade consensus node has reached a
// terminal state. Success is gated on the ConsensusCapsule reaching phase==Active
// (written by the operator only when the node's live platform status is ACTIVE),
// not on mere pod readiness.
//
// The stale-reading guard is StatefulSet-revision-based: the capsule phase and
// platformStatus are trusted only once the StatefulSet has rolled to its
// post-upgrade template. The ConsensusCapsule generation alone cannot distinguish
// an old-pod ACTIVE reading observed mid-roll, so the guard reads the STS revision
// directly.
//
//   - Active on the post-upgrade revision                → upgradeSucceeded
//   - CATASTROPHIC_FAILURE on the new revision           → upgradeCatastrophic (fast fail)
//   - NodeActiveTimeout (default 30m) from start elapsed → upgradeTimedOut (dead/missing UC)
//   - otherwise                                          → upgradeWaiting
func (r *NetworkUpgradeExecuteReconciler) checkUpgradeGate(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsule *operatorv1alpha1.ConsensusCapsule,
) (upgradeOutcome, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKey{
		Name:      consensusresources.StatefulSetName(capsule),
		Namespace: capsule.Namespace,
	}, &sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return upgradeWaiting, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "getting StatefulSet for capsule %s", capsule.Name), ReasonGetFailed),
				"Confirm the operator has get permission on statefulsets.apps")
		}
	}

	// A new-revision pod has been created (regardless of readiness); enough to trust
	// a CATASTROPHIC_FAILURE reading as belonging to the post-upgrade node — a
	// crash-looping node never becomes Ready, so this must not require readiness.
	newRevisionPodCreated := sts.Status.UpdateRevision != "" && sts.Status.UpdatedReplicas >= 1
	// The roll to the post-upgrade template is complete and serving: current
	// revision has caught up to the update revision with a Ready replica. This is
	// the revision guard for the success path.
	rolledAndReady := sts.Status.UpdateRevision != "" &&
		sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
		sts.Status.ReadyReplicas >= 1

	// Trust the platform-status reading only when the UC that wrote it runs on the
	// post-upgrade revision. Fail-closed: an unstamped or mismatched reading is
	// never trusted, so the upgrade hits the backstop rather than succeeding on a
	// stale ACTIVE.
	revisionFresh := sts.Status.UpdateRevision != "" &&
		capsule.Status.PlatformStatusRevision == sts.Status.UpdateRevision

	// CATASTROPHIC_FAILURE on the post-upgrade node is terminal — fail fast, before
	// the backstop, without waiting for readiness that will never come.
	if newRevisionPodCreated && revisionFresh && capsule.Status.PlatformStatus == operatorv1alpha1.PlatformStatusCatastrophicFailure {
		return upgradeCatastrophic, nil
	}

	// Success: the node reached ACTIVE on the fully-rolled post-upgrade revision.
	if rolledAndReady && capsule.Status.Phase == operatorv1alpha1.PhaseActive {
		return upgradeSucceeded, nil
	}

	// Backstop: covers a dead/missing UC that never advances the capsule phase.
	// Anchored to nodeUpgradeStartTime. Checked after the fast paths so a node that
	// reached ACTIVE or CATASTROPHIC_FAILURE right at the deadline is not misreported.
	timeout := r.NodeActiveTimeout
	if timeout == 0 {
		timeout = executeDefaultNodeActiveTimeout
	}
	if execute.Status.NodeUpgradeStartTime != nil && time.Since(execute.Status.NodeUpgradeStartTime.Time) > timeout {
		return upgradeTimedOut, nil
	}

	return upgradeWaiting, nil
}

func (r *NetworkUpgradeExecuteReconciler) eventf(ctx context.Context, obj *operatorv1alpha1.NetworkUpgradeExecute, eventtype string, reason errx.Reason, format string, args ...any) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "NetworkUpgradeExecute", format, args...)
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
