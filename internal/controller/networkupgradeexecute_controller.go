// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	StrategyName string
}

func NewNetworkUpgradeExecuteReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradeExecuteReconciler {
	return &NetworkUpgradeExecuteReconciler{
		Client:  c,
		Scheme:  scheme,
		Checker: registry.OCIChecker{},
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/finalizers,verbs=update

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

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	updated := &operatorv1alpha1.SelectedSources{}
	succeeded := true

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := execute.Spec.Images.Component(name)
		if !present || (comp.Enabled != nil && !*comp.Enabled) {
			continue
		}

		current := execute.Spec.SelectedSources.Get(name)
		if current == nil {
			logger.Info("no selectedSource in spec for component, skipping", "component", name)
			continue
		}

		result, err := registry.RevalidateAndFailover(ctx, r.Checker, comp, *current, checkTimeout, rng, r.StrategyName)
		if err != nil {
			logger.Info("no valid registry for component", "component", name, "error", err)
			r.eventf(&execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: %v", name, err)
			succeeded = false
			continue
		}

		if result.DidFailover {
			r.eventf(&execute, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryFailover,
				"%s: registry %s unreachable, failed over to %s (digest: %s)",
				name, result.FailedImage, result.Selected.Image, result.Selected.Digest)
		}

		src := result.Selected
		updated.Set(name, &src)
	}

	execute.Status.ObservedGeneration = execute.Generation

	execute.Status.SelectedSources = updated

	if succeeded {
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistrySelected,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonSelectionComplete,
			Message: "all selected registries healthy",
		})
	} else {
		execute.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
		now := metav1.Now()
		execute.Status.CompletionTime = &now
		setCondition(&execute.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondRegistrySelected,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonRegistryInvalid,
			Message: "no valid registry available for one or more components",
		})
	}

	if err := r.Status().Update(ctx, &execute); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	if !succeeded {
		return ctrl.Result{}, fmt.Errorf("no valid registry for one or more components")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeExecuteReconciler) eventf(obj *operatorv1alpha1.NetworkUpgradeExecute, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "RegistryHealthCheck", format, args...)
	}
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
