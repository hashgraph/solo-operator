// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sort"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// namespaceRegistryChecker returns override when set (test seam), otherwise an OCIChecker
// whose keychain is built from the docker-registry pull secrets referenced by the enabled
// containers of the ConsensusCapsules in namespace. This lets the operator's verification
// pulls inspect images in private/authenticated registries — the same credentials the
// kubelet uses to pull them — so candidates in those registries can be selected and
// verified. When no usable pull secrets are present it falls back to authn.DefaultKeychain,
// preserving public-registry behavior.
//
// Known limitations (keychain-by-convention model): the verification keychain is the union
// of the namespace's enabled-container pull secrets, which may be broader than the secrets
// any single pod carries. So (A) a present-but-stale credential for a host is used for that
// host even for a public image (verification can 401 where anonymous access would have
// worked), and (B) if capsules genuinely use different secrets for the same image, an image
// can verify here yet still ImagePullBackOff on a pod lacking that secret. Building the
// keychain from enabled containers only (below) keeps verification aligned with what pods
// actually pull in the common case; fully closing A/B (e.g. retry-anonymous on auth failure,
// or per-pod-scoped verification) is tracked separately.
func namespaceRegistryChecker(ctx context.Context, c client.Client, override registry.ImageChecker, namespace string, allowedHosts []string) (registry.ImageChecker, error) {
	if override != nil {
		return override, nil
	}
	refs, err := namespacePullSecretRefs(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	kc, err := registry.KeychainFromPullSecrets(ctx, c, namespace, refs)
	if err != nil {
		// A single malformed pull secret in the namespace must not block verification —
		// least of all for public images. Log it and fall back to the ambient/default
		// keychain rather than returning an error that would infinite-backoff the
		// reconcile and block even purely-public upgrades.
		common.Warn(ctx, ReasonKeychainBuildFailed).AnErr("error", err).
			Msg("building registry keychain from namespace pull secrets failed; falling back to default keychain")
		return registry.OCIChecker{AllowedHosts: allowedHosts}, nil
	}
	return registry.OCIChecker{Keychain: kc, AllowedHosts: allowedHosts}, nil
}

// namespacePullSecretRefs is the deduplicated union of every ConsensusCapsule container's
// image-pull secrets in namespace (from both SoftwareVersion and SoftwareVersionSource).
func namespacePullSecretRefs(ctx context.Context, c client.Client, namespace string) ([]corev1.LocalObjectReference, error) {
	var capsules operatorv1alpha1.ConsensusCapsuleList
	if err := c.List(ctx, &capsules, client.InNamespace(namespace)); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing ConsensusCapsules for registry pull secrets"), ReasonListFailed),
			"Confirm the operator has list permission on ConsensusCapsules",
		)
	}
	// c.List does not guarantee an order, so sort capsules by name before collecting.
	// MergeImagePullSecrets dedupes "first ref for a name wins" and the keychain applies
	// refs in order ("first secret for a host wins"); a stable input order makes both
	// deterministic across reconciles.
	sort.Slice(capsules.Items, func(i, j int) bool {
		return capsules.Items[i].GetName() < capsules.Items[j].GetName()
	})
	var groups [][]corev1.LocalObjectReference
	for i := range capsules.Items {
		groups = append(groups, capsulePullSecretGroups(&capsules.Items[i])...)
	}
	return common.MergeImagePullSecrets(groups...), nil
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
