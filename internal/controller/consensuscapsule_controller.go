// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/automa-saga/errx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/apipermissionproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationoverrideproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationproperties"
	"github.com/hashgraph/solo-operator/internal/resources/blocknodesconfig"
	"github.com/hashgraph/solo-operator/internal/resources/bootstrapproperties"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/resources/feeschedules"
	"github.com/hashgraph/solo-operator/internal/resources/log4j2config"
	"github.com/hashgraph/solo-operator/internal/resources/nodeproperties"
	"github.com/hashgraph/solo-operator/internal/resources/nodesettings"
	"github.com/hashgraph/solo-operator/internal/resources/simplefeesschedules"
	"github.com/hashgraph/solo-operator/internal/resources/throttlesconfig"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	ConsensusCapsuleFinalizer = "operator.solo.hiero.org/consensuscapsule-finalizer"

	// podReadinessRequeueAfter is how long to wait before rechecking when the pod is not yet ready.
	podReadinessRequeueAfter = 10 * time.Second
)

// ConsensusCapsuleReconciler reconciles a ConsensusCapsule object
type ConsensusCapsuleReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	subreconciler *subreconciler.Chain[*operatorv1alpha1.ConsensusCapsule]

	// S6 optionally overrides the s6 actuation runner (useful in tests).
	S6 consensus.S6Runner

	// clientset + restConfig back the s6 control interface: controller-runtime's
	// client cannot exec into a pod.
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

// NewConsensusCapsuleReconciler creates a new ConsensusCapsuleReconciler. Pass
// mgr.GetConfig() for restConfig; it backs the clientset used for pods/exec.
func NewConsensusCapsuleReconciler(c client.Client, scheme *runtime.Scheme, restConfig *rest.Config) (*ConsensusCapsuleReconciler, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("build clientset for pods/exec: %w", err)
	}
	return &ConsensusCapsuleReconciler{
		Client:     c,
		Scheme:     scheme,
		clientset:  clientset,
		restConfig: restConfig,
	}, nil
}

// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hiero.org,resources=consensuscapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for ConsensusCapsule resources
func (r *ConsensusCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = common.RequestContext(ctx, req)
	capsule := &operatorv1alpha1.ConsensusCapsule{}
	if err := r.Get(ctx, req.NamespacedName, capsule); err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, ReasonResourceNotFound).Msg("ConsensusCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		common.Err(ctx, err, ReasonGetFailed).Msg("Failed to get ConsensusCapsule")
		return ctrl.Result{}, err
	}

	ctx = common.ReconcileContext(ctx, "consensuscapsule", "ConsensusCapsule", capsule)
	common.Info(ctx, ReasonReconcileStart).Msg("Starting ConsensusCapsule reconciliation")
	defer common.Info(ctx, ReasonReconcileFinish).Msg("Finished ConsensusCapsule reconciliation")

	if !capsule.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, capsule)
	}

	if !controllerutil.ContainsFinalizer(capsule, ConsensusCapsuleFinalizer) {
		controllerutil.AddFinalizer(capsule, ConsensusCapsuleFinalizer)
		if err := r.Update(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// The UC sidecar is mandatory: it is the sole producer of
	// status.platformStatus, so a capsule without it can never advance past
	// Running. A disabled sidecar is a terminal configuration error — surfaced via
	// a condition + warning event, not a silent skip. No requeue: a spec edit
	// triggers a fresh reconcile.
	if err := validateUCMandatory(capsule); err != nil {
		return r.failUCSidecarRequired(ctx, capsule, err)
	}
	apimeta.RemoveStatusCondition(&capsule.Status.Conditions, operatorv1alpha1.CondUCSidecarReady)

	ready, err := r.checkConfigRefs(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		if err := r.Status().Update(ctx, capsule); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	result, err := r.subreconciler.Reconcile(ctx, capsule)
	if err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return result, err
	}
	if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	result, err = r.updateStatus(ctx, capsule)
	if err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return result, nil
}

type configRef struct {
	condType string
	refName  string
	cmName   string
}

func (r *ConsensusCapsuleReconciler) configRefs(c *operatorv1alpha1.ConsensusCapsule) []configRef {
	// Required refs — always checked regardless of whether they are set.
	refs := []configRef{
		{"Log4j2ConfigReady", c.Spec.Log4j2ConfigRef, log4j2config.ConfigMapNameForRef(c.Spec.Log4j2ConfigRef)},
		{"NodeSettingsReady", c.Spec.SettingsConfigRef, nodesettings.ConfigMapNameForRef(c.Spec.SettingsConfigRef)},
		{"ApplicationPropertiesReady", c.Spec.ApplicationPropertiesRef, applicationproperties.ConfigMapNameForRef(c.Spec.ApplicationPropertiesRef)},
	}
	// Optional refs — only checked when the ref field is non-empty; no condition is
	// set when unset so the capsule is never blocked by an absent optional config.
	for _, opt := range []struct {
		condType string
		ref      string
		fn       func(string) string
	}{
		{"ApiPermissionPropertiesReady", c.Spec.ApiPermissionPropertiesRef, apipermissionproperties.ConfigMapNameForRef},
		{"NodePropertiesReady", c.Spec.NodePropertiesRef, nodeproperties.ConfigMapNameForRef},
		{"ThrottlesConfigReady", c.Spec.ThrottlesConfigRef, throttlesconfig.ConfigMapNameForRef},
		{"FeeSchedulesReady", c.Spec.FeeSchedulesRef, feeschedules.ConfigMapNameForRef},
		{"SimpleFeesSchedulesReady", c.Spec.SimpleFeesSchedulesRef, simplefeesschedules.ConfigMapNameForRef},
		{"ApplicationOverridePropertiesReady", c.Spec.ApplicationOverridePropertiesRef, applicationoverrideproperties.ConfigMapNameForRef},
		{"BootstrapPropertiesReady", c.Spec.BootstrapPropertiesRef, bootstrapproperties.ConfigMapNameForRef},
		{"BlockNodesConfigReady", c.Spec.BlockNodesConfigRef, blocknodesconfig.ConfigMapNameForRef},
	} {
		if opt.ref != "" {
			refs = append(refs, configRef{opt.condType, opt.ref, opt.fn(opt.ref)})
		}
	}
	return refs
}

func (r *ConsensusCapsuleReconciler) checkConfigRefs(ctx context.Context, c *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	allReady := true
	for _, ref := range r.configRefs(c) {
		err := r.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: ref.cmName}, &corev1.ConfigMap{})
		switch {
		case apierrors.IsNotFound(err):
			allReady = false
			apimeta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
				Type:               ref.condType,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: c.Generation,
				Reason:             ReasonConfigMapNotFound.String(),
				Message:            fmt.Sprintf("referenced config %q has no ConfigMap %q yet", ref.refName, ref.cmName),
			})
		case err != nil:
			return false, err
		default:
			apimeta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
				Type:               ref.condType,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: c.Generation,
				Reason:             ReasonConfigMapFound.String(),
				Message:            "referenced ConfigMap exists",
			})
		}
	}
	return allReady, nil
}

