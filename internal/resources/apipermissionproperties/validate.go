// SPDX-License-Identifier: Apache-2.0

package apipermissionproperties

import "github.com/hashgraph/solo-operator/internal/common"

func ValidateContent(content string) error {
	return common.ValidateJavaProperties(content, "api-permission.properties")
}
