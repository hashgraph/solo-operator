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

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
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

	rl := common.LoggerFrom(ctx).With().
		Str("release", releaseName).
		Str("chart", hc.Spec.Chart.Chart).
		Str("version", hc.Spec.Chart.Version).
		Logger()

	if err := r.checkReleaseNameConflict(ctx, hc, releaseName); err != nil {
		return ctrl.Result{}, err
	}

	values, err := MergeValues(ctx, r.Client, hc.Namespace, &hc.Spec)
	if err != nil {
		return ctrl.Result{}, errx.WithReason(errValuesBuild.Wrap(err, "building release values"), ReasonValuesBuild)
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
			common.DebugL(&rl, ReasonReleaseUpToDate).Msg("Release up-to-date, skipping upgrade")
			return ctrl.Result{RequeueAfter: releaseCheckInterval}, nil
		}
		if existing != nil {
			common.InfoL(&rl, ReasonReleaseRetry).Str("helmStatus", existing.Status).Msg("Release not in deployed state, retrying upgrade")
		}
	}

	opts := releaseOptions(hc)
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}

	if hc.Spec.Chart.CredentialsRef != nil {
		creds, err := r.loadChartCredentials(ctx, hc.Namespace, hc.Spec.Chart.CredentialsRef.Name)
		if err != nil {
			return ctrl.Result{}, errx.WithHints(
				errx.WithReason(errChartFetch.Wrap(err, "loading chart credentials from %s", hc.Spec.Chart.CredentialsRef.Name), ReasonChartFetch),
				"Confirm the Secret named in spec.chart.credentialsRef exists and has username/password keys",
			)
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
			return common.WarnOnConflict(ctx, r.Client.Status().Update(ctx, hc))
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	common.InfoL(&rl, ReasonReleaseReconciling).Dur("timeout", opts.Timeout).Msg("Reconciling Helm release")

	info, err := r.helmClient.InstallOrUpgrade(ctx, releaseName, hc.Namespace, hc.Spec.Chart, values, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("helm install/upgrade %s: %w", releaseName, err)
	}

	common.InfoL(&rl, ReasonReleaseReconciled).Int("revision", info.Version).Str("status", info.Status).Msg("Helm release reconciled")

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
		return common.WarnOnConflict(ctx, r.Client.Status().Update(ctx, hc))
	}); err != nil {
		return ctrl.Result{}, err
	}

	if info.Status != "deployed" {
		common.InfoL(&rl, ReasonReleasePending).Str("helmStatus", info.Status).Msg("Helm release not yet deployed, requeueing")
	}
	return ctrl.Result{RequeueAfter: releaseCheckInterval}, nil
}

// checkReleaseNameConflict returns an error if another HelmCapsule in the namespace owns this release name.
// Does a List per reconcile — negligible at expected scale; replace with a releaseName indexer if needed.
func (r *ReleaseSubReconciler) checkReleaseNameConflict(ctx context.Context, hc *operatorv1alpha1.HelmCapsule, releaseName string) error {
	list := &operatorv1alpha1.HelmCapsuleList{}
	if err := r.Client.List(ctx, list, client.InNamespace(hc.Namespace)); err != nil {
		return errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing HelmCapsules"), ReasonListFailed),
			"Confirm the ServiceAccount has list permission on helmcapsules.operator.solo.hedera.com",
		)
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
			return errx.WithHints(
				errx.WithReason(
					errReleaseNameConflict.New("release %q is already owned by HelmCapsule %s (in namespace %s)",
						releaseName, other.Name, hc.Namespace),
					ReasonReleaseNameConflict,
				),
				"Set a unique spec.releaseName, or delete the conflicting HelmCapsule",
			)
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
		return nil, errx.WithHints(
			errx.WithReason(errChartFetch.New("secret %s must have non-empty \"username\" and \"password\" keys", secretName), ReasonCredentials),
			"Populate both the username and password keys in the credentials Secret",
		)
	}
	return &ChartCredentials{Username: username, Password: password}, nil
}

func releaseOptions(hc *operatorv1alpha1.HelmCapsule) ReleaseOptions {
	opts := ReleaseOptions{
		Wait:          hc.Spec.Wait,
		Atomic:        hc.Spec.Atomic,
		CleanupOnFail: hc.Spec.CleanupOnFail,
		MaxHistory:    hc.Spec.MaxHistory,
		DisableHooks:  hc.Spec.DisableHooks,
	}
	if hc.Spec.Timeout != nil && hc.Spec.Timeout.Duration > 0 {
		opts.Timeout = hc.Spec.Timeout.Duration
	}
	return opts
}
