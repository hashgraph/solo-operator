// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// BuildImage constructs a full image reference from a SoftwareVersion, or "" when sv is nil
// or has no ImageName. The operator ships no built-in default images, so callers treat an
// empty result as "no image declared".
// Format: repository/imageName:imageTag[@digest], repository/imageName, or imageName:imageTag.
// When Digest is set, it is appended as "@sha256:..." so the reference is pinned to an
// immutable content digest (the tag is retained for readability); an unset Digest yields
// a tag-only reference.
func BuildImage(sv *operatorv1alpha1.SoftwareVersion) string {
	if sv == nil || sv.ImageName == "" {
		return ""
	}

	image := sv.ImageName
	if sv.Repository != "" {
		image = sv.Repository + "/" + image
	}
	if sv.ImageTag != "" {
		image = image + ":" + sv.ImageTag
	}
	return pinDigest(image, sv.Digest)
}

// pinDigest appends an OCI content digest to an image reference, yielding an immutable
// pinned reference (ref@sha256:...). An empty digest returns ref unchanged (a mutable,
// tag-only reference). This is the single place that pins a reference to a digest, shared
// by BuildImage and PinnedRef so every code path pins identically.
func pinDigest(ref string, digest operatorv1alpha1.Sha256Digest) string {
	if digest == "" {
		return ref
	}
	return ref + "@" + string(digest)
}

// PinnedRef returns a SelectedSource's image pinned to its verified content digest
// (repo/name:tag@sha256:...), so callers deploy or prefetch the exact image verified
// during selection. It falls back to the tag alone when no digest is present — which
// should not occur for a resolved source, since selection always resolves a digest.
func PinnedRef(src *operatorv1alpha1.SelectedSource) string {
	return pinDigest(src.Image, src.Digest)
}

// PullPolicyOrDefault returns the pull policy from a SoftwareVersion, or def if unset.
func PullPolicyOrDefault(sv *operatorv1alpha1.SoftwareVersion, def corev1.PullPolicy) corev1.PullPolicy {
	if sv != nil && sv.ImagePullPolicy != "" {
		return corev1.PullPolicy(sv.ImagePullPolicy)
	}
	return def
}

// ImagePullSecretsFromVersion returns the ImagePullSecrets from a SoftwareVersion, or nil.
func ImagePullSecretsFromVersion(sv *operatorv1alpha1.SoftwareVersion) []corev1.LocalObjectReference {
	if sv == nil {
		return nil
	}
	return sv.ImagePullSecrets
}

// MergeImagePullSecrets deduplicates LocalObjectReferences by name across multiple slices.
func MergeImagePullSecrets(sources ...[]corev1.LocalObjectReference) []corev1.LocalObjectReference {
	seen := map[string]struct{}{}
	var merged []corev1.LocalObjectReference
	for _, refs := range sources {
		for _, ref := range refs {
			if _, ok := seen[ref.Name]; ok {
				continue
			}
			seen[ref.Name] = struct{}{}
			merged = append(merged, ref)
		}
	}
	return merged
}

// ParseImage splits a full OCI image reference "[[repo/]name[:tag]]" into its three components.
// The last colon is only the tag separator if no slash follows (distinguishes tag from port).
func ParseImage(image string) (repository, name, tag string) {
	ref := image
	if idx := strings.LastIndexByte(ref, ':'); idx > 0 {
		if strings.IndexByte(ref[idx+1:], '/') < 0 {
			tag = ref[idx+1:]
			ref = ref[:idx]
		}
	}
	if idx := strings.LastIndexByte(ref, '/'); idx >= 0 {
		repository = ref[:idx]
		name = ref[idx+1:]
	} else {
		name = ref
	}
	return
}