func (r *ConsensusCapsuleReconciler) handleDeletion(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(capsule, ConsensusCapsuleFinalizer) {
		common.Info(ctx, ReasonDeleting).Msg("Deleting ConsensusCapsule")
		controllerutil.RemoveFinalizer(capsule, ConsensusCapsuleFinalizer)
		if err := r.Update(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ucSidecarEnabled reports whether the mandatory UC sidecar is enabled on the capsule.
func ucSidecarEnabled(capsule *operatorv1alpha1.ConsensusCapsule) bool {
	uc := capsule.Spec.PodProperties.Containers.UC
	return uc != nil && uc.Enabled
}

// validateUCMandatory returns a terminal errorx error when the ConsensusCapsule
// does not enable the mandatory UC sidecar, and nil otherwise. The error carries
// the UCSidecarRequired reason and an operator remediation hint.
func validateUCMandatory(capsule *operatorv1alpha1.ConsensusCapsule) error {
	if ucSidecarEnabled(capsule) {
		return nil
	}
	return errx.WithHints(
		errx.WithReason(
			errUCSidecarRequired.New("ConsensusCapsule %q does not enable the mandatory UC sidecar", capsule.Name),
			ReasonUCSidecarRequired),
		"Set spec.podProperties.containers.uc.enabled=true — the UC sidecar is the sole writer of status.platformStatus",
	)
}

// failUCSidecarRequired records the terminal UC-mandatory configuration error as
// a status condition and a warning event, then stops the reconcile without a
// requeue (a spec edit triggers a fresh reconcile).
func (r *ConsensusCapsuleReconciler) failUCSidecarRequired(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, err error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Error(err, "UC sidecar is mandatory", "reason", ReasonUCSidecarRequired.String())
	if r.Recorder != nil {
		r.Recorder.Eventf(capsule, nil, corev1.EventTypeWarning, operatorv1alpha1.EventUCSidecarRequired.String(),
			"Reconcile", "%s", errx.Format(err))
	}
	capsule.Status.Phase = operatorv1alpha1.PhaseFailed
	capsule.Status.ObservedGeneration = capsule.Generation
	apimeta.SetStatusCondition(&capsule.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondUCSidecarReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: capsule.Generation,
		Reason:             ReasonUCSidecarRequired.String(),
		Message:            err.Error(),
	})
	if uerr := r.Status().Update(ctx, capsule); uerr != nil {
		if apierrors.IsConflict(uerr) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, uerr
	}
	return ctrl.Result{}, nil
}

// isPodReady returns true when the pod's PodReady condition is True.
func isPodReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// findPlatformStatusCondition returns the UC-maintained platform-status condition
// on the pod, or nil when the pod has none yet.
func findPlatformStatusCondition(pod *corev1.Pod) *corev1.PodCondition {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == operatorv1alpha1.PodConditionPlatformStatus {
			return &pod.Status.Conditions[i]
		}
	}
	return nil
}

// readPlatformStatusCondition returns the raw platform-status name the UC reported
// as the reason of the pod's platform-status condition. ok is false when the pod has
// no such condition yet — a fresh pod whose UC has not reported — so the caller
// reflects UNKNOWN rather than a stale value.
func readPlatformStatusCondition(pod *corev1.Pod) (name string, ok bool) {
	cond := findPlatformStatusCondition(pod)
	if cond == nil {
		return operatorv1alpha1.PlatformStatusUnknown, false
	}
	name = cond.Reason
	if name == "" {
		name = operatorv1alpha1.PlatformStatusUnknown
	}
	return name, true
}

// isReadyPhase reports whether phase implies the pod has passed its readiness
// checks. Both Running and Active are ready phases; the genesis-initialized
// checkpoint is marked once on the first transition into either.
func isReadyPhase(phase operatorv1alpha1.Phase) bool {
	return phase == operatorv1alpha1.PhaseRunning || phase == operatorv1alpha1.PhaseActive
}

// updateStatus updates the ConsensusCapsule status from pod state and the
// consensus node's live platform status. The operator is the SOLE writer of the
// ConsensusCapsule status: it reads the operator.solo.hiero.org/platform-status
// condition the UC maintains on the current pod, proxies it into
// status.platformStatus (+ platformStatusCode), and rolls up status.phase.
//
// Phase rollup:
//   - pod not Ready → Pending
//   - pod Ready → Running
//   - pod Ready AND platformStatus==ACTIVE on the current StatefulSet revision → Active
//   - platformStatus==CATASTROPHIC_FAILURE on the current revision → Failed (+ event)
//
// Active is a sticky milestone: once reached, transient non-ACTIVE readings
// (BEHIND, RECONNECT, …) keep phase=Active — the live truth stays in
// platformStatus so the phase does not flap. Stickiness requires a live reading,
// so it never holds Active on stale state. Active is exited when:
//   - platformStatus==CATASTROPHIC_FAILURE (fresh reading) → Failed
//   - pod is no longer Ready → Pending
//   - the current pod reports no platform status (UNKNOWN): a fresh pod whose UC
//     has not reported yet, or a Terminating pod whose UC has stopped → Running
//   - the pod's revision no longer matches the StatefulSet updateRevision
//     (image roll advanced the template before the pod restarted) → Running
//
// The platform status is trusted only from a live pod on the current StatefulSet
// revision, so no freshness stamp is needed: a new pod whose UC has not reported yet
// reflects UNKNOWN, a Terminating pod (DeletionTimestamp set) reflects UNKNOWN (its
// UC has stopped), and a pod still on a superseded revision mid-roll reflects UNKNOWN
// (single-replica StatefulSets roll by replacing the one pod, so an old-revision
// reading must never reach the status). No stale value survives a roll.
//
// The operator reads the condition via the API only; it NEVER dials the pod or the
// metrics endpoint, so it works whether it runs inside or outside the cluster.
func (r *ConsensusCapsuleReconciler) updateStatus(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(capsule.Namespace),
		client.MatchingLabels(consensus.SelectorLabels(capsule))); err != nil {
		return ctrl.Result{}, err
	}

	// updateRevision identifies the StatefulSet's latest pod template. The stale-
	// reading guard drives the phase from the platform status only when the Ready pod
	// is on this revision, so a pre-change reading from an old pod cannot promote (or
	// fast-fail) during an image roll. A missing StatefulSet leaves updateRevision empty.
	updateRevision := ""
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKey{Name: consensus.StatefulSetName(capsule), Namespace: capsule.Namespace}, sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else {
		updateRevision = sts.Status.UpdateRevision
	}

	oldPhase := capsule.Status.Phase
	oldRunState := capsule.Status.RunState
	phase := operatorv1alpha1.PhasePending
	requeue := false

	// Default to UNKNOWN — reflect no live reading unless the current pod exposes a
	// fresh platform-status condition. This deliberately does NOT retain the previous
	// value, so a new or Terminating pod yields UNKNOWN rather than a stale status.
	platformStatus := operatorv1alpha1.PlatformStatusUnknown
	platformStatusCode := operatorv1alpha1.PlatformStatusCodeUnknown

	if len(podList.Items) > 0 {
		pod := podList.Items[0]
		// Trust the condition only from a live pod on the current StatefulSet revision.
		// A Terminating pod's UC has stopped, and a pod still on a superseded revision is
		// mid-roll (single-replica StatefulSets roll by replacing the one pod) — both
		// readings are stale, so reflect UNKNOWN rather than let an old-revision reading
		// reach the PLATFORM column or drive the phase.
		onUpdateRevision := updateRevision != "" && pod.Labels[appsv1.StatefulSetRevisionLabel] == updateRevision
		if pod.DeletionTimestamp.IsZero() && onUpdateRevision {
			if name, ok := readPlatformStatusCondition(&pod); ok {
				platformStatus = name
				platformStatusCode = operatorv1alpha1.PlatformStatusCodeForName(name)
			}
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			phase = r.rollupRunningPhase(capsule, pod, platformStatus, oldPhase, onUpdateRevision, &requeue)
		case corev1.PodSucceeded:
			phase = operatorv1alpha1.PhaseSucceeded
		case corev1.PodFailed:
			phase = operatorv1alpha1.PhaseFailed
		case corev1.PodPending:
			phase = operatorv1alpha1.PhasePending
		default:
			phase = operatorv1alpha1.PhaseUnknown
		}
	}

	// The platform status is operator-written too, so a change to it (even while the
	// phase is sticky-Active) must persist for the PLATFORM column — include it in
	// the no-change short-circuit.
	if capsule.Status.Phase == phase &&
		capsule.Status.PlatformStatus == platformStatus &&
		capsule.Status.PlatformStatusCode == platformStatusCode &&
		capsule.Status.RunState == oldRunState &&
		capsule.Status.ObservedGeneration == capsule.Generation {
		if requeue {
			return ctrl.Result{RequeueAfter: podReadinessRequeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}

	// Proxy the live platform status before emitting events / writing status so the
	// Failed event message and the PLATFORM column reflect the current reading.
	capsule.Status.PlatformStatus = platformStatus
	capsule.Status.PlatformStatusCode = platformStatusCode

	// Only on non-ready→ready transitions; a gen-change reconcile while already
	// ready (Running or Active) must not overwrite a manually-reset checkpoint
	// before the pod restarts. Sticky-Active keeps phase from dipping out of the
	// ready set, so this never re-fires on transient platform-status dips.
	if isReadyPhase(phase) && !isReadyPhase(oldPhase) {
		if err := r.markGenesisInitialized(ctx, capsule); err != nil {
			common.Err(ctx, err, ReasonCheckpointUpdateFailed).Msg("failed to update checkpoint")
		}
	}

	r.emitPhaseEvent(capsule, oldPhase, phase)

	gossipPort := consensus.ResolveGossipPort(capsule)

	capsule.Status.Phase = phase
	capsule.Status.ObservedGeneration = capsule.Generation
	capsule.Status.AddressBookEntry = &operatorv1alpha1.AddressBookEntry{
		NodeId:           capsule.Spec.NodeId,
		NickName:         capsule.Name,
		SelfName:         capsule.Name,
		Weight:           capsule.Spec.Weight,
		InternalHostname: consensus.InternalHostname(capsule),
		InternalPort:     gossipPort,
		ExternalHostname: capsule.Spec.ExternalAddress,
		ExternalPort:     gossipPort,
		NodeAccount:      capsule.Spec.AccountId,
	}

	if err := r.Status().Update(ctx, capsule); err != nil {
		return ctrl.Result{}, err
	}

	if requeue {
		return ctrl.Result{RequeueAfter: podReadinessRequeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

// rollupRunningPhase determines the phase for a Running pod by layering the live
// platform status (proxied from the pod's platform-status condition) on top of pod
// readiness. It sets *requeue when the pod is not yet Ready so the controller
// re-checks readiness.
//
// The platform status drives the phase (promote to Active, fast-fail on
// CATASTROPHIC_FAILURE) only when the pod runs on the current StatefulSet revision
// (onUpdateRevision, computed by the caller). This is the stale-reading guard that
// replaces the old revision stamp: a pre-roll reading from a pod on the previous
// template cannot drive the phase — and, since the caller already blanks platformStatus
// to UNKNOWN off the current revision, the guards below are belt-and-suspenders.
func (r *ConsensusCapsuleReconciler) rollupRunningPhase(
	capsule *operatorv1alpha1.ConsensusCapsule,
	pod corev1.Pod,
	platformStatus string,
	oldPhase operatorv1alpha1.Phase,
	onUpdateRevision bool,
	requeue *bool,
) operatorv1alpha1.Phase {
	// CATASTROPHIC_FAILURE on the current-revision pod is terminal — fail fast,
	// before the readiness gate (a crash-looping node never becomes Ready).
	if onUpdateRevision && platformStatus == operatorv1alpha1.PlatformStatusCatastrophicFailure {
		return operatorv1alpha1.PhaseFailed
	}

	// Gate the ready phases on the pod's Ready condition so we only report
	// Running/Active once all containers have passed their readiness checks. This
	// requires no TCP dial, so it works out-of-cluster under `task dev`.
	if !isPodReady(pod) {
		*requeue = true
		return operatorv1alpha1.PhasePending
	}

	switch {
	case onUpdateRevision && platformStatus == operatorv1alpha1.PlatformStatusActive:
		return operatorv1alpha1.PhaseActive
	case onUpdateRevision && oldPhase == operatorv1alpha1.PhaseActive &&
		platformStatus != operatorv1alpha1.PlatformStatusUnknown:
		// Sticky-Active: a transient non-ACTIVE dip (BEHIND, RECONNECT, …) keeps the
		// phase at Active while the Ready pod stays on the current revision. Stickiness
		// requires a live reading (platformStatus != UNKNOWN): a fresh pod that has not
		// yet reported, or a Terminating pod whose UC has stopped, both blank the reading
		// to UNKNOWN, so Active is not held on stale state — it falls back to Running
		// until the current pod actually reports ACTIVE again. Once the pod rolls to a
		// new template (onUpdateRevision false) Active is likewise not sticky.
		return operatorv1alpha1.PhaseActive
	case platformStatus == operatorv1alpha1.PlatformStatusUnknown && stopIntended(capsule):
		return operatorv1alpha1.PhaseStopped
	default:
		return operatorv1alpha1.PhaseRunning
	}
}

// emitPhaseEvent records a Kubernetes event when the capsule crosses into the
// Active or Failed phase. No event is posted when the phase is unchanged.
func (r *ConsensusCapsuleReconciler) emitPhaseEvent(capsule *operatorv1alpha1.ConsensusCapsule, oldPhase, newPhase operatorv1alpha1.Phase) {
	if r.Recorder == nil || oldPhase == newPhase {
		return
	}
	switch newPhase {
	case operatorv1alpha1.PhaseActive:
		r.Recorder.Eventf(capsule, nil, corev1.EventTypeNormal, operatorv1alpha1.EventConsensusNodeActive.String(),
			"UpdateStatus", "consensus node reached platform status ACTIVE")
	case operatorv1alpha1.PhaseFailed:
		msg := "consensus node pod entered Failed phase"
		if capsule.Status.PlatformStatus == operatorv1alpha1.PlatformStatusCatastrophicFailure {
			msg = fmt.Sprintf("consensus node reported platform status %s", capsule.Status.PlatformStatus)
		}
		r.Recorder.Eventf(capsule, nil, corev1.EventTypeWarning, operatorv1alpha1.EventConsensusNodeFailed.String(),
			"UpdateStatus", "%s", msg)
	}
}

// markGenesisInitialized is a no-op if the checkpoint is missing or already true.
func (r *ConsensusCapsuleReconciler) markGenesisInitialized(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Name: consensus.CheckpointConfigMapName(capsule), Namespace: capsule.Namespace}
	if err := r.Get(ctx, key, cm); err != nil {
		return client.IgnoreNotFound(err)
	}
	if cm.Data[consensus.CheckpointKeyGenesisInitialized] == consensus.CheckpointValueTrue {
		return nil
	}
	patch := client.MergeFrom(cm.DeepCopy())
	cm.Data[consensus.CheckpointKeyGenesisInitialized] = consensus.CheckpointValueTrue
	return r.Patch(ctx, cm, patch)
}

// SetupWithManager sets up the controller with the Manager
func (r *ConsensusCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("consensuscapsule")
	}
	if r.subreconciler == nil {
		if r.S6 != nil {
			r.subreconciler = consensus.NewSubReconcilersWithS6(r.Client, r.Scheme, r.Recorder, r.S6)
		} else {
			r.subreconciler = consensus.NewSubReconcilers(r.Client, r.Scheme, r.Recorder, r.clientset, r.restConfig)
		}
	}
	// Skip status-only updates (LoadBalancer IP, PVC binding, STS readyReplicas, etc.)
	genChanged := builder.WithPredicates(predicate.GenerationChangedPredicate{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ConsensusCapsule{}, genChanged).
		Owns(&corev1.ServiceAccount{}, genChanged).
		Owns(&corev1.ConfigMap{}, genChanged).
		Owns(&corev1.Service{}, genChanged).
		Owns(&corev1.PersistentVolumeClaim{}, genChanged).
		Owns(&appsv1.StatefulSet{}, genChanged).
		Owns(&rbacv1.RoleBinding{}, genChanged).
		// The UC reports platform status on its own pod's condition — a pod status
		// change, not a capsule change — so the capsule For(...) stays gen-only. Wake
		// the operator on the pod's platform-status condition (and pod readiness) so it
		// can re-proxy the value and roll up the phase. The pod→capsule map keys off the
		// consensus node-name label; the predicate filters to the platform-status
		// condition / readiness so unrelated pod churn is ignored.
		Watches(&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(mapPodToCapsule),
			builder.WithPredicates(platformStatusPodPredicate())).
		Named("consensuscapsule").
		Complete(r)
}

// mapPodToCapsule maps a consensus-node pod to a reconcile request for its owning
// ConsensusCapsule. Only pods carrying the consensus part-of + node-name labels map
// to a request; every other pod event resolves to nil so the operator is not woken by
// unrelated pod churn in the namespace.
func mapPodToCapsule(_ context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}
	if pod.Labels[consensus.LabelPartOf] != consensus.PartOfConsensusNode {
		return nil
	}
	name := pod.Labels[consensus.LabelSoloNodeName]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: pod.Namespace, Name: name}}}
}

