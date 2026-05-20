// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/upgrade"
	"github.com/hashgraph/solo-operator/internal/upgrade/registry"
)

const (
	defaultPodReadyTimeout    = 5 * time.Minute
	executePrereqRequeueAfter = 30 * time.Second
	executeDaemonRequeueAfter = 30 * time.Second
	executePodRequeueAfter    = 15 * time.Second
)

// NetworkUpgradeExecuteReconciler drives the operator-side half of a network
// upgrade execute step: scale down the consensus pod, hand off to the daemon
// via status, then apply the three-rule image patch and observe pod readiness.
type NetworkUpgradeExecuteReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	Registry        registry.Validator
	PodReadyTimeout time.Duration
}

// NewNetworkUpgradeExecuteReconciler constructs a NetworkUpgradeExecuteReconciler.
func NewNetworkUpgradeExecuteReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	validator registry.Validator,
	podReadyTimeout time.Duration,
) *NetworkUpgradeExecuteReconciler {
	if podReadyTimeout <= 0 {
		podReadyTimeout = defaultPodReadyTimeout
	}
	return &NetworkUpgradeExecuteReconciler{
		Client:          c,
		Scheme:          scheme,
		Recorder:        recorder,
		Registry:        validator,
		PodReadyTimeout: podReadyTimeout,
	}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeexecutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for NetworkUpgradeExecute.
func (r *NetworkUpgradeExecuteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cr := &operatorv1alpha1.NetworkUpgradeExecute{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cr.Status.Phase == operatorv1alpha1.ExecutePhaseSucceeded || cr.Status.Phase == operatorv1alpha1.ExecutePhaseFailed {
		return ctrl.Result{}, nil
	}

	switch cr.Status.Phase {
	case "", operatorv1alpha1.ExecutePhasePending:
		return r.handleInitial(ctx, cr)
	case operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon,
		operatorv1alpha1.ExecutePhasePendingInfraUpgrade:
		return r.handleAwaitingDaemon(ctx, cr)
	case operatorv1alpha1.ExecutePhasePendingNodeUpgrade:
		return r.handlePendingNodeUpgrade(ctx, cr)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", cr.Status.Phase)
	}
}

func (r *NetworkUpgradeExecuteReconciler) handleInitial(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	prepare, err := upgrade.FindPrepareForOperation(ctx, r.Client, cr.Namespace, cr.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	freeze, err := upgrade.FindFreezeForOperation(ctx, r.Client, cr.Namespace, cr.Spec.OperationID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if prepare == nil || prepare.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradePrepare", "operationId", cr.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}
	if freeze == nil || freeze.Status.Phase != operatorv1alpha1.PhaseSucceeded {
		logger.Info("waiting for NetworkUpgradeFreeze", "operationId", cr.Spec.OperationID)
		return ctrl.Result{RequeueAfter: executePrereqRequeueAfter}, nil
	}

	if cr.Status.StartTime == nil {
		now := metav1.Now()
		cr.Status.StartTime = &now
	}

	updatedSel, failReason := r.revalidate(ctx, cr)
	if failReason != "" {
		return r.markFailed(ctx, cr, failReason)
	}
	if updatedSel != nil {
		cr.Status.SelectedSources = updatedSel
	}

	if err := r.setConsensusCapsulesOffline(ctx, cr, true); err != nil {
		return ctrl.Result{}, err
	}
	if r.Recorder != nil {
		r.Recorder.Event(cr, corev1.EventTypeNormal,
			operatorv1alpha1.EventPodScaledDown,
			"ConsensusCapsule.spec.offline=true; consensus pod will be scaled to 0")
	}

	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondConsensusCapsulePatched, metav1.ConditionFalse,
		operatorv1alpha1.ReasonReadyForDaemon, "waiting for daemon to apply infra/config", gen)
	cr.Status.Phase = operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if r.Recorder != nil {
		r.Recorder.Event(cr, corev1.EventTypeNormal,
			operatorv1alpha1.EventReadyForDaemon,
			"ReadyForProvisionerDaemon: handed off to provisioner-daemon")
	}
	return ctrl.Result{RequeueAfter: executeDaemonRequeueAfter}, nil
}

func (r *NetworkUpgradeExecuteReconciler) handleAwaitingDaemon(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	daemonCond := apimeta.FindStatusCondition(cr.Status.Conditions, operatorv1alpha1.CondDaemonResult)
	if daemonCond == nil {
		return ctrl.Result{RequeueAfter: executeDaemonRequeueAfter}, nil
	}
	if daemonCond.Status == metav1.ConditionFalse {
		return r.markFailed(ctx, cr, fmt.Sprintf("daemon reported failure: %s", daemonCond.Message))
	}
	if cr.Status.Phase != operatorv1alpha1.ExecutePhasePendingNodeUpgrade {
		return ctrl.Result{RequeueAfter: executeDaemonRequeueAfter}, nil
	}
	return r.handlePendingNodeUpgrade(ctx, cr)
}

func (r *NetworkUpgradeExecuteReconciler) handlePendingNodeUpgrade(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	patched := apimeta.IsStatusConditionTrue(cr.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
	if !patched {
		return r.applyCapsulePatch(ctx, cr)
	}
	return r.watchPodReadiness(ctx, cr)
}

func (r *NetworkUpgradeExecuteReconciler) applyCapsulePatch(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	sel := r.effectiveSelection(cr)

	capsules, err := r.listConsensusCapsules(ctx, cr)
	if err != nil {
		return ctrl.Result{}, err
	}
	for i := range capsules {
		cc := &capsules[i]
		original := cc.DeepCopy()
		if err := applyThreeRulePatch(cc, &cr.Spec.Images, sel); err != nil {
			return r.markFailed(ctx, cr, fmt.Sprintf("patch ConsensusCapsule %q: %v", cc.Name, err))
		}
		cc.Spec.Offline = false
		if err := r.Patch(ctx, cc, client.MergeFrom(original)); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		logger.Info("patched ConsensusCapsule", "name", cc.Name)
	}

	if r.Recorder != nil {
		r.Recorder.Eventf(cr, corev1.EventTypeNormal,
			operatorv1alpha1.EventCapsulePatched,
			"patched %d ConsensusCapsule(s) with new images; offline cleared", len(capsules))
	}

	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondConsensusCapsulePatched, metav1.ConditionTrue,
		operatorv1alpha1.ReasonPatchApplied, "ConsensusCapsule updated; offline cleared", gen)
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondNodesUpgraded, metav1.ConditionFalse,
		operatorv1alpha1.ReasonUpgradeFailed, "waiting for pods to become Ready", gen)
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: executePodRequeueAfter}, nil
}

func (r *NetworkUpgradeExecuteReconciler) watchPodReadiness(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (ctrl.Result, error) {
	capsules, err := r.listConsensusCapsules(ctx, cr)
	if err != nil {
		return ctrl.Result{}, err
	}
	ready, total, nodeIDs, err := r.evaluateReadiness(ctx, cr, capsules)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ready == total && total > 0 {
		gen := cr.Generation
		upgrade.SetCondition(&cr.Status.Conditions,
			operatorv1alpha1.CondNodesUpgraded, metav1.ConditionTrue,
			operatorv1alpha1.ReasonPodReady, "all consensus pods Ready", gen)
		now := metav1.Now()
		cr.Status.UpgradedNodes = nodeIDs
		cr.Status.Phase = operatorv1alpha1.ExecutePhaseSucceeded
		cr.Status.CompletionTime = &now
		cr.Status.ObservedGeneration = gen
		if err := r.Status().Update(ctx, cr); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		if r.Recorder != nil {
			r.Recorder.Event(cr, corev1.EventTypeNormal,
				operatorv1alpha1.EventUpgradeSucceeded,
				"network upgrade succeeded")
		}
		return ctrl.Result{}, nil
	}

	if cr.Status.StartTime != nil && time.Since(cr.Status.StartTime.Time) > r.PodReadyTimeout {
		return r.markFailed(ctx, cr,
			fmt.Sprintf("pods did not become Ready within %s (%d/%d ready)", r.PodReadyTimeout, ready, total))
	}

	gen := cr.Generation
	changed := upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondNodeStarting, metav1.ConditionTrue,
		operatorv1alpha1.ReasonPodReady,
		fmt.Sprintf("%d/%d pods Ready", ready, total), gen)
	if changed || cr.Status.ObservedGeneration != gen {
		cr.Status.ObservedGeneration = gen
		if err := r.Status().Update(ctx, cr); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: executePodRequeueAfter}, nil
}

func (r *NetworkUpgradeExecuteReconciler) listConsensusCapsules(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) ([]operatorv1alpha1.ConsensusCapsule, error) {
	list := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := r.List(ctx, list,
		client.InNamespace(cr.Namespace),
		client.MatchingLabels{common.LabelOrbit: cr.Spec.Orbit}); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *NetworkUpgradeExecuteReconciler) setConsensusCapsulesOffline(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute, offline bool) error {
	capsules, err := r.listConsensusCapsules(ctx, cr)
	if err != nil {
		return err
	}
	for i := range capsules {
		cc := &capsules[i]
		if cc.Spec.Offline == offline {
			continue
		}
		patch := client.MergeFrom(cc.DeepCopy())
		cc.Spec.Offline = offline
		if err := r.Patch(ctx, cc, patch); err != nil {
			return err
		}
	}
	return nil
}

func (r *NetworkUpgradeExecuteReconciler) evaluateReadiness(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute, capsules []operatorv1alpha1.ConsensusCapsule) (ready, total int, nodeIDs []string, err error) {
	podList := &corev1.PodList{}
	if listErr := r.List(ctx, podList,
		client.InNamespace(cr.Namespace),
		client.MatchingLabels{common.LabelOrbit: cr.Spec.Orbit}); listErr != nil {
		return 0, 0, nil, listErr
	}
	readyByInstance := make(map[string]bool, len(capsules))
	for _, p := range podList.Items {
		if !isPodReady(p) {
			continue
		}
		readyByInstance[p.Labels[common.LabelInstance]] = true
	}
	nodeIDs = make([]string, 0, len(capsules))
	for i := range capsules {
		cc := &capsules[i]
		total++
		if readyByInstance[cc.Name] {
			ready++
			nodeIDs = append(nodeIDs, strconv.FormatInt(cc.Spec.NodeId, 10))
		}
	}
	return ready, total, nodeIDs, nil
}

// effectiveSelection prefers the post-revalidation selection in status when
// present, falling back to the spec copy delivered by the UC.
func (r *NetworkUpgradeExecuteReconciler) effectiveSelection(cr *operatorv1alpha1.NetworkUpgradeExecute) *operatorv1alpha1.SelectedSources {
	if cr.Status.SelectedSources != nil {
		return cr.Status.SelectedSources
	}
	cp := cr.Spec.SelectedSources
	return &cp
}

func (r *NetworkUpgradeExecuteReconciler) revalidate(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute) (updated *operatorv1alpha1.SelectedSources, failReason string) {
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
			if _, verif, err := r.Registry.Validate(ctx, comp, regSpec); err == nil {
				if !expectsHashes || verif {
					continue
				}
				logger.Info("layer hash mismatch at execute; failing over",
					"component", name, "image", current.Image)
			}
		}

		alt := pickFailover(ctx, r.Registry, comp, current.Image, expectsHashes)
		if alt == nil {
			logger.Info("no valid registry during execute", "component", name)
			return nil, fmt.Sprintf("no valid registry for component %q during execute", name)
		}
		if r.Recorder != nil {
			r.Recorder.Eventf(cr, corev1.EventTypeWarning,
				operatorv1alpha1.EventRegistryFailover,
				"component=%s switched from %s to %s", name, current.Image, alt.Image)
		}
		upgrade.SetSelected(&merged, name, alt)
		anyChanged = true
	}

	if !anyChanged {
		return nil, ""
	}
	return &merged, ""
}

