// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// BuildImage constructs a full image reference from a SoftwareVersion
// Format: repository/imageName:imageTag, repository/imageName, or imageName:imageTag
func BuildImage(sv operatorv1alpha1.SoftwareVersion) string {
	if sv.ImageName == "" {
		return ""
	}

	image := sv.ImageName
	if sv.Repository != "" {
		image = sv.Repository + "/" + image
	}
	if sv.ImageTag != "" {
		image = image + ":" + sv.ImageTag
	}
	return image
}

// BuildImageOrDefault constructs an image reference from SoftwareVersion or returns the default
func BuildImageOrDefault(sv *operatorv1alpha1.SoftwareVersion, defaultImage string) string {
	if sv == nil {
		return defaultImage
	}
	if image := BuildImage(*sv); image != "" {
		return image
	}
	return defaultImage
}

// PullPolicyOrDefault returns the pull policy from a SoftwareVersion, or def if unset.
func PullPolicyOrDefault(sv *operatorv1alpha1.SoftwareVersion, def corev1.PullPolicy) corev1.PullPolicy {
	if sv != nil && sv.ImagePullPolicy != "" {
		return corev1.PullPolicy(sv.ImagePullPolicy)
	}
	return def
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
