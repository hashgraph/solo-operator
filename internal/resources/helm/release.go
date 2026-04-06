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

type ReleaseSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HelmCapsule]
	helmClient Client
}

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

	if err := r.checkReleaseNameConflict(ctx, hc, releaseName); err != nil {
		return ctrl.Result{}, err
	}

	values, err := MergeValues(ctx, r.Client, hc.Namespace, &hc.Spec)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("%w: %s", ErrValuesBuild, err)
	}

	hash, err := ComputeHash(hc.Spec.Chart, values)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hash == hc.Status.ValuesHash {
		// Only skip if actually deployed — failed/pending releases must be retried.
		existing, err := r.helmClient.GetRelease(releaseName, hc.Namespace)
		if err == nil && existing != nil && existing.Status == "deployed" {
			logger.V(1).Info("Release up-to-date, skipping upgrade")
			return ctrl.Result{}, nil
		}
	}

	opts := releaseOptions(hc)
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
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

// checkReleaseNameConflict fails if another live HelmCapsule in the same namespace
// resolves to the same release name. The check is best-effort (TOCTOU under
// concurrent reconciles); a ValidatingWebhook provides stronger admission-time enforcement.
func (r *ReleaseSubReconciler) checkReleaseNameConflict(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, releaseName string) error {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.Client.List(ctx, list, client.InNamespace(hc.Namespace)); err != nil {
		return fmt.Errorf("%w: listing HelmCapsules: %w", ErrReleaseNameConflict, err)
	}
	for _, other := range list.Items {
		if other.UID == hc.UID || other.DeletionTimestamp != nil {
			continue
		}
		otherRelease := other.Spec.ReleaseName
		if otherRelease == "" {
			otherRelease = other.Name
		}
		if otherRelease == releaseName {
			return fmt.Errorf("%w: release %q is already owned by HelmCapsule %s (in namespace %s)",
				ErrReleaseNameConflict, releaseName, other.Name, hc.Namespace)
		}
	}
	return nil
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