// platformStatusPodPredicate fires on the pod events the capsule phase rollup
// depends on: a pod appearing or disappearing, a change to the platform-status
// condition, or a pod-readiness flip. It drops the frequent no-op status writes
// (metric updates, resourceVersion bumps) so the operator is not woken on every pod
// heartbeat.
func platformStatusPodPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok1 := e.ObjectOld.(*corev1.Pod)
			newPod, ok2 := e.ObjectNew.(*corev1.Pod)
			if !ok1 || !ok2 {
				return true
			}
			return platformStatusCondChanged(oldPod, newPod) || isPodReady(*oldPod) != isPodReady(*newPod)
		},
	}
}

// platformStatusCondChanged reports whether the platform-status condition differs
// between two pod revisions — added, removed, or its status/reason/message changed.
func platformStatusCondChanged(oldPod, newPod *corev1.Pod) bool {
	o := findPlatformStatusCondition(oldPod)
	n := findPlatformStatusCondition(newPod)
	switch {
	case o == nil && n == nil:
		return false
	case o == nil || n == nil:
		return true
	default:
		return o.Status != n.Status || o.Reason != n.Reason || o.Message != n.Message
	}
}

// stopIntended reports whether the operator has asked this node NOT to run:
// either an explicit DesiredRunState=Stopped, or a Manual node that has not been
// explicitly told to run. A node that is merely down (crashed / still starting)
// is deliberately excluded so an outage surfaces as Pending/Failed, not Stopped.
func stopIntended(c *operatorv1alpha1.ConsensusCapsule) bool {
	if c.Spec.DesiredRunState == operatorv1alpha1.RunStateStopped {
		return true
	}
	return c.Spec.StartPolicy == operatorv1alpha1.StartPolicyManual &&
		c.Spec.DesiredRunState != operatorv1alpha1.RunStateRunning
}
