// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// releaseCheckInterval is how often we recheck a deployed release to catch external drift (e.g. helm rollback).
const releaseCheckInterval = 5 * time.Minute

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
		return ctrl.Result{}, fmt.Errorf("%w: %w", ErrValuesBuild, err)
	}

	hash, err := ComputeHash(hc.Spec.Chart, values)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hash == hc.Status.ValuesHash {
		existing, err := r.helmClient.GetRelease(releaseName, hc.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("checking existing release: %w", err)
		}
		if existing != nil && existing.Status == "deployed" {
			logger.V(1).Info("Release up-to-date, skipping upgrade")
			return ctrl.Result{RequeueAfter: releaseCheckInterval}, nil
		}
		if existing != nil {
			logger.Info("Release not in deployed state, retrying upgrade", "helmStatus", existing.Status)
		}
	}

	opts := releaseOptions(hc)
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}

	if hc.Spec.Chart.CredentialsRef != nil {
		creds, err := r.loadChartCredentials(ctx, hc.Namespace, hc.Spec.Chart.CredentialsRef.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("%w: loading chart credentials from %s: %w", ErrChartFetch, hc.Spec.Chart.CredentialsRef.Name, err)
		}
		opts.Credentials = creds
	}

	// Mark Running before the helm call — it can block for minutes.
	// Retry on conflict: the informer cache may serve a stale resourceVersion.
	if hc.Status.Phase != operatorv1alpha1.PhaseRunning {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Client.Get(ctx, types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, hc); err != nil {
				return err
			}
			if hc.Status.Phase == operatorv1alpha1.PhaseRunning {
				return nil
			}
			hc.Status.Phase = operatorv1alpha1.PhaseRunning
			return r.Client.Status().Update(ctx, hc)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciling Helm release", "timeout", opts.Timeout)

	info, err := r.helmClient.InstallOrUpgrade(ctx, releaseName, hc.Namespace, hc.Spec.Chart, values, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("helm install/upgrade %s: %w", releaseName, err)
	}

	logger.Info("Helm release reconciled", "revision", info.Version, "status", info.Status)

	// Persist the Helm status atomically with retry so that ValuesHash is
	// always saved even when a concurrent spec update races with us.
	// Without this, a conflict in the caller's updateStatus would leave
	// ValuesHash unpersisted, causing a spurious re-upgrade on the next reconcile.
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: hc.Name, Namespace: hc.Namespace}, hc); err != nil {
			return err
		}
		hc.Status.ReleaseVersion = info.Version
		hc.Status.InstalledChart = info.ChartString
		hc.Status.ReleaseStatus = info.Status
		if info.Status == "deployed" {
			hc.Status.ValuesHash = hash // only save hash once deployed so failures are retried
		}
		return r.Client.Status().Update(ctx, hc)
	}); err != nil {
		return ctrl.Result{}, err
	}

	if info.Status != "deployed" {
		logger.Info("Helm release not yet deployed, requeueing", "helmStatus", info.Status)
	}
	return ctrl.Result{RequeueAfter: releaseCheckInterval}, nil
}

// checkReleaseNameConflict returns an error if another HelmCapsule in the namespace already owns this release name.
// This performs a List API call on every reconcile. At the expected scale (single-digit HelmCapsules per namespace)
// the cost is negligible. If namespaces grow large, replace with a field indexer on spec.releaseName.
func (r *ReleaseSubReconciler) checkReleaseNameConflict(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, releaseName string) error {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.Client.List(ctx, list, client.InNamespace(hc.Namespace)); err != nil {
		return fmt.Errorf("listing HelmCapsules: %w", err)
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

func (r *ReleaseSubReconciler) loadChartCredentials(ctx context.Context, namespace, secretName string) (*ChartCredentials, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err != nil {
		return nil, err
	}
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	if username == "" || password == "" {
		return nil, fmt.Errorf("secret %s must have non-empty \"username\" and \"password\" keys", secretName)
	}
	return &ChartCredentials{Username: username, Password: password}, nil
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
