// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
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

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch

func (r *NetworkUpgradeFreezeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var freeze operatorv1alpha1.NetworkUpgradeFreeze
	if err := r.Get(ctx, req.NamespacedName, &freeze); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if freeze.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		freeze.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if freeze.Status.Phase == "" || freeze.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		freeze.Status.StartTime = &now
		freeze.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &freeze); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing status: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	prepare, err := r.findPrepare(ctx, freeze.Namespace, freeze.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradePrepare to succeed", "operationId", freeze.Spec.OperationID)
		return ctrl.Result{RequeueAfter: freezePrereqRequeueAfter}, nil
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	updated := &operatorv1alpha1.SelectedSources{}
	succeeded := true

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := freeze.Spec.Images.Component(name)
		if !present || (comp.Enabled != nil && !*comp.Enabled) {
			continue
		}

		current := freeze.Spec.SelectedSources.Get(name)
		if current == nil {
			logger.Info("no selectedSource in spec for component, skipping", "component", name)
			continue
		}

		result, err := registry.RevalidateAndFailover(ctx, r.Checker, comp, *current, checkTimeout, rng, r.StrategyName)
		if err != nil {
			logger.Info("no valid registry for component", "component", name, "error", err)
			r.eventf(&freeze, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: %v", name, err)
			succeeded = false
			continue
		}

		if result.DidFailover {
			r.eventf(&freeze, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryFailover,
				"%s: registry %s unreachable, failed over to %s (digest: %s)",
				name, result.FailedImage, result.Selected.Image, result.Selected.Digest)
		}

		src := result.Selected
		updated.Set(name, &src)
	}

	freeze.Status.ObservedGeneration = freeze.Generation

	if succeeded {
		now := metav1.Now()
		freeze.Status.CompletionTime = &now
		freeze.Status.Phase = operatorv1alpha1.PhaseSucceeded
		freeze.Status.SelectedSources = updated
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistryHealthCheck,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonHealthCheckPassed,
			Message: "all selected registries healthy",
		})
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondReadyForFreeze,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPreFreezePassed,
			Message: "registry health check passed; ready for freeze",
		})
	} else {
		// Stay Running — registry issues are transient, freeze retries automatically.
		freeze.Status.Phase = operatorv1alpha1.PhaseRunning
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistryHealthCheck,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid,
			Message: "no valid registry available for one or more components",
		})
		setCondition(&freeze.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondReadyForFreeze,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid,
			Message: "no valid registry available for one or more components",
		})
	}

	if err := r.Status().Update(ctx, &freeze); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	if !succeeded {
		return ctrl.Result{}, fmt.Errorf("no valid registry for one or more components")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeReconciler) findPrepare(ctx context.Context, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradePrepare, error) {
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

func (r *NetworkUpgradeFreezeReconciler) eventf(obj *operatorv1alpha1.NetworkUpgradeFreeze, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "RegistryHealthCheck", format, args...)
	}
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
