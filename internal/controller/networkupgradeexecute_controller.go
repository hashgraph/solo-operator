// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/automa-saga/errx"
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
	"github.com/hashgraph/solo-operator/internal/registry"
	consensusresources "github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/uc"
)

const (
	// executeDefaultNodeActiveTimeout is the backstop for a node to reach platform
	// status ACTIVE after an upgrade, anchored to nodeUpgradeStartTime. It covers a
	// dead/missing UC that never advances the capsule phase. Reaching ACTIVE can
	// legitimately take much longer than pod-readiness (event replay, reconnect,
	// state loading), so it defaults generously and is operator-tunable via
	// --node-active-timeout.
	executeDefaultNodeActiveTimeout    = 10 * time.Minute
	executeDefaultNodeActiveAlertEvery = 5 * time.Minute
)

type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	StrategyName string
	// AllowedRegistryHosts is the --allowed-registry-hosts SSRF allow-list, applied to the
	// per-reconcile OCIChecker built with a namespace-derived keychain.
	AllowedRegistryHosts []string
	// NodeActiveTimeout bounds how long to wait for the consensus node to reach
	// platform status ACTIVE after an upgrade before emitting the first warning.
	NodeActiveTimeout time.Duration
	// NodeActiveAlertInterval is the interval between repeated warning events
	// after the initial NodeActiveDeadlineExceeded condition is set.
	NodeActiveAlertInterval time.Duration
}

func NewNetworkUpgradeExecuteReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeExecuteReconciler {
	return &NetworkUpgradeExecuteReconciler{
		Client:                  c,
		Scheme:                  scheme,
		NodeActiveTimeout:       executeDefaultNodeActiveTimeout,
		NodeActiveAlertInterval: executeDefaultNodeActiveAlertEvery,
	}
}

