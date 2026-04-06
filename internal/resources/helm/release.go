// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// ReleaseSubReconciler reconciles the Helm release for a HelmCapsule.
type ReleaseSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HelmCapsule]
	helmClient Client
}

// NewSubReconcilers returns the subreconciler chain for the HelmCapsule controller.
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, helmClient Client) *subreconciler.Chain[*operatorv1alpha1.HelmCapsule] {
	return subreconciler.NewChain[*operatorv1alpha1.HelmCapsule](
		&ReleaseSubReconciler{
			BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
			helmClient:        helmClient,
		},
	)
}

func (r *ReleaseSubReconciler) Name() string { return "helm-release" }

func (r *ReleaseSubReconciler) Reconcile(ctx context.Context, hc *operatorv1alpha1.HelmCapsule) (ctrl.Result, error) {
	releaseName := hc.Spec.ReleaseName
	if releaseName == "" {
		releaseName = hc.Name
	}

	logger := log.FromContext(ctx).WithValues(
		"release", releaseName,
		"chart", hc.Spec.Chart.Chart,
		"version", hc.Spec.Chart.Version,
	)

	values, err := MergeValues(ctx, r.Client, hc.Namespace, &hc.Spec)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("%w: %s", ErrValuesBuild, err)
	}

	hash, err := ComputeHash(hc.Spec.Chart, values)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hash == hc.Status.ValuesHash {
		// Skip only if the release is actually deployed. A failed or pending-install
		// release must be retried even when the spec hasn't changed.
		existing, err := r.helmClient.GetRelease(releaseName, hc.Namespace)
		if err == nil && existing != nil && existing.Status == "deployed" {
			logger.V(1).Info("Release up-to-date, skipping upgrade")
			return ctrl.Result{}, nil
		}
	}

	opts := releaseOptions(hc)
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout // apply default here so the log is accurate
	}
	logger.Info("Reconciling Helm release", "timeout", opts.Timeout)

	info, err := r.helmClient.InstallOrUpgrade(ctx, releaseName, hc.Namespace, hc.Spec.Chart, values, opts)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Helm release reconciled", "revision", info.Version, "status", info.Status)

	hc.Status.ReleaseVersion = info.Version
	hc.Status.InstalledChart = info.ChartString
	hc.Status.ValuesHash = hash

	return ctrl.Result{}, nil
}

func releaseOptions(hc *operatorv1alpha1.HelmCapsule) ReleaseOptions {
	opts := ReleaseOptions{
		Wait:            hc.Spec.Wait,
		Atomic:          hc.Spec.Atomic,
		CleanupOnFail:   hc.Spec.CleanupOnFail,
		MaxHistory:      hc.Spec.MaxHistory,
		CreateNamespace: hc.Spec.CreateNamespace,
		DisableHooks:    hc.Spec.DisableHooks,
	}
	if hc.Spec.Timeout != nil && hc.Spec.Timeout.Duration > 0 {
		opts.Timeout = hc.Spec.Timeout.Duration
	}
	return opts
}
