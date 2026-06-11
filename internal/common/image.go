// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"

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

// ParseImage is the inverse of BuildImage: it splits a full OCI image reference
// "[[repo/]name[:tag]]" into its three components.
func ParseImage(image string) (repository, name, tag string) {
	ref := image
	// Peel off the tag: last colon that is not inside a port number (no slash after it).
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
