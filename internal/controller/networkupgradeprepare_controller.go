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

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
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

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=networkupgradeprepares/finalizers,verbs=update

func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var prepare operatorv1alpha1.NetworkUpgradePrepare
	if err := r.Get(ctx, req.NamespacedName, &prepare); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradeprepare", "NetworkUpgradePrepare", &prepare)

	if res, err := ensureFinalizer(ctx, r.Client, &prepare, FinalizerNetworkUpgradePrepare); res != nil {
		return *res, err
	}

	if prepare.Status.Phase == operatorv1alpha1.PhaseSucceeded ||
		prepare.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if ok, msg := validateOperationIDLabel(prepare.Labels, prepare.Spec.OperationID); !ok {
		return rejectInvalidOperationIDLabel(ctx, r.Client, &prepare, &prepare.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &prepare, corev1.EventTypeWarning, reason, "%s", m) },
			func() { prepare.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	if ok, msg := validateStepLabel(prepare.Labels, operatorv1alpha1.StepPrepare); !ok {
		return rejectInvalidStepLabel(ctx, r.Client, &prepare, &prepare.Status.Conditions,
			func(reason errx.Reason, m string) { r.eventf(ctx, &prepare, corev1.EventTypeWarning, reason, "%s", m) },
			func() { prepare.Status.Phase = operatorv1alpha1.PhaseFailed },
			msg)
	}

	if prepare.Status.Phase == "" || prepare.Status.Phase == operatorv1alpha1.PhasePending {
		now := metav1.Now()
		prepare.Status.StartTime = &now
		prepare.Status.Phase = operatorv1alpha1.PhaseRunning
		if err := r.Status().Update(ctx, &prepare); err != nil {
			return ctrl.Result{}, wrapStatusInit(err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	sibling, err := prepareSiblingExists(ctx, r.Client, prepare.Namespace, string(prepare.UID), prepare.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if sibling {
		msg := fmt.Sprintf("operationId %q already used by another NetworkUpgradePrepare", prepare.Spec.OperationID)
		r.eventf(ctx, &prepare, corev1.EventTypeWarning, operatorv1alpha1.EventManifestInvalid, "%s", msg)
		prepare.Status.Phase = operatorv1alpha1.PhaseFailed
		prepare.Status.ObservedGeneration = prepare.Generation
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondManifestValidated,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonDuplicateOpID.String(),
			Message:            msg,
			ObservedGeneration: prepare.Generation,
		})
		if err := r.Status().Update(ctx, &prepare); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
		}
		return ctrl.Result{}, errx.WithReason(errReconcile.New("%s", msg), ReasonManifestInvalid)
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	strategy := registry.NewStrategy(r.StrategyName, rng)
	common.Debug(ctx, ReasonRegistryStrategy).Str("strategy", strategy.Name()).Msg("registry iteration strategy")

	selected := &operatorv1alpha1.SelectedSources{}
	succeeded := true
	allHashesValid := true

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := prepare.Spec.Images.Component(name)
		if !present {
			r.eventf(ctx, &prepare, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
				"%s: omitted from manifest; current image left unchanged", name)
			continue
		}
		if comp.Enabled != nil && !*comp.Enabled {
			r.eventf(ctx, &prepare, corev1.EventTypeNormal, operatorv1alpha1.EventSidecarDisabled,
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
				common.Info(ctx, ReasonRegistryUnreachable).Str("component", string(name)).Str("image", res.Image).AnErr("error", res.Err).Msg("registry unreachable")
			case registry.StateHashMismatch:
				allHashesValid = false
				common.Info(ctx, ReasonRegistryHashMismatch).Str("component", string(name)).Str("image", res.Image).AnErr("error", res.Err).Msg("registry failed layer hash verification")
				r.eventf(ctx, &prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
					"%s: registry %s failed layer hash verification: %v", name, res.Image, res.Err)
			case registry.StateNoHashes:
				allHashesValid = false
				common.Info(ctx, ReasonRegistryNoHashes).Str("component", string(name)).Str("image", res.Image).AnErr("error", res.Err).Msg("image has no layer hashes configured")
				r.eventf(ctx, &prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
					"%s: image %s has no layer hashes configured: %v", name, res.Image, res.Err)
			default:
				common.Info(ctx, ReasonRegistryUnexpectedState).Str("component", string(name)).Str("image", res.Image).Str("state", string(res.State)).Msg("unexpected registry state")
			}
		}

		reachable := len(results) - unreachable
		r.eventf(ctx, &prepare, corev1.EventTypeNormal, operatorv1alpha1.EventRegistryAuditComplete,
			"%s: %d/%d registries reachable", name, reachable, len(results))
		if unreachable > 0 {
			r.eventf(ctx, &prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryUnreachable,
				"%s: %d/%d registries unreachable", name, unreachable, len(results))
		}

		winner, ok := registry.SelectValid(results, rng)
		if !ok {
			r.eventf(ctx, &prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
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
		r.eventf(ctx, &prepare, corev1.EventTypeNormal, operatorv1alpha1.EventRegistrySelected,
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
			Reason:             operatorv1alpha1.ReasonSchemaValid.String(),
			Message:            "manifest schema valid",
			ObservedGeneration: prepare.Generation,
		})
		imageValidationCond := metav1.Condition{
			Type:               operatorv1alpha1.CondImageValidation,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonAllImagesValid.String(),
			Message:            "all registries probed successfully",
			ObservedGeneration: prepare.Generation,
		}
		if !allHashesValid {
			imageValidationCond.Status = metav1.ConditionFalse
			imageValidationCond.Reason = operatorv1alpha1.ReasonRegistryInvalid.String()
			imageValidationCond.Message = "one or more images failed layer hash verification"
		}
		setCondition(&prepare.Status.Conditions, imageValidationCond)
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.CondRegistrySelected,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonSelectionComplete.String(),
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
			Reason:             operatorv1alpha1.ReasonRegistryInvalid.String(),
			Message:            "no valid registry available for one or more components",
			ObservedGeneration: prepare.Generation,
		})
	}

	if err := r.Status().Update(ctx, &prepare); err != nil {
		return ctrl.Result{}, wrapStatusUpdate(err, "updating status")
	}

	if !succeeded {
		return ctrl.Result{}, errx.WithHints(
			errx.WithReason(errReconcile.New("no valid registry for one or more components"), ReasonNoValidRegistry),
			"Confirm at least one configured registry serves a valid image for each component")
	}
	return ctrl.Result{}, nil
}

func (r *NetworkUpgradePrepareReconciler) eventf(ctx context.Context, obj *operatorv1alpha1.NetworkUpgradePrepare, eventtype string, reason errx.Reason, format string, args ...any) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "RegistryAudit", format, args...)
}

const fieldOperationID = "spec.operationId"

func prepareIndexFunc(o client.Object) []string {
	return []string{o.(*operatorv1alpha1.NetworkUpgradePrepare).Spec.OperationID}
}

// RegisterPrepareIndex registers the fieldOperationID field index on NetworkUpgradePrepare.
func RegisterPrepareIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.NetworkUpgradePrepare{},
		fieldOperationID,
		prepareIndexFunc,
	)
}

func (r *NetworkUpgradePrepareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("networkupgradeprepare")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradePrepare{}).
		Named("networkupgradeprepare").
		Complete(r)
}
