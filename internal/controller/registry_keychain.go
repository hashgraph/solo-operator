// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// namespaceRegistryChecker returns override when set (test seam), otherwise an OCIChecker
// whose keychain is built from the docker-registry pull secrets referenced by the
// ConsensusCapsules in namespace. This lets the operator's verification pulls inspect
// images in private/authenticated registries — the same credentials the kubelet uses to
// pull them — so candidates in those registries can be selected and verified. When no pull
// secrets are present it falls back to authn.DefaultKeychain, preserving public-registry
// behavior.
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
		return nil, err
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
	var groups [][]corev1.LocalObjectReference
	for i := range capsules.Items {
		groups = append(groups, capsulePullSecretGroups(&capsules.Items[i])...)
	}
	return common.MergeImagePullSecrets(groups...), nil
}

// capsulePullSecretGroups collects the image-pull secret refs of every container in a
// ConsensusCapsule, from both the single SoftwareVersion and any SoftwareVersionSource
// candidate repositories.
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
	addVersion(ct.ConsensusNode.SoftwareVersion)
	addSource(ct.ConsensusNode.SoftwareVersionSource)
	if u := ct.RecordsUploader; u != nil {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.EventsUploader; u != nil {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.BlocksUploader; u != nil {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if u := ct.BackupUploader; u != nil {
		addVersion(u.SoftwareVersion)
		addSource(u.SoftwareVersionSource)
	}
	if uc := ct.UC; uc != nil {
		addVersion(uc.SoftwareVersion)
		addSource(uc.SoftwareVersionSource)
	}
	if a := ct.Alloy; a != nil {
		addVersion(a.SoftwareVersion)
		addSource(a.SoftwareVersionSource)
	}
	return groups
}
