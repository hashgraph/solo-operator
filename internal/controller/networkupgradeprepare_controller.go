// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"math/rand/v2"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
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
	Recorder record.EventRecorder
	Checker  registry.ImageChecker
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

	// fresh per run, so iteration order is randomised each reconcile
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		comp, present := prepare.Spec.Images.Component(name)
		if !present || (comp.Enabled != nil && !*comp.Enabled) {
			continue
		}

		results := registry.CheckAll(ctx, r.Checker, comp, checkTimeout, rng)

		reachable := 0
		for _, res := range results {
			if res.Reachable {
				reachable++
			} else {
				logger.Info("registry unreachable", "component", name, "image", res.Image, "error", res.Err)
			}
		}

		unreachable := len(results) - reachable
		if r.Recorder != nil {
			r.Recorder.Eventf(&prepare, "Normal", operatorv1alpha1.EventRegistryAuditComplete,
				"%s: %d/%d registries reachable", name, reachable, len(results))
			if unreachable > 0 {
				r.Recorder.Eventf(&prepare, "Warning", operatorv1alpha1.EventRegistryUnreachable,
					"%s: %d/%d registries unreachable", name, unreachable, len(results))
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *NetworkUpgradePrepareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("networkupgradeprepare")
	}
	if r.Checker == nil {
		r.Checker = registry.OCIChecker{}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.NetworkUpgradePrepare{}).
		Named("networkupgradeprepare").
		Complete(r)
}
