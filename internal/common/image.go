// SPDX-License-Identifier: Apache-2.0

package common

import (
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
