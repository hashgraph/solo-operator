// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// capsuleRegistryChecker returns override when set (test seam), otherwise an OCIChecker
// whose keychain is built from the pull secrets of the single ConsensusCapsule named
// capsuleName — the node whose UC created the upgrade CR. Scoping to that one capsule means
// the operator's verification pulls use exactly the credentials the node's pod pulls with,
// so an image can't verify-pass then ImagePullBackOff on that node, and no unrelated
// capsule's (possibly stale) credential pollutes verification.
//
// It is best-effort: a missing capsule (e.g. scaled down mid-upgrade) or a malformed pull
// secret must not block verification — least of all for public images — so it falls back to
// authn.DefaultKeychain rather than failing the reconcile.
func capsuleRegistryChecker(ctx context.Context, c client.Client, override registry.ImageChecker, namespace, capsuleName string, allowedHosts []string) (registry.ImageChecker, error) {
	if override != nil {
		return override, nil
	}
	refs, err := capsulePullSecretRefs(ctx, c, namespace, capsuleName)
	if err != nil {
		return nil, err
	}
	kc, err := registry.KeychainFromPullSecrets(ctx, c, namespace, refs)
	if err != nil {
		// A malformed pull secret must not fail the reconcile (which would infinite-backoff
		// and block even public upgrades). Log it and fall back to the default keychain.
		common.Warn(ctx, ReasonKeychainBuildFailed).AnErr("error", err).
			Msg("building registry keychain from capsule pull secrets failed; falling back to default keychain")
		return registry.OCIChecker{AllowedHosts: allowedHosts}, nil
	}
	return registry.OCIChecker{Keychain: kc, AllowedHosts: allowedHosts}, nil
}

// capsulePullSecretRefs is the deduplicated union of the named ConsensusCapsule's
// enabled-container image-pull secrets. A missing capsule yields nil (KeychainFromPullSecrets
// then falls back to the default keychain); other Get errors are returned to requeue.
func capsulePullSecretRefs(ctx context.Context, c client.Client, namespace, capsuleName string) ([]corev1.LocalObjectReference, error) {
	var capsule operatorv1alpha1.ConsensusCapsule
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: capsuleName}, &capsule); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting ConsensusCapsule %s for registry pull secrets", capsuleName), ReasonGetFailed),
			"Confirm the operator has get permission on ConsensusCapsules",
		)
	}
	return common.MergeImagePullSecrets(capsulePullSecretGroups(&capsule)...), nil
}

// capsulePullSecretGroups collects the image-pull secret refs of every container that will
// actually run in a ConsensusCapsule — the consensus node plus each ENABLED sidecar/uploader
// — from both the single SoftwareVersion and any SoftwareVersionSource candidate
// repositories. Disabled containers are skipped: the kubelet never uses their pull secrets,
// so harvesting them would pollute the verification keychain (see the enabled gate mirrored
// in sourcedContainers).
func capsulePullSecretGroups(capsule *operatorv1alpha1.ConsensusCapsule) [][]corev1.LocalObjectReference {
	ct := &capsule.Spec.PodProperties.Containers
	var groups [][]corev1.LocalObjectReference
	addVersion := func(sv *operatorv1alpha1.SoftwareVersion) {
		if sv != nil && len(sv.ImagePullSecrets) > 0 {
			groups = append(groups, sv.ImagePullSecrets)
		}
	}
	addSource := func(src *operatorv1alpha1.SoftwareVersionSource) {
		if src == nil {
			return
		}
		for i := range src.ImageRepositories {
			if len(src.ImageRepositories[i].ImagePullSecrets) > 0 {
				groups = append(groups, src.ImageRepositories[i].ImagePullSecrets)
			}
		}
	}
	// The consensus node always runs.
	addVersion(ct.ConsensusNode.SoftwareVersion)
	addSource(ct.ConsensusNode.SoftwareVersionSource)
	if u := ct.RecordsUploader; u != nil && u.Enabled {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.EventsUploader; u != nil && u.Enabled {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.BlocksUploader; u != nil && u.Enabled {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.BackupUploader; u != nil && u.Enabled {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if uc := ct.UC; uc != nil && uc.Enabled {
		addVersion(uc.SoftwareVersion)
		addSource(uc.SoftwareVersionSource)
	}
	if a := ct.Alloy; a != nil && a.Enabled {
		addVersion(a.SoftwareVersion)
		addSource(a.SoftwareVersionSource)
	}
	return groups
}