func (r *NetworkUpgradeExecuteReconciler) logPhaseElapsed(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute, phase string) {
	if execute.Status.StartTime == nil {
		return
	}
	elapsed := time.Since(execute.Status.StartTime.Time).Truncate(time.Millisecond)
	common.Info(ctx, ReasonPhaseTransition).
		Str("phase", phase).
		Str("elapsed", elapsed.String()).
		Msg("Execute phase transition")
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

// WithNodeActiveAlertInterval overrides the interval between repeated warning
// events after the initial NodeActiveDeadlineExceeded condition is set.
func (r *NetworkUpgradeExecuteReconciler) WithNodeActiveAlertInterval(d time.Duration) *NetworkUpgradeExecuteReconciler {
	if d > 0 {
		r.NodeActiveAlertInterval = d
	}
	return r
}

// WithAllowedRegistryHosts configures the SSRF allow-list enforced before any
// registry is dialed. See registry.OCIChecker.AllowedHosts.
func (r *NetworkUpgradeExecuteReconciler) WithAllowedRegistryHosts(hosts []string) *NetworkUpgradeExecuteReconciler {
	r.AllowedRegistryHosts = hosts
	return r
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeexecutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezes,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch

func (r *NetworkUpgradeExecuteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var execute operatorv1alpha1.NetworkUpgradeExecute
	if err := r.Get(ctx, req.NamespacedName, &execute); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradeexecute", "NetworkUpgradeExecute", &execute)
	common.Info(ctx, ReasonReconcileStart).Msg("Starting NetworkUpgradeExecute reconciliation")
	defer common.Info(ctx, ReasonReconcileFinish).Msg("Finished NetworkUpgradeExecute reconciliation")

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

	if ok, msg := validateStepLabel(execute.Labels, operatorv1alpha1.StepExecute); !ok {
		return rejectInvalidStepLabel(ctx, r.Client, &execute, &execute.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &execute, corev1.EventTypeWarning, reason, "%s", m) },
			func() { execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed },
			msg)
	}

	if execute.Status.Phase == "" || execute.Status.Phase == operatorv1alpha1.ExecutePhasePending {
		sibling, err := executeSiblingExists(ctx, r.Client, execute.Namespace, string(execute.UID), execute.Spec.ConsensusCapsule, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if sibling {
			msg := fmt.Sprintf("Duplicate operationId %q + consensusCapsule %q; already used by another NetworkUpgradeExecute", execute.Spec.OperationID, execute.Spec.ConsensusCapsule)
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
		capsule, err := r.getCapsule(ctx, &execute)
		if err != nil {
			return ctrl.Result{}, err
		}
		orbit, err := r.getOrbit(ctx, capsule)
		if err != nil {
			return ctrl.Result{}, err
		}
		ucfg := operatorv1alpha1.ResolveUpgradeConfig(orbit)

		if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			common.Info(ctx, ReasonWaitingForPrepare).Msg("Waiting for NetworkUpgradePrepare to succeed")
			return ctrl.Result{RequeueAfter: ucfg.PrereqRequeueInterval}, nil
		}

		freeze, err := findFreeze(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if freeze == nil || freeze.Status.Phase != operatorv1alpha1.PhaseSucceeded {
			common.Info(ctx, ReasonWaitingForFreeze).Msg("Waiting for NetworkUpgradeFreeze to succeed")
			return ctrl.Result{RequeueAfter: ucfg.PrereqRequeueInterval}, nil
		}

		if capsule == nil {
			return ctrl.Result{RequeueAfter: ucfg.NodeReadyRequeueInterval}, nil
		}
		mode, err := consensusresources.ResolveUCMode(ctx, r.Client, capsule)
		if err != nil {
			return ctrl.Result{}, err
		}

		now := metav1.Now()
		execute.Status.StartTime = &now
		execute.Status.ObservedGeneration = execute.Generation

		// Both modes transition to ReadyForProvisionerDaemon; the difference is who
		// performs the config step and, in mainnet, whether the pod is scaled down
		// first:
		//
		//   - Mainnet: the host-level provisioner daemon does the config step (and,
		//     when an infra upgrade is required, may tear down the cluster). The pod
		//     is scaled down here ONLY when infra work is required — i.e. the same
		//     skipScaleDown gate cluster-only uses. A config-only mainnet upgrade
		//     (skipScaleDown=true, no infra) leaves the pod up: the daemon's config
		//     step is pod-state-agnostic (it writes host-trusted paths + Create-only
		//     config CRs, never the live pod volume), so it behaves like cluster-only.
		//   - Cluster-only: the pod stays up. The in-pod UC provisioner-proxy reads the
		//     upgrade package from its PVC, deploys the config CRs, and advances the
		//     phase — none of which the operator can do, as only the UC can read
		//     data/config/. Scale-down is deferred to reconcilePendingNodeUpgrade.
		//
		// skipScaleDown = SkipScaleDown && !infraAccess is recomputed identically in
		// reconcilePendingNodeUpgrade; the offline flag stays the single source of
		// truth for the bring-online step, so scale-down and scale-up can't drift.
		if mode == uc.ModeMainnet {
			skipScaleDown := ucfg.SkipScaleDown && !requiresInfraAccess(&execute, orbit)
			if !skipScaleDown {
				if err := r.ensureCapsuleOffline(ctx, &execute, capsule); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon
		if err := r.Status().Update(ctx, &execute); err != nil {
			return ctrl.Result{}, wrapStatusInit(err)
		}
		r.logPhaseElapsed(ctx, &execute, string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))
		r.eventf(ctx, &execute, corev1.EventTypeNormal, operatorv1alpha1.EventReadyForDaemon,
			"Waiting for provisioner for operationId %q", execute.Spec.OperationID)
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
		common.Info(ctx, ReasonUnhandledPhase).Str("phase", string(execute.Status.Phase)).Msg("Encountered unhandled execute phase")
		return ctrl.Result{}, nil
	}
}

func (r *NetworkUpgradeExecuteReconciler) reconcileWaitingForDaemon(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsule, err := r.getCapsule(ctx, execute)
	if err != nil {
		return ctrl.Result{}, err
	}
	ucfg, err := r.resolveUpgradeConfig(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	daemonCond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondDaemonResult)

	if daemonCond == nil {
		if res, err := r.syncObservedGeneration(ctx, execute); res != nil {
			return *res, err
		}
		return ctrl.Result{RequeueAfter: ucfg.DaemonRequeueInterval}, nil
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
			"Provisioner daemon reported failure: %s", daemonCond.Reason)
		return ctrl.Result{}, nil
	}

	// DaemonResult=True is a positive handoff, but the phase advance is the
	// daemon's to make (HIP-1496): only the host-side provisioner daemon exists
	// across the infra teardown/reinstall window, so it — not the operator —
	// writes PendingInfraUpgrade (the durable pre-teardown checkpoint) and
	// PendingNodeUpgrade (post-reinstall). The operator must not race it. We only
	// record observedGeneration and wait for the daemon to advance status.phase,
	// at which point the top-level switch dispatches to the matching handler.
	if res, err := r.syncObservedGeneration(ctx, execute); res != nil {
		return *res, err
	}
	return ctrl.Result{RequeueAfter: ucfg.DaemonRequeueInterval}, nil
}

// syncObservedGeneration persists status.observedGeneration when it has drifted
// from the object's generation, and reports whether the caller should return early
// (non-nil result). While waiting on the provisioner daemon the reconciler is
// otherwise a no-op, so skipping the Status().Update when nothing changed avoids
// rewriting identical status — and the resourceVersion churn / watch events it
// triggers — on every DaemonRequeueInterval tick. The daemon's eventual phase
// advance still re-enqueues via the watch, with RequeueAfter as the backstop.
func (r *NetworkUpgradeExecuteReconciler) syncObservedGeneration(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (*ctrl.Result, error) {
	if execute.Status.ObservedGeneration == execute.Generation {
		return nil, nil
	}
	execute.Status.ObservedGeneration = execute.Generation
	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return &ctrl.Result{Requeue: true}, nil
		}
		return &ctrl.Result{}, wrapStatusUpdate(err, "updating status")
	}
	return nil, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcilePendingInfraUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsule, err := r.getCapsule(ctx, execute)
	if err != nil {
		return ctrl.Result{}, err
	}
	ucfg, err := r.resolveUpgradeConfig(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	// The daemon owns the PendingInfraUpgrade → PendingNodeUpgrade transition
	// (HIP-1496): after reinstalling the cluster it sets ConfigCRsApplied=True and
	// advances status.phase to PendingNodeUpgrade itself. The operator only waits
	// here — it must not write the phase and race the daemon across the teardown
	// window. CondConfigCRsApplied is surfaced for observability while we wait.
	cond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondConfigCRsApplied)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		common.Info(ctx, ReasonWaitingForConfigCRsApplied).Msg("Waiting for daemon to set CondConfigCRsApplied=True")
	} else {
		// ConfigCRsApplied=True but still in PendingInfraUpgrade: the daemon set the
		// condition without advancing the phase (a non-atomic handoff). Log each tick
		// so a stall here leaves a forensic trail instead of a silent poll.
		common.Info(ctx, ReasonWaitingForDaemonPhaseAdvance).
			Msg("Config CRs applied; waiting for daemon to advance phase to PendingNodeUpgrade")
	}
	return ctrl.Result{RequeueAfter: ucfg.PrereqRequeueInterval}, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcilePendingNodeUpgrade(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsule, err := r.getCapsule(ctx, execute)
	if err != nil {
		return ctrl.Result{}, err
	}
	orbit, err := r.getOrbit(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}
	ucfg := operatorv1alpha1.ResolveUpgradeConfig(orbit)
	if capsule == nil {
		return ctrl.Result{RequeueAfter: ucfg.NodeReadyRequeueInterval}, nil
	}

	// The daemon (not the operator) advances status.phase to PendingNodeUpgrade,
	// so record the node-upgrade start on first observation of this phase — it is
	// an operator-side metric the daemon does not write.
	if execute.Status.NodeUpgradeStartTime == nil {
		now := metav1.Now()
		execute.Status.NodeUpgradeStartTime = &now
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "recording node upgrade start time")
		}
		r.logPhaseElapsed(ctx, execute, string(operatorv1alpha1.ExecutePhasePendingNodeUpgrade))
		return ctrl.Result{Requeue: true}, nil
	}

	skipScaleDown := ucfg.SkipScaleDown && !requiresInfraAccess(execute, orbit)

	capsulePatched := findCondition(execute.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if capsulePatched == nil || capsulePatched.Status != metav1.ConditionTrue {
		if !skipScaleDown {
			if err := r.ensureCapsuleOffline(ctx, execute, capsule); err != nil {
				return ctrl.Result{}, err
			}
		}

		prepare, err := findPrepare(ctx, r.Client, execute.Namespace, execute.Spec.OperationID)
		if err != nil {
			return ctrl.Result{}, err
		}

		if _, prefetchOK := prefetchCompleted(prepare); prefetchOK {
			// Prefetch succeeded — skip registry re-validation. The images are
			// likely cached on nodes; the kubelet uses them via IfNotPresent.
			common.Info(ctx, operatorv1alpha1.ReasonPrefetchCompleted).Msg("Prefetched images; skipping registry re-validation")
			ss := execute.Spec.SelectedSources
			execute.Status.SelectedSources = &ss
			setCondition(&execute.Status.Conditions, metav1.Condition{
				Type:    operatorv1alpha1.CondRegistrySelected,
				Status:  metav1.ConditionTrue,
				Reason:  operatorv1alpha1.ReasonPrefetchCompleted.String(),
				Message: "images prefetched on nodes; registry re-validation skipped",
			})
		} else {
			rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
			checker, err := capsuleRegistryChecker(ctx, r.Client, r.Checker, execute.Namespace, execute.Spec.ConsensusCapsule, r.AllowedRegistryHosts)
			if err != nil {
				return ctrl.Result{}, err
			}
			skipUC := map[operatorv1alpha1.ComponentName]bool{operatorv1alpha1.ComponentUC: true}
			updated, registryOK := registry.RevalidateComponents(
				ctx, checker,
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
					"Registry validation failed for operationId %q", execute.Spec.OperationID)
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
		}

		patch := client.MergeFrom(capsule.DeepCopy())
		if err := r.applyImagePatch(execute, capsule); err != nil {
			return ctrl.Result{}, err
		}
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
		r.logPhaseElapsed(ctx, execute, operatorv1alpha1.CondConsensusCapsulePatched)
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulePatched,
			"ConsensusCapsule %s patched with new images for operationId %q", capsule.Name, execute.Spec.OperationID)
		return ctrl.Result{Requeue: true}, nil
	}

	capsulesOnline := findCondition(execute.Status.Conditions, operatorv1alpha1.CondCapsulesOnline)
	if capsulesOnline == nil || capsulesOnline.Status != metav1.ConditionTrue {
		// Bring the capsule online whenever it is offline — the operator owns that
		// lifecycle regardless of skipScaleDown. Both modes gate scale-down on the same
		// skipScaleDown = SkipScaleDown && !infraAccess (mainnet at init, cluster-only
		// here), but the timing differs, so the current skipScaleDown value is not a
		// reliable proxy for whether a scale-down actually happened. The offline flag is:
		// if the capsule is offline, an online patch is owed. Gating this on
		// !skipScaleDown instead stranded the pod at 0 replicas whenever the gate that
		// scaled it down disagreed with the gate evaluated here.
		wasOffline := capsule.Spec.Offline
		if wasOffline {
			patch := client.MergeFrom(capsule.DeepCopy())
			capsule.Spec.Offline = false
			if err := r.Patch(ctx, capsule, patch); err != nil {
				return ctrl.Result{}, errx.WithHints(
					errx.WithReason(errK8sAPI.Wrap(err, "bringing capsule %s online", capsule.Name), ReasonScaleFailed),
					"Confirm the operator has patch permission on consensuscapsules")
			}
		}
		onlineMsg := "ConsensusCapsule brought online"
		if !wasOffline {
			onlineMsg = "ConsensusCapsule images patched in-place; RollingUpdate will restart the pod"
		}
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondCapsulesOnline,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonCapsulesOnline.String(),
			Message: onlineMsg,
		})
		execute.Status.ObservedGeneration = execute.Generation
		if err := r.Status().Update(ctx, execute); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status after bringing capsule online")
		}
		r.logPhaseElapsed(ctx, execute, operatorv1alpha1.CondCapsulesOnline)
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventCapsulesOnline,
			"ConsensusCapsule %s brought online for operationId %q", capsule.Name, execute.Spec.OperationID)
		return ctrl.Result{RequeueAfter: ucfg.NodeReadyRequeueInterval}, nil
	}

	outcome, err := r.checkUpgradeGate(ctx, execute, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch outcome {
	case upgradeTimedOut:
		return r.warnDeadlineExceeded(ctx, execute)

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
		r.logPhaseElapsed(ctx, execute, string(operatorv1alpha1.ExecutePhaseSucceeded))
		r.eventf(ctx, execute, corev1.EventTypeNormal, operatorv1alpha1.EventUpgradeSucceeded,
			"Upgrade succeeded for operationId %q capsule %s", execute.Spec.OperationID, capsule.Name)
		return ctrl.Result{}, nil

	default: // upgradeWaiting — the consensus node has not yet reached ACTIVE.
		return ctrl.Result{RequeueAfter: ucfg.NodeReadyRequeueInterval}, nil
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

// warnDeadlineExceeded sets a NodeActiveDeadlineExceeded=True condition and
// emits a warning event when the backstop timeout fires, but does NOT move the
// Execute CR to Failed. The reconciler keeps requeueing so the node can still
// recover (e.g. slow event replay).
//
// Alerts are rate-limited to one per NodeActiveAlertInterval. The condition's
// LastTransitionTime is repurposed as the "last alert" timestamp: on each alert
// it is force-set to now (bypassing setCondition, which only updates it on a
// status flip). A status-update–triggered re-enqueue short-circuits via the
// time guard before writing again, so the loop self-quenches.
func (r *NetworkUpgradeExecuteReconciler) warnDeadlineExceeded(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
) (ctrl.Result, error) {
	now := metav1.Now()
	alertEvery := r.nodeActiveAlertInterval()

	if cond := findCondition(execute.Status.Conditions, operatorv1alpha1.CondNodeActiveDeadlineExceeded); cond != nil {
		since := now.Sub(cond.LastTransitionTime.Time)
		if since >= 0 && since < alertEvery {
			return ctrl.Result{RequeueAfter: alertEvery - since}, nil
		}
	}

	elapsed := time.Since(execute.Status.NodeUpgradeStartTime.Time).Truncate(time.Second)
	newCond := metav1.Condition{
		Type:               operatorv1alpha1.CondNodeActiveDeadlineExceeded,
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonNodeActiveDeadlineExceeded.String(),
		Message:            fmt.Sprintf("consensus node has not reached platform status ACTIVE after %s; still waiting", elapsed),
		ObservedGeneration: execute.Generation,
		LastTransitionTime: now,
	}
	replaced := false
	for i := range execute.Status.Conditions {
		if execute.Status.Conditions[i].Type == newCond.Type {
			execute.Status.Conditions[i] = newCond
			replaced = true
			break
		}
	}
	if !replaced {
		execute.Status.Conditions = append(execute.Status.Conditions, newCond)
	}

	if err := r.Status().Update(ctx, execute); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, wrapStatusUpdate(err, "setting NodeActiveDeadlineExceeded condition")
	}

	r.eventf(ctx, execute, corev1.EventTypeWarning, operatorv1alpha1.EventNodeActiveDeadlineExceeded,
		"Consensus node has not reached ACTIVE after %s for operationId %q; upgrade is still in progress",
		elapsed, execute.Spec.OperationID)

	return ctrl.Result{RequeueAfter: alertEvery}, nil
}

// nodeActiveTimeout returns the effective timeout duration.
func (r *NetworkUpgradeExecuteReconciler) nodeActiveTimeout() time.Duration {
	if r.NodeActiveTimeout > 0 {
		return r.NodeActiveTimeout
	}
	return executeDefaultNodeActiveTimeout
}

// nodeActiveAlertInterval returns the effective alert repeat interval.
func (r *NetworkUpgradeExecuteReconciler) nodeActiveAlertInterval() time.Duration {
	if r.NodeActiveAlertInterval > 0 {
		return r.NodeActiveAlertInterval
	}
	return executeDefaultNodeActiveAlertEvery
}

func (r *NetworkUpgradeExecuteReconciler) getCapsule(ctx context.Context, execute *operatorv1alpha1.NetworkUpgradeExecute) (*operatorv1alpha1.ConsensusCapsule, error) {
	var capsule operatorv1alpha1.ConsensusCapsule
	err := r.Get(ctx, types.NamespacedName{Name: execute.Spec.ConsensusCapsule, Namespace: execute.Namespace}, &capsule)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting ConsensusCapsule %s", execute.Spec.ConsensusCapsule), ReasonGetFailed),
			"Confirm the operator has get permission on consensuscapsules")
	}
	return &capsule, nil
}

