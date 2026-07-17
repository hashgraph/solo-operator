// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/resources/apipermissionproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationoverrideproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationproperties"
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
}

// NewConsensusCapsuleReconciler creates a new ConsensusCapsuleReconciler
func NewConsensusCapsuleReconciler(c client.Client, scheme *runtime.Scheme) *ConsensusCapsuleReconciler {
	return &ConsensusCapsuleReconciler{
		Client: c,
		Scheme: scheme,
	}
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
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
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
		Type:               "UCSidecarReady",
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

// isReadyPhase reports whether phase implies the pod has passed its readiness
// checks. Both Running and Active are ready phases; the genesis-initialized
// checkpoint is marked once on the first transition into either.
func isReadyPhase(phase operatorv1alpha1.Phase) bool {
	return phase == operatorv1alpha1.PhaseRunning || phase == operatorv1alpha1.PhaseActive
}

// updateStatus updates the ConsensusCapsule status based on pod state and the
// consensus node's live platform status (written by the UC sidecar into
// status.platformStatus).
//
// Phase rollup:
//   - pod not Ready → Pending
//   - pod Ready → Running
//   - pod Ready AND platformStatus==ACTIVE for the current pod revision → Active
//   - platformStatus==CATASTROPHIC_FAILURE → Failed (+ event)
//
// Active is a sticky milestone: once reached, transient non-ACTIVE readings
// (BEHIND, RECONNECT, …) keep phase=Active — the live truth stays in
// platformStatus so the phase does not flap. Active is exited when:
//   - platformStatus==CATASTROPHIC_FAILURE (fresh reading) → Failed
//   - pod is no longer Ready → Pending
//   - the pod's revision no longer matches the StatefulSet updateRevision
//     (image roll advanced the template before the pod restarted) → Running
//
// The operator reads platformStatus via the API only; it NEVER dials the pod or
// the metrics endpoint, so it works whether it runs inside or outside the cluster.
func (r *ConsensusCapsuleReconciler) updateStatus(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(capsule.Namespace),
		client.MatchingLabels(consensus.SelectorLabels(capsule))); err != nil {
		return ctrl.Result{}, err
	}

	// updateRevision identifies the StatefulSet's latest pod template. The stale-
	// reading guard promotes to Active only when the Ready pod is on this revision,
	// so a pre-change ACTIVE reading from an old pod cannot promote during an image
	// roll. A missing StatefulSet leaves updateRevision empty (no promotion).
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
	phase := operatorv1alpha1.PhasePending
	requeue := false
	if len(podList.Items) > 0 {
		pod := podList.Items[0]
		switch pod.Status.Phase {
		case corev1.PodRunning:
			phase = r.rollupRunningPhase(pod, capsule.Status.PlatformStatus, capsule.Status.PlatformStatusRevision, oldPhase, updateRevision, &requeue)
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

	if capsule.Status.Phase == phase && capsule.Status.ObservedGeneration == capsule.Generation {
		if requeue {
			return ctrl.Result{RequeueAfter: podReadinessRequeueAfter}, nil
		}
		return ctrl.Result{}, nil
	}

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
// platform status (UC-written) on top of pod readiness. It sets *requeue when the
// pod is not yet Ready so the controller re-checks readiness.
func (r *ConsensusCapsuleReconciler) rollupRunningPhase(
	pod corev1.Pod,
	platformStatus string,
	platformStatusRevision string,
	oldPhase operatorv1alpha1.Phase,
	updateRevision string,
	requeue *bool,
) operatorv1alpha1.Phase {
	// Trust the platform-status reading only when the UC that wrote it runs on the
	// current StatefulSet revision. Fail-closed: an unstamped or mismatched reading
	// is never trusted, so the node stays Running until the current-revision UC
	// writes a fresh status.
	revisionFresh := updateRevision != "" && platformStatusRevision == updateRevision

	if revisionFresh && platformStatus == operatorv1alpha1.PlatformStatusCatastrophicFailure {
		return operatorv1alpha1.PhaseFailed
	}

	// Gate the ready phases on the pod's Ready condition so we only report
	// Running/Active once all containers have passed their readiness checks. This
	// requires no TCP dial, so it works out-of-cluster under `task dev`.
	if !isPodReady(pod) {
		*requeue = true
		return operatorv1alpha1.PhasePending
	}

	onUpdateRevision := updateRevision != "" && pod.Labels[appsv1.StatefulSetRevisionLabel] == updateRevision
	switch {
	case revisionFresh && platformStatus == operatorv1alpha1.PlatformStatusActive && onUpdateRevision:
		return operatorv1alpha1.PhaseActive
	case oldPhase == operatorv1alpha1.PhaseActive && onUpdateRevision && revisionFresh:
		// Sticky-Active: a transient non-ACTIVE dip (BEHIND, RECONNECT, …) keeps the
		// phase at Active while the Ready pod stays on the current revision and the UC
		// has stamped a fresh platformStatusRevision. Requiring revisionFresh is fail-
		// closed: if the UC hasn't written since a roll, Active is not sticky until it does.
		return operatorv1alpha1.PhaseActive
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
		r.subreconciler = consensus.NewSubReconcilers(r.Client, r.Scheme, r.Recorder)
	}
	// Skip status-only updates (LoadBalancer IP, PVC binding, STS readyReplicas, etc.)
	genChanged := builder.WithPredicates(predicate.GenerationChangedPredicate{})
	// The capsule's own watch must additionally wake on status.platformStatus or
	// status.platformStatusRevision changes: the UC sidecar's platform-status
	// write is a status-only update that GenerationChangedPredicate would
	// otherwise drop. The operator's own phase write is neither a generation nor
	// a platformStatus change, so it does not self-loop. Only the For(...)
	// predicate is widened; Owns(...) stay gen-only.
	forPredicate := builder.WithPredicates(genOrPlatformStatusChangedPredicate())
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ConsensusCapsule{}, forPredicate).
		Owns(&corev1.ServiceAccount{}, genChanged).
		Owns(&corev1.ConfigMap{}, genChanged).
		Owns(&corev1.Service{}, genChanged).
		Owns(&corev1.PersistentVolumeClaim{}, genChanged).
		Owns(&appsv1.StatefulSet{}, genChanged).
		Owns(&rbacv1.RoleBinding{}, genChanged).
		Named("consensuscapsule").
		Complete(r)
}

// genOrPlatformStatusChangedPredicate fires on a generation change (spec change)
// OR a status.platformStatus/platformStatusRevision change. It mirrors
// GenerationChangedPredicate for Create/Delete/Generic and adds the platform-
// status comparison on Update so the UC sidecar's status-only write wakes the
// controller. Both platformStatus AND platformStatusRevision are compared: the
// UC may write the same platformStatus value with a new revision after a pod
// restart, and the controller must see the refreshed revision to promote to Active.
func genOrPlatformStatusChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldC, ok1 := e.ObjectOld.(*operatorv1alpha1.ConsensusCapsule)
			newC, ok2 := e.ObjectNew.(*operatorv1alpha1.ConsensusCapsule)
			if !ok1 || !ok2 {
				return true
			}
			if oldC.GetGeneration() != newC.GetGeneration() {
				return true
			}
			return oldC.Status.PlatformStatus != newC.Status.PlatformStatus ||
				oldC.Status.PlatformStatusRevision != newC.Status.PlatformStatusRevision
		},
	}
}
