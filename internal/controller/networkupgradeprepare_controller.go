// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/automa-saga/errx"
	batchv1 "k8s.io/api/batch/v1"
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
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/resources/prefetch"
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
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var prepare operatorv1alpha1.NetworkUpgradePrepare
	if err := r.Get(ctx, req.NamespacedName, &prepare); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctx = common.ReconcileContext(ctx, "networkupgradeprepare", "NetworkUpgradePrepare", &prepare)

	if res, err := ensureFinalizer(ctx, r.Client, &prepare, FinalizerNetworkUpgradePrepare); res != nil {
		return *res, err
	}

	if prepare.Status.Phase == operatorv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	if prepare.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return r.reconcilePrefetch(ctx, &prepare)
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
	return ctrl.Result{Requeue: true}, nil
}

func (r *NetworkUpgradePrepareReconciler) reconcilePrefetch(ctx context.Context, prepare *operatorv1alpha1.NetworkUpgradePrepare) (ctrl.Result, error) {
	completed := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
	if completed != nil {
		return ctrl.Result{}, nil
	}

	created := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCreated)
	if created == nil {
		return r.createPrefetchJobs(ctx, prepare)
	}

	return r.checkPrefetchJobs(ctx, prepare)
}

func (r *NetworkUpgradePrepareReconciler) createPrefetchJobs(ctx context.Context, prepare *operatorv1alpha1.NetworkUpgradePrepare) (ctrl.Result, error) {
	if prepare.Status.SelectedSources == nil {
		return ctrl.Result{}, nil
	}

	ucfg, err := resolveUpgradeConfigFromOrbit(ctx, r.Client, prepare.Spec.Orbit)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ucfg.PrefetchEnabled {
		// Prefetch turned off via orbit.spec.upgrade.consensusNode.prefetch.enabled.
		// Record it and let Freeze/Execute run normal registry validation.
		common.Info(ctx, operatorv1alpha1.ReasonPrefetchDisabled).Msg("image prefetch disabled; skipping")
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCreated,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchDisabled.String(),
			Message: "image prefetch disabled",
		})
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchDisabled.String(),
			Message: "image prefetch disabled",
		})
		if err := r.Status().Update(ctx, prepare); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch status")
		}
		return ctrl.Result{}, nil
	}

	ttlSeconds := int32(ucfg.PrefetchJobTTL.Seconds())

	var capsules operatorv1alpha1.ConsensusCapsuleList
	if err := r.List(ctx, &capsules,
		client.InNamespace(prepare.Namespace),
		client.MatchingFields{fieldCapsuleOrbit: prepare.Spec.Orbit},
	); err != nil {
		return ctrl.Result{}, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing ConsensusCapsules"), ReasonListFailed),
			"Confirm the operator has list permission on ConsensusCapsules")
	}

	if len(capsules.Items) == 0 {
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCreated,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchSkipped.String(),
			Message: "no ConsensusCapsules found for orbit",
		})
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchSkipped.String(),
			Message: "no ConsensusCapsules found; prefetch skipped",
		})
		if err := r.Status().Update(ctx, prepare); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch status")
		}
		return ctrl.Result{}, nil
	}

	images := collectUniqueImages(prepare.Status.SelectedSources)
	if len(images) == 0 {
		return ctrl.Result{}, nil
	}

	targets := r.resolvePrefetchTargets(ctx, capsules.Items, ucfg.MaxPrefetchNodes)
	if len(targets) == 0 {
		// No node could be resolved (e.g. no consensus pods are scheduled yet).
		// Nothing to prefetch — mark skipped so Freeze/Execute fall through to
		// normal registry validation.
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCreated,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchSkipped.String(),
			Message: "no prefetch targets could be resolved",
		})
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchSkipped.String(),
			Message: "no prefetch targets; prefetch skipped",
		})
		if err := r.Status().Update(ctx, prepare); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch status")
		}
		return ctrl.Result{}, nil
	}

	// Create one Job per target node in a single pass — each Job pulls all images
	// via init containers, so object count is bounded by node count (not
	// nodes × images). Concurrent registry pulls stay bounded: node-pinned mode
	// is capped at MaxPrefetchNodes and pod-node mode by the (small) consensus
	// node count, and kubelet serializes image pulls per node. Job names are
	// deterministic, so Create is idempotent (AlreadyExists is skipped) and a
	// retry after a partial failure is safe.
	for _, target := range targets {
		job, err := prefetch.BuildPrefetchJob(prepare, target.target, target.id, images, ttlSeconds, r.Scheme)
		if err != nil {
			return ctrl.Result{}, errx.WithReason(errReconcile.Wrap(err, "building prefetch Job"), ReasonPrefetchJobBuild)
		}
		if err := r.Create(ctx, job); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			return ctrl.Result{}, errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "creating prefetch Job %s", job.Name), ReasonPrefetchJobCreate),
				"Confirm the operator has create permission on batch/v1 Jobs")
		}
	}

	targetCount := len(targets)
	r.eventf(ctx, prepare, corev1.EventTypeNormal, operatorv1alpha1.EventPrefetchCreated,
		"created %d prefetch Jobs across %d nodes for %d images", targetCount, targetCount, len(images))

	setCondition(&prepare.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.CondImagePrefetchCreated,
		Status:  metav1.ConditionTrue,
		Reason:  operatorv1alpha1.ReasonPrefetchCreated.String(),
		Message: fmt.Sprintf("created %d prefetch Jobs across %d nodes for %d images", targetCount, targetCount, len(images)),
	})
	if err := r.Status().Update(ctx, prepare); err != nil {
		return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch created status")
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

type prefetchTargetEntry struct {
	target prefetch.PrefetchTarget
	id     string
}

// resolvePrefetchTargets picks the set of prefetch Jobs to create: targetsBySelector
// when node-selector coverage applies, otherwise targetsByPod.
func (r *NetworkUpgradePrepareReconciler) resolvePrefetchTargets(
	ctx context.Context,
	capsules []operatorv1alpha1.ConsensusCapsule,
	maxNodes int32,
) []prefetchTargetEntry {
	if t, ok := r.targetsBySelector(ctx, capsules, maxNodes); ok {
		return t
	}
	return r.targetsByPod(ctx, capsules)
}

// targetsBySelector derives targets from the capsules' nodeSelector: it pins one
// Job to every node the selector matches — i.e. every node a pod could
// reschedule onto — which is the best coverage for mobile pods on network PVCs.
//
// It returns (targets, true) when this coverage applies, or (nil, false) —
// logging why — to signal the caller to fall back to targetsByPod. It applies
// only when node-pinning is enabled (maxNodes > 0), all capsules share a
// ServiceAccount (so one common SA carries the pull secrets), a nodeSelector is
// set, and the matched pool is non-empty and within maxNodes.
func (r *NetworkUpgradePrepareReconciler) targetsBySelector(
	ctx context.Context,
	capsules []operatorv1alpha1.ConsensusCapsule,
	maxNodes int32,
) ([]prefetchTargetEntry, bool) {
	if maxNodes <= 0 {
		common.Debug(ctx, operatorv1alpha1.ReasonPrefetchCreated).
			Msg("maxPrefetchNodes <= 0; using pod-node targeting")
		return nil, false
	}

	saName, saOK := commonServiceAccount(capsules)
	if !saOK {
		// Expected when capsules carry per-node pull secrets; targetsByPod lets
		// each Job use its own capsule's ServiceAccount. Not an error.
		common.Info(ctx, operatorv1alpha1.ReasonPrefetchCreated).
			Msg("capsules use different ServiceAccounts; using pod-node targeting")
		return nil, false
	}

	selectors := collectUniqueNodeSelectors(capsules)
	if len(selectors) == 0 {
		// No nodeSelector (e.g. single-node clusters) — nothing to enumerate. The
		// normal path there, not an error.
		common.Debug(ctx, operatorv1alpha1.ReasonPrefetchCreated).
			Msg("node-selector is empty; using pod-node targeting")
		return nil, false
	}

	candidateNodes := r.collectCandidateNodes(ctx, selectors)
	switch {
	case len(candidateNodes) == 0:
		// A nodeSelector is set but matched no nodes (or the node list failed).
		common.Debug(ctx, operatorv1alpha1.ReasonPrefetchCreated).
			Msg("consensus nodeSelector matched no nodes; using pod-node targeting")
		return nil, false
	case int32(len(candidateNodes)) > maxNodes:
		// Genuine degradation: the matched pool is larger than the cap, so we
		// cannot cache on every node the pod could reschedule onto. Surface it.
		common.Warn(ctx, operatorv1alpha1.ReasonPrefetchCreated).
			Msgf("%d nodes match the consensus nodeSelector, exceeding maxPrefetchNodes (%d); using pod-node targeting (spare-node coverage reduced — raise maxPrefetchNodes to cover the pool)",
				len(candidateNodes), maxNodes)
		return nil, false
	}

	common.Info(ctx, operatorv1alpha1.ReasonPrefetchCreated).
		Msgf("pinning prefetch Jobs to %d nodes matching the consensus nodeSelector", len(candidateNodes))
	tolerations := unionTolerations(capsules)
	targets := make([]prefetchTargetEntry, 0, len(candidateNodes))
	for _, nodeName := range candidateNodes {
		targets = append(targets, prefetchTargetEntry{
			target: prefetch.TargetFromNode(nodeName, saName, tolerations),
			id:     nodeName,
		})
	}
	return targets, true
}

// unionTolerations returns the deduplicated union of tolerations across all
// capsules. Node-pinned prefetch Jobs bypass the scheduler (so NoSchedule taints
// are ignored) but the taint manager still evicts pods lacking a matching
// toleration for NoExecute taints; applying the union keeps prefetch pods on
// dedicated, tainted consensus node pools.
func unionTolerations(capsules []operatorv1alpha1.ConsensusCapsule) []corev1.Toleration {
	seen := make(map[string]bool)
	var out []corev1.Toleration
	for i := range capsules {
		for _, t := range capsules[i].Spec.PodProperties.Tolerations {
			key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", t.Key, t.Operator, t.Value, t.Effect)
			if t.TolerationSeconds != nil {
				key += fmt.Sprintf("\x00%d", *t.TolerationSeconds)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, t)
		}
	}
	return out
}

// targetsByPod derives targets from the nodes currently hosting the consensus
// pods: it pins one Job per such node, each using its hosting capsule's
// ServiceAccount and tolerations. It is the fallback when targetsBySelector
// cannot apply. A StatefulSet RollingUpdate recreates the pod on the same node
// (guaranteed for node-local storage, and the common case otherwise), so the
// current pod-node is the best predictor of the restart node. Pods that are not
// yet scheduled are skipped (best-effort), and nodes are deduplicated across
// capsules.
func (r *NetworkUpgradePrepareReconciler) targetsByPod(
	ctx context.Context,
	capsules []operatorv1alpha1.ConsensusCapsule,
) []prefetchTargetEntry {
	seen := make(map[string]bool)
	var targets []prefetchTargetEntry
	for i := range capsules {
		capsule := &capsules[i]
		var pods corev1.PodList
		if err := r.List(ctx, &pods,
			client.InNamespace(capsule.Namespace),
			client.MatchingLabels(consensus.SelectorLabels(capsule)),
		); err != nil {
			common.Warn(ctx, operatorv1alpha1.ReasonPrefetchCreated).
				Msgf("failed to list pods for capsule %s: %v; skipping", capsule.Name, err)
			continue
		}
		// SA + tolerations come from the hosting capsule.
		ct := prefetch.TargetFromCapsule(capsule)
		for j := range pods.Items {
			nodeName := pods.Items[j].Spec.NodeName
			if nodeName == "" || seen[nodeName] {
				continue
			}
			seen[nodeName] = true
			targets = append(targets, prefetchTargetEntry{
				target: prefetch.TargetFromNode(nodeName, ct.ServiceAccountName, ct.Tolerations),
				id:     nodeName,
			})
		}
	}
	return targets
}

// collectCandidateNodes lists nodes matching the given nodeSelectors and returns
// a deduplicated list of node names.
func (r *NetworkUpgradePrepareReconciler) collectCandidateNodes(
	ctx context.Context,
	selectors []map[string]string,
) []string {
	seen := make(map[string]bool)
	var nodes []string
	for _, sel := range selectors {
		var nodeList corev1.NodeList
		if err := r.List(ctx, &nodeList, client.MatchingLabels(sel)); err != nil {
			common.Warn(ctx, operatorv1alpha1.ReasonPrefetchCreated).
				Msgf("failed to list nodes for selector %v: %v; pinning to current pod nodes", sel, err)
			return nil
		}
		for i := range nodeList.Items {
			name := nodeList.Items[i].Name
			if !seen[name] {
				seen[name] = true
				nodes = append(nodes, name)
			}
		}
	}
	return nodes
}

// collectUniqueNodeSelectors returns the deduplicated set of non-empty
// nodeSelector maps from capsules. Returns nil if any capsule has no
// nodeSelector (can't enumerate without a selector).
func collectUniqueNodeSelectors(capsules []operatorv1alpha1.ConsensusCapsule) []map[string]string {
	type selectorKey string
	seen := make(map[selectorKey]bool)
	var selectors []map[string]string

	for i := range capsules {
		ns := capsules[i].Spec.PodProperties.NodeSelector
		if len(ns) == 0 {
			return nil
		}
		key := selectorKey(fmt.Sprintf("%v", ns))
		if !seen[key] {
			seen[key] = true
			selectors = append(selectors, ns)
		}
	}
	return selectors
}

// commonServiceAccount returns the SA name and true if all capsules share the
// same ServiceAccount. When they differ, node-pinned Jobs can't inherit the
// correct pull secrets, so the caller should fall back to capsule-based
// scheduling.
func commonServiceAccount(capsules []operatorv1alpha1.ConsensusCapsule) (string, bool) {
	if len(capsules) == 0 {
		return "", true
	}
	sa := prefetch.TargetFromCapsule(&capsules[0]).ServiceAccountName
	for i := 1; i < len(capsules); i++ {
		if prefetch.TargetFromCapsule(&capsules[i]).ServiceAccountName != sa {
			return "", false
		}
	}
	return sa, true
}

func (r *NetworkUpgradePrepareReconciler) checkPrefetchJobs(ctx context.Context, prepare *operatorv1alpha1.NetworkUpgradePrepare) (ctrl.Result, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs,
		client.InNamespace(prepare.Namespace),
		client.MatchingLabels{
			operatorv1alpha1.LabelOperationID:  prepare.Spec.OperationID,
			operatorv1alpha1.LabelPrefetchRole: operatorv1alpha1.RolePrefetch,
		},
	); err != nil {
		return ctrl.Result{}, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing prefetch Jobs"), ReasonListFailed),
			"Confirm the operator has list permission on batch/v1 Jobs")
	}

	if len(jobs.Items) == 0 {
		// Guard against informer-cache staleness: if the created condition was
		// set very recently, the cache may not have observed the Jobs yet.
		// Requeue to let the cache catch up before deciding.
		created := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCreated)
		if created != nil && time.Since(created.LastTransitionTime.Time) < 30*time.Second {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// We got here with created=True (Jobs were created) but the list is now
		// empty, so the Jobs vanished — TTL-cleaned, manually deleted, or a
		// cache stall beyond the grace window. An empty list is ambiguous: it
		// could mean every Job succeeded, but it could equally hide a failed
		// Job that was already cleaned. Prefetch is a best-effort optimization,
		// so bias toward the safe direction: do NOT claim success. Mark the
		// outcome inconclusive so Freeze/Execute fall back to normal registry
		// validation instead of skipping it on an unverifiable assumption.
		setCondition(&prepare.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.CondImagePrefetchCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  operatorv1alpha1.ReasonPrefetchInconclusive.String(),
			Message: "prefetch Jobs no longer present; completion could not be confirmed — falling back to registry validation",
		})
		if err := r.Status().Update(ctx, prepare); err != nil {
			return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch completed status")
		}
		return ctrl.Result{}, nil
	}

	allDone := true
	anyFailed := false
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if jobHasCondition(j, batchv1.JobComplete) {
			continue
		}
		if jobHasCondition(j, batchv1.JobFailed) {
			anyFailed = true
			r.eventf(ctx, prepare, corev1.EventTypeWarning, operatorv1alpha1.EventPrefetchFailed,
				"prefetch Job %s failed", j.Name)
			continue
		}
		allDone = false
	}

	if !allDone {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	cond := metav1.Condition{
		Type:   operatorv1alpha1.CondImagePrefetchCompleted,
		Status: metav1.ConditionTrue,
		Reason: operatorv1alpha1.ReasonPrefetchCompleted.String(),
	}
	if anyFailed {
		cond.Status = metav1.ConditionFalse
		cond.Reason = operatorv1alpha1.ReasonPrefetchFailed.String()
		cond.Message = "one or more prefetch Jobs failed; upgrade will use registry pull"
	} else {
		cond.Message = "all prefetch Jobs succeeded; images cached on nodes"
		r.eventf(ctx, prepare, corev1.EventTypeNormal, operatorv1alpha1.EventPrefetchCompleted,
			"all prefetch Jobs succeeded for operationId %q", prepare.Spec.OperationID)
	}
	setCondition(&prepare.Status.Conditions, cond)
	if err := r.Status().Update(ctx, prepare); err != nil {
		return ctrl.Result{}, wrapStatusUpdate(err, "updating prefetch completed status")
	}
	return ctrl.Result{}, nil
}