// getOrbit fetches the Orbit referenced by the capsule. A nil capsule or empty
// Orbit ref returns (nil, nil). Otherwise the referenced Orbit must exist:
// spec.orbit is a required field and the Orbit's finalizer cascade-deletes its
// capsules, so a live capsule always has a live Orbit. A NotFound (or any other
// API error) therefore returns an error so the reconcile retries rather than
// silently falling back to defaults against a dangling ref — e.g. a transient
// window during Orbit deletion, or a misnamed spec.orbit.
func (r *NetworkUpgradeExecuteReconciler) getOrbit(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (*operatorv1alpha1.Orbit, error) {
	if capsule == nil || capsule.Spec.Orbit == "" {
		return nil, nil
	}
	var orbit operatorv1alpha1.Orbit
	if err := r.Get(ctx, client.ObjectKey{Name: capsule.Spec.Orbit}, &orbit); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting Orbit %s referenced by capsule %s", capsule.Spec.Orbit, capsule.Name), ReasonGetFailed),
			"Confirm the ConsensusCapsule's spec.orbit names an existing Orbit and the operator has get permission on orbits")
	}
	return &orbit, nil
}

func (r *NetworkUpgradeExecuteReconciler) resolveUpgradeConfig(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (operatorv1alpha1.EffectiveUpgradeConfig, error) {
	orbit, err := r.getOrbit(ctx, capsule)
	if err != nil {
		return operatorv1alpha1.EffectiveUpgradeConfig{}, err
	}
	return operatorv1alpha1.ResolveUpgradeConfig(orbit), nil
}

// requiresInfraAccess reports whether the upgrade needs host-level provisioner
// work: it compares the declared provisioner versions on the Execute CR spec
// against the Orbit CR annotations, returning true when either differs (or the
// annotation is absent but a version is declared — safe fallback). The Orbit is
// passed in (already fetched by the caller alongside ResolveUpgradeConfig) so the
// check adds no extra Get per reconcile.
func requiresInfraAccess(execute *operatorv1alpha1.NetworkUpgradeExecute, orbit *operatorv1alpha1.Orbit) bool {
	prov := execute.Spec.Provisioner
	if prov == nil || (prov.DaemonVersion == "" && prov.CLIVersion == "") {
		return false
	}
	if orbit == nil {
		return prov.DaemonVersion != "" || prov.CLIVersion != ""
	}
	ann := orbit.GetAnnotations()
	if prov.DaemonVersion != "" && prov.DaemonVersion != ann[operatorv1alpha1.AnnotationProvisionerDaemonVersion] {
		return true
	}
	if prov.CLIVersion != "" && prov.CLIVersion != ann[operatorv1alpha1.AnnotationProvisionerCLIVersion] {
		return true
	}
	return false
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
		"Scaled down ConsensusCapsule %s/%s for operationId %q", capsule.Namespace, capsule.Name, execute.Spec.OperationID)
	return nil
}

