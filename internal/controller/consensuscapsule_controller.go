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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
	"github.com/hashgraph/solo-operator/internal/resources/log4j2config"
	"github.com/hashgraph/solo-operator/internal/resources/nodesettings"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const (
	ConsensusCapsuleFinalizer = "operator.solo.hedera.com/consensuscapsule-finalizer"

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

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for ConsensusCapsule resources
func (r *ConsensusCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting ConsensusCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished ConsensusCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	capsule := &operatorv1alpha1.ConsensusCapsule{}
	if err := r.Get(ctx, req.NamespacedName, capsule); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ConsensusCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ConsensusCapsule")
		return ctrl.Result{}, err
	}

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
	return []configRef{
		{"Log4j2ConfigReady", c.Spec.Log4j2ConfigRef, log4j2config.ConfigMapNameForRef(c.Spec.Log4j2ConfigRef)},
		{"NodeSettingsReady", c.Spec.SettingsConfigRef, nodesettings.ConfigMapNameForRef(c.Spec.SettingsConfigRef)},
	}
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
				Reason:             "ConfigMapNotFound",
				Message:            fmt.Sprintf("referenced config %q has no ConfigMap %q yet", ref.refName, ref.cmName),
			})
		case err != nil:
			return false, err
		default:
			apimeta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
				Type:               ref.condType,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: c.Generation,
				Reason:             "ConfigMapFound",
				Message:            "referenced ConfigMap exists",
			})
		}
	}
	return allReady, nil
}

func (r *ConsensusCapsuleReconciler) handleDeletion(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(capsule, ConsensusCapsuleFinalizer) {
		logger.Info("Deleting ConsensusCapsule",
			"namespace", capsule.Namespace,
			"name", capsule.Name)
		controllerutil.RemoveFinalizer(capsule, ConsensusCapsuleFinalizer)
		if err := r.Update(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
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

// updateStatus updates the ConsensusCapsule status based on pod state.
// Phase transitions to Running only once the pod's Ready condition is True,
// ensuring all containers have passed their readiness checks.
func (r *ConsensusCapsuleReconciler) updateStatus(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(capsule.Namespace),
		client.MatchingLabels(consensus.SelectorLabels(capsule))); err != nil {
		return ctrl.Result{}, err
	}

	phase := operatorv1alpha1.PhasePending
	requeue := false
	if len(podList.Items) > 0 {
		pod := podList.Items[0]
		switch pod.Status.Phase {
		case corev1.PodRunning:
			// Gate PhaseRunning on the pod's Ready condition so we only
			// report Running once all containers have passed their readiness
			// checks. This works whether the operator runs inside or outside
			// the cluster (no TCP dial to cluster-internal DNS required).
			if isPodReady(pod) {
				phase = operatorv1alpha1.PhaseRunning
			} else {
				phase = operatorv1alpha1.PhasePending
				requeue = true
			}
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

	// Only on non-Running→Running transitions; a gen-change reconcile while already Running
	// must not overwrite a manually-reset checkpoint before the pod restarts.
	if phase == operatorv1alpha1.PhaseRunning && capsule.Status.Phase != operatorv1alpha1.PhaseRunning {
		if err := r.markGenesisInitialized(ctx, capsule); err != nil {
			logger.Error(err, "failed to update checkpoint")
		}
	}

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
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ConsensusCapsule{}, genChanged).
		Owns(&corev1.ServiceAccount{}, genChanged).
		Owns(&corev1.ConfigMap{}, genChanged).
		Owns(&corev1.Service{}, genChanged).
		Owns(&corev1.PersistentVolumeClaim{}, genChanged).
		Owns(&appsv1.StatefulSet{}, genChanged).
		Owns(&rbacv1.RoleBinding{}, genChanged).
		Named("consensuscapsule").
		Complete(r)
}