func pickFailover(
	ctx context.Context,
	v registry.Validator,
	comp *operatorv1alpha1.UpgradeComponentSpec,
	skipImage string,
	expectsHashes bool,
) *operatorv1alpha1.SelectedSource {
	for i := range comp.Registries {
		alt := comp.Registries[i]
		if alt.Image == skipImage {
			continue
		}
		digest, verif, err := v.Validate(ctx, comp, &alt)
		if err != nil {
			continue
		}
		if expectsHashes && !verif {
			continue
		}
		return &operatorv1alpha1.SelectedSource{
			Image:               alt.Image,
			Digest:              digest,
			LayerHashesVerified: verif,
		}
	}
	return nil
}

func (r *NetworkUpgradeExecuteReconciler) markFailed(ctx context.Context, cr *operatorv1alpha1.NetworkUpgradeExecute, reason string) (ctrl.Result, error) {
	gen := cr.Generation
	upgrade.SetCondition(&cr.Status.Conditions,
		operatorv1alpha1.CondNodesUpgraded, metav1.ConditionFalse,
		operatorv1alpha1.ReasonUpgradeFailed, reason, gen)
	now := metav1.Now()
	cr.Status.Phase = operatorv1alpha1.ExecutePhaseFailed
	cr.Status.CompletionTime = &now
	cr.Status.ObservedGeneration = gen
	if err := r.Status().Update(ctx, cr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if r.Recorder != nil {
		r.Recorder.Event(cr, corev1.EventTypeWarning,
			operatorv1alpha1.EventUpgradeFailed, reason)
	}
	return ctrl.Result{}, nil
}

// applyThreeRulePatch mutates the given ConsensusCapsule in place according to
// the upgrade three-rule logic:
//
//   - present + enabled=true → update the matching container's SoftwareVersion
//     to the selected image + digest
//   - omitted from spec.images → leave the container unchanged
//   - present + enabled=false → remove the sidecar container from the spec
//
// The "uc" component is validation-only at this stage and is not propagated
// onto ConsensusCapsule, since there is no UC container in the consensus pod.
func applyThreeRulePatch(
	cc *operatorv1alpha1.ConsensusCapsule,
	images *operatorv1alpha1.UpgradeImagesSpec,
	sel *operatorv1alpha1.SelectedSources,
) error {
	if cc == nil || images == nil || sel == nil {
		return errors.New("nil input to applyThreeRulePatch")
	}
	containers := &cc.Spec.PodProperties.Containers

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp := upgrade.ComponentSpec(images, name)
		present, enabled := upgrade.ComponentEnabled(comp)
		if !present {
			continue
		}
		if !enabled {
			if name == operatorv1alpha1.ComponentConsensusNode {
				return fmt.Errorf("consensusNode cannot be disabled via the upgrade manifest")
			}
			disableContainer(containers, name)
			continue
		}
		src := upgrade.SelectedFor(sel, name)
		if src == nil {
			return fmt.Errorf("missing selectedSources entry for component %q", name)
		}
		if err := setContainerImage(containers, name, src); err != nil {
			return err
		}
	}
	return nil
}

