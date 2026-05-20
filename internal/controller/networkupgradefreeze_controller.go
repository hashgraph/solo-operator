// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/upgrade"
	"github.com/hashgraph/solo-operator/internal/upgrade/registry"
)

const freezePrereqRequeueAfter = 30 * time.Second

// NetworkUpgradeFreezeReconciler re-validates selected registries at freeze
// time, performing failover when needed.
type NetworkUpgradeFreezeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Registry registry.Validator
}

// NewNetworkUpgradeFreezeReconciler constructs a NetworkUpgradeFreezeReconciler.
func NewNetworkUpgradeFreezeReconciler(c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, validator registry.Validator) *NetworkUpgradeFreezeReconciler {
	return &NetworkUpgradeFreezeReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: recorder,
		Registry: validator,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for NetworkUpgradeFreeze.
func (r *NetworkUpgradeFreezeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cr := &operatorv1alpha1.NetworkUpgradeFreeze{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cr.Status.Phase == operatorv1alpha1.PhaseSucceeded || cr.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	prepare, err := upgrade.FindPrepareForOperation(ctx, r.Client, cr.Namespace, cr.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradePrepare to succeed",
			"operationId", cr.Spec.OperationID)
		return ctrl.Result{RequeueAfter: freezePrereqRequeueAfter}, nil
	}

	if cr.Status.Phase == "" {
		now := metav1.Now()
		cr.Status.Phase = operatorv1alpha1.PhaseRunning
		cr.Status.StartTime = &now
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	revalidated, failedReason := r.revalidate(ctx, cr)
	if failedReason != "" {
		return r.markFailed(ctx, cr, failedReason)
	}
	return r.markSucceeded(ctx, cr, revalidated)
}

// revalidate confirms each currently-selected registry is still reachable and
// performs failover when not. Returns the full merged SelectedSources when any
// entry changed (or nil when nothing changed), and a failure reason when no
// valid registry exists for some component.
func (r *NetworkUpgradeFreezeReconciler) revalidate(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradeFreeze,
) (updated *operatorv1alpha1.SelectedSources, failReason string) {
	logger := log.FromContext(ctx)
	merged := cr.Spec.SelectedSources
	anyChanged := false

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp := upgrade.ComponentSpec(&cr.Spec.Images, name)
		present, enabled := upgrade.ComponentEnabled(comp)
		if !present || !enabled {
			continue
		}
		current := upgrade.SelectedFor(&merged, name)
		if current == nil {
			return nil, fmt.Sprintf("spec.selectedSources missing entry for %q", name)
		}

		expectsHashes := upgrade.ComponentExpectsLayerHashes(comp)
		regSpec := findRegistry(comp, current.Image)
		if regSpec != nil {
			if digest, verif, err := r.Registry.Validate(ctx, comp, regSpec); err == nil {
				switch {
				case expectsHashes && !verif:
					logger.Info("layer hash mismatch on selected registry; failing over",
						"component", name, "image", current.Image)
				case digest == current.Digest || current.Digest == "":
					continue
				default:
					logger.Info("digest changed since prepare; recording new digest",
						"component", name, "image", current.Image,
						"oldDigest", current.Digest, "newDigest", digest)
					upgrade.SetSelected(&merged, name, &operatorv1alpha1.SelectedSource{
						Image:               current.Image,
						Digest:              digest,
						LayerHashesVerified: verif,
					})
					anyChanged = true
					continue
				}
			}
		}

		fallback := pickFailover(ctx, r.Registry, comp, current.Image, expectsHashes)
		if fallback == nil {
			return nil, fmt.Sprintf("no valid registry for component %q during freeze", name)
		}
		if r.Recorder != nil {
			r.Recorder.Eventf(cr, corev1.EventTypeWarning,
				operatorv1alpha1.EventRegistryFailover,
				"component=%s switched from %s to %s", name, current.Image, fallback.Image)
		}
		upgrade.SetSelected(&merged, name, fallback)
		anyChanged = true
	}

	if !anyChanged {
		return nil, ""
	}
	return &merged, ""
}

func findRegistry(comp *operatorv1alpha1.UpgradeComponentSpec, image string) *operatorv1alpha1.UpgradeRegistrySpec {
	for i := range comp.Registries {
		if comp.Registries[i].Image == image {
			return &comp.Registries[i]
		}
	}
	return nil
}

func (r *NetworkUpgradeFreezeReconciler) markSucceeded(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradeFreeze,
	updatedSelection *operatorv1alpha1.SelectedSources,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondRegistryHealthCheck, metav1.ConditionTrue,
		operatorv1alpha1.ReasonHealthCheckPassed, "selected registries reachable", gen)
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondReadyForFreeze, metav1.ConditionTrue,
		operatorv1alpha1.ReasonPreFreezePassed, "ready for network freeze", gen)

	if updatedSelection != nil {
		cr.Status.SelectedSources = updatedSelection
	}
	now := metav1.Now()
	cr.Status.CompletionTime = &now
	cr.Status.Phase = operatorv1alpha1.PhaseSucceeded
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradeFreezeReconciler) markFailed(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradeFreeze,
	reason string,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondRegistryHealthCheck, metav1.ConditionFalse,
		operatorv1alpha1.ReasonRegistryInvalid, reason, gen)
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondReadyForFreeze, metav1.ConditionFalse,
		operatorv1alpha1.ReasonRegistryInvalid, reason, gen)

	now := metav1.Now()
	cr.Status.CompletionTime = &now
	cr.Status.Phase = operatorv1alpha1.PhaseFailed
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeFreezeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Registry == nil {
		return errors.New("NetworkUpgradeFreezeReconciler.Registry is required")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeFreeze{}).
		Named("networkupgradefreeze").
		Complete(r)
}
