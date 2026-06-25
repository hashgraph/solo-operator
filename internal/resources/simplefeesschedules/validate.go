// SPDX-License-Identifier: Apache-2.0

package simplefeesschedules

import "github.com/hashgraph/solo-operator/internal/common"

func ValidateContent(content string) error {
	return common.ValidateJSON(content, "simpleFeesSchedules.json")
}