func jobHasCondition(j *batchv1.Job, condType batchv1.JobConditionType) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == condType && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func collectUniqueImages(sources *operatorv1alpha1.SelectedSources) []string {
	if sources == nil {
		return nil
	}
	seen := make(map[string]bool)
	var images []string
	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		src := sources.Get(name)
		if src == nil || seen[src.Image] {
			continue
		}
		seen[src.Image] = true
		images = append(images, src.Image)
	}
	return images
}

func (r *NetworkUpgradePrepareReconciler) eventf(ctx context.Context, obj *operatorv1alpha1.NetworkUpgradePrepare, eventtype string, reason errx.Reason, format string, args ...any) {
	common.Eventf(ctx, r.Recorder, obj, eventtype, reason, "RegistryAudit", format, args...)
}

const fieldOperationID = "spec.operationId"

func prepareIndexFunc(o client.Object) []string {
	return []string{o.(*operatorv1alpha1.NetworkUpgradePrepare).Spec.OperationID}
}

const fieldCapsuleOrbit = "spec.orbit"

func capsuleOrbitIndexFunc(o client.Object) []string {
	return []string{o.(*operatorv1alpha1.ConsensusCapsule).Spec.Orbit}
}

// RegisterCapsuleOrbitIndex registers the spec.orbit field index on ConsensusCapsule.
func RegisterCapsuleOrbitIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1alpha1.ConsensusCapsule{},
		fieldCapsuleOrbit,
		capsuleOrbitIndexFunc,
	)
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
		Owns(&batchv1.Job{}).
		Named("networkupgradeprepare").
		Complete(r)
}
