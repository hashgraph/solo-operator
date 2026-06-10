// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

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

const checkTimeout = 15 * time.Second

type NetworkUpgradePrepareReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Checker  registry.ImageChecker

	// StrategyName is the --registry-order flag value; "" or unknown → Random.
	StrategyName string
}

func NewNetworkUpgradePrepareReconciler(c client.Client, scheme *runtime.Scheme) *NetworkUpgradePrepareReconciler {
	return &NetworkUpgradePrepareReconciler{
		Client:  c,
		Scheme:  scheme,
		Checker: registry.OCIChecker{},
	}
}

func (r *NetworkUpgradePrepareReconciler) WithStrategy(name string) *NetworkUpgradePrepareReconciler {
	r.StrategyName = name
	return r
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/finalizers,verbs=update

func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var prepare operatorv1alpha1.NetworkUpgradePrepare
	if err := r.Get(ctx, req.NamespacedName, &prepare); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if prepare.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		prepare.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if prepare.Status.Phase == "" || prepare.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		prepare.Status.StartTime = &now
		prepare.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &prepare); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing status: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Step 1: validate operationId uniqueness within the namespace.
	var siblings operatorv1alpha1.NetworkUpgradePrepareList
	if err := r.List(ctx, &siblings,
		client.InNamespace(prepare.Namespace),
		client.MatchingFields{fieldOperationID: prepare.Spec.OperationID},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing siblings: %w", err)
	}
	for _, s := range siblings.Items {
		if s.UID == prepare.UID || s.Spec.OperationID != prepare.Spec.OperationID {
			continue
		}
		msg := fmt.Sprintf("operationId %q already used by %s/%s", prepare.Spec.OperationID, s.Namespace, s.Name)
		r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
		prepare.Status.Phase = operatorv1alpha1.PhaseFailed
		prepare.Status.ObservedGeneration = prepare.Generation
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondManifestValidated,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonDuplicateOpID,
			Message:            msg,
			ObservedGeneration: prepare.Generation,
		})
		if err := r.Status().Update(ctx, &prepare); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
		}
		return ctrl.Result{}, fmt.Errorf("%s", msg)
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	strategy := registry.NewStrategy(r.StrategyName, rng)
	logger.V(1).Info("registry iteration strategy", "strategy", strategy.Name())

	selected := &operatorv1alpha1.SelectedSources{}
	succeeded := true
	allHashesValid := true

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := prepare.Spec.Images.Component(name)
		if !present {
			r.event(&prepare, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
				"%s: omitted from manifest; current image left unchanged", name)
			continue
		}
		if comp.Enabled != nil && !*comp.Enabled {
			r.event(&prepare, corev1.EventTypeNormal, operatorv1alpha1.EventSidecarDisabled,
				"%s: enabled=false; sidecar will be removed on execute", name)
			continue
		}

		results := registry.CheckAll(ctx, r.Checker, comp, checkTimeout, strategy)

		unreachable := 0
		for _, res := range results {
			switch res.State {
			case registry.StateValid:
			case registry.StateUnreachable:
				unreachable++
				logger.Info("registry unreachable", "component", name, "image", res.Image, "error", res.Err)
			case registry.StateHashMismatch:
				allHashesValid = false
				logger.Info("registry failed layer hash verification", "component", name, "image", res.Image, "error", res.Err)
				r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
					"%s: registry %s failed layer hash verification: %v", name, res.Image, res.Err)
			case registry.StateNoHashes:
				allHashesValid = false
				logger.Info("image has no layer hashes configured", "component", name, "image", res.Image, "error", res.Err)
				r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
					"%s: image %s has no layer hashes configured: %v", name, res.Image, res.Err)
			default:
				logger.Info("unexpected registry state", "component", name, "image", res.Image, "state", res.State)
			}
		}

		reachable := len(results) - unreachable
		r.event(&prepare, corev1.EventTypeNormal, operatorv1alpha1.EventRegistryAuditComplete,
			"%s: %d/%d registries reachable", name, reachable, len(results))
		if unreachable > 0 {
			r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryUnreachable,
				"%s: %d/%d registries unreachable", name, unreachable, len(results))
		}

		winner, ok := registry.SelectValid(results, rng)
		if !ok {
			r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: no valid registry available", name)
			succeeded = false
			continue
		}
		winnerSpec := registry.SpecForImage(comp, winner.Image)
		selected.Set(name, &operatorv1alpha1.SelectedSource{
			Image:               winner.Image,
			Digest:              winner.Digest,
			LayerHashesVerified: winner.LayerHashesVerified,
			LayerHashes:         winnerSpec.LayerHashes,
		})
		r.event(&prepare, corev1.EventTypeNormal, operatorv1alpha1.EventRegistrySelected,
			"%s: selected %s (digest: %s, layerHashesVerified: %v)", name, winner.Image, winner.Digest, winner.LayerHashesVerified)
	}

	prepare.Status.ObservedGeneration = prepare.Generation

	if succeeded {
		now := metav1.Now()
		prepare.Status.CompletionTime = &now
		prepare.Status.Phase = operatorv1alpha1.PhaseSucceeded
		prepare.Status.SelectedSources = selected
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondManifestValidated,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonSchemaValid,
			Message:            "manifest schema valid",
			ObservedGeneration: prepare.Generation,
		})
		imageValidationCond := metav1.Condition{
			Type:               operatorv1alpha1.CondImageValidation,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonAllImagesValid,
			Message:            "all registries probed successfully",
			ObservedGeneration: prepare.Generation,
		}
		if !allHashesValid {
			imageValidationCond.Status = metav1.ConditionFalse
			imageValidationCond.Reason = operatorv1alpha1.ReasonRegistryInvalid
			imageValidationCond.Message = "one or more images failed layer hash verification"
		}
		setCondition(&prepare.Status.Conditions, imageValidationCond)
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondRegistrySelected,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonSelectionComplete,
			Message:            "registry selected for all components",
			ObservedGeneration: prepare.Generation,
		})
	} else {
		// Registry failures are transient — stay Running and let controller-runtime
		// requeue with backoff. PhaseFailed is only set for non-recoverable errors
		// (e.g. duplicate operationId above).
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondRegistrySelected,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonRegistryInvalid,
			Message:            "no valid registry available for one or more components",
			ObservedGeneration: prepare.Generation,
		})
	}

	if err := r.Status().Update(ctx, &prepare); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	if !succeeded {
		return ctrl.Result{}, fmt.Errorf("no valid registry for one or more components")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradePrepareReconciler) event(obj *operatorv1alpha1.NetworkUpgradePrepare, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "RegistryAudit", format, args...)
	}
}

const fieldOperationID = "spec.operationId"

func (r *NetworkUpgradePrepareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradeprepare")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradePrepare{},
		fieldOperationID,
		func(o client.Object) []string {
			return []string{o.(*operatorv1alpha1.NetworkUpgradePrepare).Spec.OperationID}
		},
	); err != nil {
		return fmt.Errorf("registering operationId index: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradePrepare{}).
		Named("networkupgradeprepare").
		Complete(r)
}
