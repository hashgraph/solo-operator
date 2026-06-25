// SPDX-License-Identifier: Apache-2.0

package applicationoverrideproperties

import "github.com/hashgraph/solo-operator/internal/common"

func ValidateContent(content string) error {
	return common.ValidateJavaProperties(content, "application-override.properties")
}
