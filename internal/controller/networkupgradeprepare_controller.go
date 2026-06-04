// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"math/rand/v2"
	"time"

	corev1 "k8s.io/api/core/v1"
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

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=networkupgradeprepares/finalizers,verbs=update

func (r *NetworkUpgradePrepareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var prepare operatorv1alpha1.NetworkUpgradePrepare
	if err := r.Get(ctx, req.NamespacedName, &prepare); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// fresh per run so the random order changes each reconcile
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	strategy := registry.NewStrategy(r.StrategyName, rng)
	logger.V(1).Info("registry iteration strategy", "strategy", strategy.Name())

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := prepare.Spec.Images.Component(name)
		if !present || (comp.Enabled != nil && !*comp.Enabled) {
			continue
		}

		results := registry.CheckAll(ctx, r.Checker, comp, checkTimeout, strategy)

		unreachable := 0
		for _, res := range results {
			switch res.State {
			case registry.StateValid:
				// counted via the reachable total below
			case registry.StateUnreachable:
				unreachable++
				logger.Info("registry unreachable", "component", name, "image", res.Image, "error", res.Err)
			case registry.StateHashMismatch:
				// emitted per-registry (unlike aggregated unreachable) so each tampered image is named
				logger.Info("registry failed layer hash verification", "component", name, "image", res.Image, "error", res.Err)
				r.event(&prepare, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
					"%s: registry %s failed layer hash verification: %v", name, res.Image, res.Err)
			case registry.StateNoHashes:
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
	}

	return ctrl.Result{}, nil
}

func (r *NetworkUpgradePrepareReconciler) event(obj *operatorv1alpha1.NetworkUpgradePrepare, eventtype, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, nil, eventtype, reason, "RegistryAudit", format, args...)
	}
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
