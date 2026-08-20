// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

const freezePrereqRequeueAfter = 30 * time.Second

type NetworkUpgradeFreezeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	StrategyName string
}

func NewNetworkUpgradeFreezeReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeFreezeReconciler {
	return &NetworkUpgradeFreezeReconciler{
		Client:  c,
		Scheme:  scheme,
		Checker: registry.OCIChecker{},
	}
}

func (r *NetworkUpgradeFreezeReconciler) WithStrategy(name string) *NetworkUpgradeFreezeReconciler {
	r.StrategyName = name
	return r
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradefreezes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares,verbs=get;list;watch

func (r *NetworkUpgradeFreezeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var freeze operatorv1alpha1.NetworkUpgradeFreeze
	if err := r.Get(ctx, req.NamespacedName, &freeze); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradefreeze", "NetworkUpgradeFreeze", &freeze)

	if res, err := ensureFinalizer(ctx, r.Client, &freeze, FinalizerNetworkUpgradeFreeze); res != nil {
		return *res, err
	}

	if freeze.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		freeze.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if ok, msg := validateOperationIDLabel(freeze.Labels, freeze.Spec.OperationID); !ok {
		return rejectInvalidOperationIDLabel(ctx, r.Client, &freeze, &freeze.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &freeze, corev1.EventTypeWarning, reason, "%s", m) },
			func() { freeze.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	if ok, msg := validateStepLabel(freeze.Labels, operatorv1alpha1.StepFreeze); !ok {
		return rejectInvalidStepLabel(ctx, r.Client, &freeze, &freeze.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &freeze, corev1.EventTypeWarning, reason, "%s", m) },
			func() { freeze.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	sibling, err := freezeSiblingExists(ctx, r.Client, freeze.Namespace, string(freeze.UID), freeze.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if sibling {
		msg := fmt.Sprintf("operationId %q already used by another NetworkUpgradeFreeze", freeze.Spec.OperationID)
		r.eventf(ctx, &freeze, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
		freeze.Status.Phase = operatorv1alpha1.PhaseFailed
		freeze.Status.ObservedGeneration = freeze.Generation
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondManifestValidated,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonDuplicateOpID.String(),
			Message:            msg,
			ObservedGeneration: freeze.Generation,
		})
		if err := r.Status().Update(ctx, &freeze); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
		}
		return ctrl.Result{}, errx.WithReason(errReconcile.New("%s", msg), ReasonManifestInvalid)
	}

	prepare, err := findPrepare(ctx, r.Client, freeze.Namespace, freeze.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		common.Info(ctx, ReasonWaitingForPrepare).Msg("waiting for NetworkUpgradePrepare to succeed")
		return ctrl.Result{RequeueAfter: freezePrereqRequeueAfter}, nil
	}

	if freeze.Status.Phase == "" || freeze.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		freeze.Status.StartTime = &now
		freeze.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &freeze); err != nil {
			return ctrl.Result{}, wrapStatusInit(err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if _, prefetchOK := prefetchCompleted(prepare); prefetchOK {
		// Prefetch succeeded — skip registry health check. The images are
		// likely cached on nodes; the kubelet uses them via IfNotPresent.
		common.Info(ctx, operatorv1alpha1.ReasonPrefetchCompleted).Msg("images prefetched; skipping registry health check")
		now := metav1.Now()
		freeze.Status.CompletionTime = &now
		freeze.Status.Phase = operatorv1alpha1.PhaseSucceeded
		freeze.Status.SelectedSources = prepare.Status.SelectedSources
		freeze.Status.ObservedGeneration = freeze.Generation
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetched,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPrefetchCompleted.String(),
			Message: "images prefetched on nodes; registry health check skipped",
		})
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondReadyForFreeze,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPreFreezePassed.String(),
			Message: "prefetch completed; ready for freeze",
		})
		if err := r.Status().Update(ctx, &freeze); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
		}
		return ctrl.Result{}, nil
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	updated, succeeded := registry.RevalidateComponents(
		ctx, r.Checker,
		freeze.Spec.Images, freeze.Spec.SelectedSources,
		checkTimeout, rng, r.StrategyName,
		nil,
		func(name operatorv1alpha1.ComponentName, res registry.FailoverResult) {
			r.eventf(ctx, &freeze, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryFailover,
				"%s: registry %s unreachable, failed over to %s (digest: %s)",
				name, res.FailedImage, res.Selected.Image, res.Selected.Digest)
		},
		func(name operatorv1alpha1.ComponentName, err error) {
			r.eventf(ctx, &freeze, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: %v", name, err)
		},
	)

	freeze.Status.ObservedGeneration = freeze.Generation

	if succeeded {
		now := metav1.Now()
		freeze.Status.CompletionTime = &now
		freeze.Status.Phase = operatorv1alpha1.PhaseSucceeded
		freeze.Status.SelectedSources = updated
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistryHealthCheck,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonHealthCheckPassed.String(),
			Message: "all selected registries healthy",
		})
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondReadyForFreeze,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPreFreezePassed.String(),
			Message: "registry health check passed; ready for freeze",
		})
	} else {
		// Stay Running — registry issues are transient, freeze retries automatically.
		freeze.Status.Phase = operatorv1alpha1.PhaseRunning
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistryHealthCheck,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid.String(),
			Message: "no valid registry available for one or more components",
		})
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondReadyForFreeze,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid.String(),
			Message: "no valid registry available for one or more components",
		})
	}

	if err := r.Status().Update(ctx, &freeze); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
	}

	if !succeeded {
		return ctrl.Result{}, errx.WithHints(
			errx.WithReason(errReconcile.New("no valid registry for one or more components"), ReasonNoValidRegistry),
			"Confirm at least one configured registry serves a valid image for each component")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeReconciler) eventf(ctx context.Context, obj *operatorv1alpha1.NetworkUpgradeFreeze, eventtype string, reason errx.Reason, format string, args ...any) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "RegistryHealthCheck", format, args...)
}

func (r *NetworkUpgradeFreezeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradefreeze")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreeze{}).
		Named("networkupgradefreeze").
		Complete(r)
}
