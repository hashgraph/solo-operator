// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/operations/genesis"
)

const (
	NetworkGenesisFinalizer = "operator.solo.hedera.com/networkgenesis-finalizer"

	networkGenesisConditionReady  = "Ready"
	networkGenesisConditionFailed = "Failed"
)

// NetworkGenesisReconciler produces the genesis ConfigMaps. The CR is
// created by solo-provisioner — never by the operator itself.
type NetworkGenesisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewNetworkGenesisReconciler(c client.Client, scheme *runtime.Scheme) *NetworkGenesisReconciler {
	return &NetworkGenesisReconciler{Client: c, Scheme: scheme}
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkgeneses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkgeneses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkgeneses/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=orbits,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=consensuscapsules,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

func (r *NetworkGenesisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting NetworkGenesis reconciliation", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished NetworkGenesis reconciliation", "namespace", req.Namespace, "name", req.Name)

	ng := &operatorv1alpha1.NetworkGenesis{}
	if err := r.Get(ctx, req.NamespacedName, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !ng.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, ng)
	}

	if !controllerutil.ContainsFinalizer(ng, NetworkGenesisFinalizer) {
		controllerutil.AddFinalizer(ng, NetworkGenesisFinalizer)
		if err := r.Update(ctx, ng); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Succeeded is terminal; Failed is observational so transient errors
	// recover via controller-runtime backoff on the next requeue.
	if ng.Status.Phase == operatorv1alpha1.PhaseSucceeded {
		return ctrl.Result{}, nil
	}

	if err := r.run(ctx, ng); err != nil {
		if _, ferr := r.fail(ctx, ng, err.Error()); ferr != nil {
			log.FromContext(ctx).Error(ferr, "failed to update status after error")
		}
		return ctrl.Result{}, err
	}
	return r.succeed(ctx, ng)
}

func (r *NetworkGenesisReconciler) run(ctx context.Context, ng *operatorv1alpha1.NetworkGenesis) error {
	orbit := &operatorv1alpha1.Orbit{}
	if err := r.Get(ctx, client.ObjectKey{Name: ng.Spec.Orbit}, orbit); err != nil {
		return fmt.Errorf("orbit %q not found: %w", ng.Spec.Orbit, err)
	}

	var (
		materials []genesis.RosterMaterial
		err       error
	)
	if len(ng.Spec.Roster) > 0 {
		materials, err = genesis.MaterialsFromSpec(ng.Spec.Roster)
	} else {
		materials, err = genesis.MaterialsFromCluster(ctx, r.Client, ng.Namespace, orbit)
	}
	if err != nil {
		return err
	}

	genesisJSON, err := genesis.BuildGenesisNetworkJSON(materials)
	if err != nil {
		return fmt.Errorf("build genesis-network.json: %w", err)
	}
	if err := genesis.UpsertConfigMap(ctx, r.Client, ng.Namespace, genesis.GenesisConfigMapName, ng.Spec.Orbit,
		map[string]string{genesis.GenesisNetworkFileName: string(genesisJSON)}, nil); err != nil {
		return fmt.Errorf("upsert %s: %w", genesis.GenesisConfigMapName, err)
	}

	abBin, err := genesis.BuildAddressBookBin(materials)
	if err != nil {
		return fmt.Errorf("build addressbook-bin: %w", err)
	}
	if err := genesis.UpsertConfigMap(ctx, r.Client, ng.Namespace, genesis.AddressBookBinConfigMapName, ng.Spec.Orbit,
		nil, map[string][]byte{genesis.AddressBookBinKey: abBin}); err != nil {
		return fmt.Errorf("upsert %s: %w", genesis.AddressBookBinConfigMapName, err)
	}

	podIPs, err := genesis.CollectPodIPs(ctx, r.Client, ng.Namespace)
	if err != nil {
		return fmt.Errorf("collect pod IPs: %w", err)
	}
	clusterData := genesis.BuildClusterConfig(orbit, materials, podIPs)
	if err := genesis.UpsertConfigMap(ctx, r.Client, ng.Namespace, genesis.ClusterConfigMapName, ng.Spec.Orbit,
		clusterData, nil); err != nil {
		return fmt.Errorf("upsert %s: %w", genesis.ClusterConfigMapName, err)
	}

	return nil
}

// handleDeletion lets generated ConfigMaps outlive the CR — they are still
// mounted by ConsensusCapsule pods.
func (r *NetworkGenesisReconciler) handleDeletion(ctx context.Context, ng *operatorv1alpha1.NetworkGenesis) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(ng, NetworkGenesisFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(ng, NetworkGenesisFinalizer)
	return ctrl.Result{}, r.Update(ctx, ng)
}

func (r *NetworkGenesisReconciler) succeed(ctx context.Context, ng *operatorv1alpha1.NetworkGenesis) (ctrl.Result, error) {
	now := metav1.Now()
	ng.Status.Phase = operatorv1alpha1.PhaseSucceeded
	ng.Status.ObservedGeneration = ng.Generation
	ng.Status.CompletionTime = &now
	ng.Status.GeneratedConfigMaps = []string{
		genesis.GenesisConfigMapName,
		genesis.AddressBookBinConfigMapName,
		genesis.ClusterConfigMapName,
	}
	setNetworkGenesisCondition(&ng.Status.Conditions, metav1.Condition{
		Type:               networkGenesisConditionReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "GenesisCompleted",
		Message:            "All genesis ConfigMaps generated",
		ObservedGeneration: ng.Generation,
	})
	return ctrl.Result{}, r.Status().Update(ctx, ng)
}

func (r *NetworkGenesisReconciler) fail(ctx context.Context, ng *operatorv1alpha1.NetworkGenesis, message string) (ctrl.Result, error) {
	now := metav1.Now()
	ng.Status.Phase = operatorv1alpha1.PhaseFailed
	ng.Status.ObservedGeneration = ng.Generation
	ng.Status.CompletionTime = &now
	setNetworkGenesisCondition(&ng.Status.Conditions, metav1.Condition{
		Type:               networkGenesisConditionFailed,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "GenesisFailed",
		Message:            message,
		ObservedGeneration: ng.Generation,
	})
	return ctrl.Result{}, r.Status().Update(ctx, ng)
}

func setNetworkGenesisCondition(conditions *[]operatorv1alpha1.Condition, c metav1.Condition) {
	for i := range *conditions {
		if (*conditions)[i].Type == c.Type {
			(*conditions)[i] = c
			return
		}
	}
	*conditions = append(*conditions, c)
}

func (r *NetworkGenesisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkGenesis{}).
		Named("networkgenesis").
		Complete(r)
}
