// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

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

// NetworkUpgradePrepareReconciler validates every registry of every
// present-and-enabled component and records one selected source per component.
type NetworkUpgradePrepareReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Registry registry.Validator
}

// NewNetworkUpgradePrepareReconciler constructs a NetworkUpgradePrepareReconciler.
func NewNetworkUpgradePrepareReconciler(c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, validator registry.Validator) *NetworkUpgradePrepareReconciler {
	return &NetworkUpgradePrepareReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: recorder,
		Registry: validator,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for NetworkUpgradePrepare.
func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cr := &operatorv1alpha1.NetworkUpgradePrepare{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cr.Status.Phase == operatorv1alpha1.PhaseSucceeded || cr.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
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

	logger.Info("validating NetworkUpgradePrepare", "operationId", cr.Spec.OperationID)

	selected, summary, failed := r.validateAll(ctx, cr)

	r.recordAuditEvent(cr, summary)

	if failed != "" {
		return r.markFailed(ctx, cr, failed)
	}
	return r.markSucceeded(ctx, cr, selected)
}

type componentSummary struct {
	total     int
	reachable int
}

// validateAll walks every present-and-enabled component, probes every registry,
// and returns the per-component selection (one valid registry, chosen at random)
// plus a human-readable reachability summary.
func (r *NetworkUpgradePrepareReconciler) validateAll(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradePrepare,
) (selected *operatorv1alpha1.SelectedSources, summary map[operatorv1alpha1.ComponentName]componentSummary, failReason string) {
	logger := log.FromContext(ctx)
	selected = &operatorv1alpha1.SelectedSources{}
	summary = map[operatorv1alpha1.ComponentName]componentSummary{}

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp := upgrade.ComponentSpec(&cr.Spec.Images, name)
		present, enabled := upgrade.ComponentEnabled(comp)
		if !present || !enabled {
			continue
		}

		type candidate struct {
			image  string
			digest string
			verif  bool
		}
		var valid []candidate

		expectsHashes := upgrade.ComponentExpectsLayerHashes(comp)
		order := rand.Perm(len(comp.Registries))
		for _, idx := range order {
			reg := comp.Registries[idx]
			digest, verif, err := r.Registry.Validate(ctx, comp, &reg)
			if err != nil {
				logger.Info("registry unreachable",
					"component", name, "image", reg.Image, "error", err)
				if r.Recorder != nil {
					r.Recorder.Eventf(cr, corev1.EventTypeWarning,
						operatorv1alpha1.EventRegistryUnreachable,
						"component=%s image=%s: %v", name, reg.Image, err)
				}
				continue
			}
			if expectsHashes && !verif {
				logger.Info("registry layer hash mismatch",
					"component", name, "image", reg.Image)
				if r.Recorder != nil {
					r.Recorder.Eventf(cr, corev1.EventTypeWarning,
						operatorv1alpha1.EventRegistryUnreachable,
						"component=%s image=%s: layer hash mismatch", name, reg.Image)
				}
				continue
			}
			valid = append(valid, candidate{reg.Image, digest, verif})
		}

		summary[name] = componentSummary{total: len(comp.Registries), reachable: len(valid)}

		if len(valid) == 0 {
			if r.Recorder != nil {
				r.Recorder.Eventf(cr, corev1.EventTypeWarning,
					operatorv1alpha1.EventRegistryInvalid,
					"component=%s: no valid registry out of %d", name, len(comp.Registries))
			}
			return nil, summary, fmt.Sprintf("no valid registry for component %q", name)
		}

		pick := valid[rand.Intn(len(valid))]
		upgrade.SetSelected(selected, name, &operatorv1alpha1.SelectedSource{
			Image:               pick.image,
			Digest:              pick.digest,
			LayerHashesVerified: pick.verif,
		})
		if r.Recorder != nil {
			r.Recorder.Eventf(cr, corev1.EventTypeNormal,
				operatorv1alpha1.EventRegistrySelected,
				"component=%s image=%s digest=%s", name, pick.image, pick.digest)
		}
	}

	return selected, summary, ""
}

func (r *NetworkUpgradePrepareReconciler) recordAuditEvent(
	cr *operatorv1alpha1.NetworkUpgradePrepare,
	summary map[operatorv1alpha1.ComponentName]componentSummary,
) {
	if r.Recorder == nil || len(summary) == 0 {
		return
	}
	msg := ""
	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		s, ok := summary[name]
		if !ok {
			continue
		}
		if msg != "" {
			msg += ", "
		}
		msg += fmt.Sprintf("%s: %d/%d reachable", name, s.reachable, s.total)
	}
	r.Recorder.Event(cr, corev1.EventTypeNormal, operatorv1alpha1.EventRegistryAuditComplete, msg)
}

func (r *NetworkUpgradePrepareReconciler) markSucceeded(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradePrepare,
	selected *operatorv1alpha1.SelectedSources,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondManifestValidated, metav1.ConditionTrue,
		operatorv1alpha1.ReasonSchemaValid, "manifest schema validated", gen)
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondImageValidation, metav1.ConditionTrue,
		operatorv1alpha1.ReasonAllImagesValid, "registry probes completed", gen)
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondRegistrySelected, metav1.ConditionTrue,
		operatorv1alpha1.ReasonSelectionComplete, "one valid registry selected per present-and-enabled component", gen)

	now := metav1.Now()
	cr.Status.SelectedSources = selected
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

func (r *NetworkUpgradePrepareReconciler) markFailed(
	ctx context.Context,
	cr *operatorv1alpha1.NetworkUpgradePrepare,
	reason string,
) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondRegistrySelected, metav1.ConditionFalse,
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
func (r *NetworkUpgradePrepareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Registry == nil {
		return errors.New("NetworkUpgradePrepareReconciler.Registry is required")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradePrepare{}).
		Named("networkupgradeprepare").
		Complete(r)
}
