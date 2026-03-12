// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// BuildImage constructs a full image reference from a SoftwareVersion
// Format: repository/imageName:imageTag
func BuildImage(sv operatorv1alpha1.SoftwareVersion) string {
	return fmt.Sprintf("%s/%s:%s", sv.Repository, sv.ImageName, sv.ImageTag)
}

// BuildImageOrDefault constructs an image reference from SoftwareVersion or returns the default
func BuildImageOrDefault(sv *operatorv1alpha1.SoftwareVersion, defaultImage string) string {
	if sv == nil {
		return defaultImage
	}
	return BuildImage(*sv)
}