func (r *NetworkUpgradeExecuteReconciler) applyImagePatch(
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	capsule *operatorv1alpha1.ConsensusCapsule,
) error {
	images := execute.Spec.Images
	selected := execute.Status.SelectedSources

	// pinErr captures the first component whose selected source lacks a content digest.
	// The deployed image is pinned to that digest, so an unpinned source must abort the
	// execute (terminal) rather than silently deploy a mutable tag. This is the last of
	// the layered digest checks (CRD-required + UC-enforced upstream).
	var pinErr error

	// applyComponent returns the selected image tag and its verified content digest.
	// The digest is threaded through to SoftwareVersion.Digest so the deployed reference
	// is pinned to the exact image verified during Prepare (see setSoftwareVersion).
	applyComponent := func(name operatorv1alpha1.ComponentName) (image string, digest operatorv1alpha1.Sha256Digest, remove bool, upgrade bool) {
		comp, present := images.Component(name)
		if !present {
			return "", "", false, false
		}
		if comp.Enabled != nil && !*comp.Enabled {
			return "", "", true, false
		}
		src := selected.Get(name)
		if src == nil {
			return "", "", false, false
		}
		if src.Digest == "" && pinErr == nil {
			pinErr = errx.WithReason(
				errReconcile.New("selected source for component %q (operationId %s) has no content digest; refusing to deploy an unpinned (mutable-tag) image", name, execute.Spec.OperationID),
				ReasonSourceUnpinned)
		}
		return src.Image, src.Digest, false, true
	}

	if img, digest, _, upgrade := applyComponent(operatorv1alpha1.ComponentConsensusNode); upgrade {
		setSoftwareVersion(&capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion, img, digest)
	}

	if img, digest, remove, upgrade := applyComponent(operatorv1alpha1.ComponentRecordStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.RecordsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.RecordsUploader, img, digest)
	}

	if img, digest, remove, upgrade := applyComponent(operatorv1alpha1.ComponentEventStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.EventsUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.EventsUploader, img, digest)
	}

	if img, digest, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBlockStreamUploader); remove {
		capsule.Spec.PodProperties.Containers.BlocksUploader = nil
	} else if upgrade {
		ensureCheetah(&capsule.Spec.PodProperties.Containers.BlocksUploader, img, digest)
	}

	if img, digest, remove, upgrade := applyComponent(operatorv1alpha1.ComponentBackupUploader); remove {
		capsule.Spec.PodProperties.Containers.BackupUploader = nil
	} else if upgrade {
		ensureStream(&capsule.Spec.PodProperties.Containers.BackupUploader, img, digest)
	}

	if img, digest, remove, upgrade := applyComponent(operatorv1alpha1.ComponentUC); remove {
		capsule.Spec.PodProperties.Containers.UC = nil
	} else if upgrade {
		ensureUC(&capsule.Spec.PodProperties.Containers.UC, img, digest)
	}

	return pinErr
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
// (written by the operator only when the live pod's platform status is ACTIVE on the
// current revision), not on mere pod readiness.
//
// The stale-reading guard is StatefulSet-revision-based: the capsule phase and
// platformStatus are trusted only once the StatefulSet has produced / rolled to its
// post-upgrade template. Because the operator now reflects only the live pod's
// condition (an old pod's reading cannot persist in the capsule status), the guard
// here only needs to confirm the post-upgrade pod exists / has rolled — no per-reading
// revision stamp is required.
//
//   - Active on the fully-rolled post-upgrade revision      → upgradeSucceeded
//   - CATASTROPHIC_FAILURE once the new-revision pod exists  → upgradeCatastrophic (fast fail)
//   - NodeActiveTimeout (default 10m) from start elapsed     → upgradeTimedOut (warning, non-terminal)
//   - otherwise                                              → upgradeWaiting
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
	// The operator only reflects the live pod's condition into capsule.Status, so a
	// CATASTROPHIC here (with a new-revision pod created) is the post-upgrade node.
	newRevisionPodCreated := sts.Status.UpdateRevision != "" && sts.Status.UpdatedReplicas >= 1
	// The roll to the post-upgrade template is complete and serving: current
	// revision has caught up to the update revision with a Ready replica. This is
	// the revision guard for the success path.
	rolledAndReady := sts.Status.UpdateRevision != "" &&
		sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
		sts.Status.ReadyReplicas >= 1

	// CATASTROPHIC_FAILURE on the post-upgrade node is terminal — fail fast, before
	// the backstop, without waiting for readiness that will never come.
	if newRevisionPodCreated && capsule.Status.PlatformStatus == operatorv1alpha1.PlatformStatusCatastrophicFailure {
		return upgradeCatastrophic, nil
	}

	// Success: the node reached ACTIVE on the fully-rolled post-upgrade revision.
	// phase==Active already implies the operator saw ACTIVE from a Ready pod on the
	// current revision, so combined with rolledAndReady a stale reading cannot mark
	// the upgrade Succeeded.
	if rolledAndReady && capsule.Status.Phase == operatorv1alpha1.PhaseActive {
		return upgradeSucceeded, nil
	}

	// Backstop: covers a dead/missing UC that never advances the capsule phase.
	// Anchored to nodeUpgradeStartTime. Checked after the fast paths so a node that
	// reached ACTIVE or CATASTROPHIC_FAILURE right at the deadline is not misreported.
	// Exceeding the deadline emits a warning but does not fail the upgrade — the
	// node may still recover (e.g. slow event replay).
	if execute.Status.NodeUpgradeStartTime != nil && time.Since(execute.Status.NodeUpgradeStartTime.Time) > r.nodeActiveTimeout() {
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
	// r.Checker is left nil in production: the reconcile builds a checker per namespace
	// with a keychain from the ConsensusCapsule pull secrets. Tests inject a non-nil one.
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeExecute{}).
		Named("networkupgradeexecute").
		Complete(r)
}