func disableContainer(c *operatorv1alpha1.ConsensusContainers, name operatorv1alpha1.ComponentName) {
	switch name {
	case operatorv1alpha1.ComponentRecordStreamUploader:
		c.RecordsUploader = nil
	case operatorv1alpha1.ComponentEventStreamUploader:
		c.EventsUploader = nil
	case operatorv1alpha1.ComponentBlockStreamUploader:
		c.BlocksUploader = nil
	case operatorv1alpha1.ComponentBackupUploader:
		c.BackupUploader = nil
	}
}

func setContainerImage(c *operatorv1alpha1.ConsensusContainers, name operatorv1alpha1.ComponentName, src *operatorv1alpha1.SelectedSource) error {
	switch name {
	case operatorv1alpha1.ComponentConsensusNode:
		if c.ConsensusNode.SoftwareVersion == nil {
			c.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{}
		}
		fillSoftwareVersion(c.ConsensusNode.SoftwareVersion, src)
	case operatorv1alpha1.ComponentRecordStreamUploader:
		if c.RecordsUploader == nil {
			c.RecordsUploader = &operatorv1alpha1.CheetahUploader{}
		}
		if c.RecordsUploader.SoftwareVersion == nil {
			c.RecordsUploader.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{}
		}
		fillSoftwareVersion(c.RecordsUploader.SoftwareVersion, src)
	case operatorv1alpha1.ComponentEventStreamUploader:
		if c.EventsUploader == nil {
			c.EventsUploader = &operatorv1alpha1.CheetahUploader{}
		}
		if c.EventsUploader.SoftwareVersion == nil {
			c.EventsUploader.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{}
		}
		fillSoftwareVersion(c.EventsUploader.SoftwareVersion, src)
	case operatorv1alpha1.ComponentBlockStreamUploader:
		if c.BlocksUploader == nil {
			c.BlocksUploader = &operatorv1alpha1.CheetahUploader{}
		}
		if c.BlocksUploader.SoftwareVersion == nil {
			c.BlocksUploader.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{}
		}
		fillSoftwareVersion(c.BlocksUploader.SoftwareVersion, src)
	case operatorv1alpha1.ComponentBackupUploader:
		if c.BackupUploader == nil {
			c.BackupUploader = &operatorv1alpha1.StreamUploader{}
		}
		if c.BackupUploader.SoftwareVersion == nil {
			c.BackupUploader.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{}
		}
		fillSoftwareVersion(c.BackupUploader.SoftwareVersion, src)
	case operatorv1alpha1.ComponentUC:
		// UC is validation-only; there is no UC container in the consensus pod yet.
		return nil
	default:
		return fmt.Errorf("unknown component %q", name)
	}
	return nil
}

func fillSoftwareVersion(sv *operatorv1alpha1.SoftwareVersion, src *operatorv1alpha1.SelectedSource) {
	host, path, tag := splitImageRef(src.Image)
	sv.Repository = host
	sv.ImageName = path
	sv.ImageTag = tag
	sv.ManifestHash = src.Digest
}

func splitImageRef(ref string) (host, path, tag string) {
	tag = "latest"
	rest := ref
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[:at]
	}
	if colon := strings.LastIndex(rest, ":"); colon > strings.LastIndex(rest, "/") {
		tag = rest[colon+1:]
		rest = rest[:colon]
	}
	var ok bool
	host, path, ok = strings.Cut(rest, "/")
	if !ok {
		host = ""
		path = rest
	}
	return
}

// SetupWithManager wires the controller into the manager.
func (r *NetworkUpgradeExecuteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Registry == nil {
		return errors.New("NetworkUpgradeExecuteReconciler.Registry is required")
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradeExecute{}).
		Named("networkupgradeexecute").
		Complete(r)
}
