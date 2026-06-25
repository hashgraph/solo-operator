// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import "github.com/hashgraph/solo-operator/internal/common"

func ValidateContent(content string) error {
	return common.ValidateJSON(content, "throttles.json")
}