func ensureCheetah(ptr **operatorv1alpha1.CheetahUploader, image string, digest operatorv1alpha1.Sha256Digest) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.CheetahUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image, digest)
}

func ensureStream(ptr **operatorv1alpha1.StreamUploader, image string, digest operatorv1alpha1.Sha256Digest) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.StreamUploader{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image, digest)
}

func ensureUC(ptr **operatorv1alpha1.UcSidecar, image string, digest operatorv1alpha1.Sha256Digest) {
	if *ptr == nil {
		*ptr = &operatorv1alpha1.UcSidecar{}
	}
	setSoftwareVersion(&(*ptr).SoftwareVersion, image, digest)
}

// setSoftwareVersion decomposes the selected image tag into repo/name/tag and pins the
// verified content digest onto the SoftwareVersion. common.BuildImage then emits a
// digest-pinned reference (repo/name:tag@sha256:...), so the kubelet runs exactly the
// image that was verified during Prepare. image is always the tag-only ref (ParseImage
// never sees the digest); digest is carried separately.
func setSoftwareVersion(sv **operatorv1alpha1.SoftwareVersion, image string, digest operatorv1alpha1.Sha256Digest) {
	if *sv == nil {
		*sv = &operatorv1alpha1.SoftwareVersion{}
	}
	(*sv).Repository, (*sv).ImageName, (*sv).ImageTag = common.ParseImage(image)
	(*sv).Digest = digest
}
