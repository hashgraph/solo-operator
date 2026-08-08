// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import (
	"encoding/json"
	"sort"
)

var requiredOps = []string{
	// Active query operations
	"ConsensusGetTopicInfo",
	"ContractCallLocal",
	"ContractGetBytecode",
	"ContractGetInfo",
	"CryptoGetAccountBalance",
	"CryptoGetAccountRecords",
	"CryptoGetInfo",
	"FileGetContents",
	"FileGetInfo",
	"GetVersionInfo",
	"ScheduleGetInfo",
	"TokenGetInfo",
	"TokenGetNftInfo",
	"TransactionGetRecord",
	"TransactionGetReceipt",

	// Operator-critical file transactions
	"FileAppend",
	"FileCreate",
	"FileUpdate",
}

type throttlesDoc struct {
	Buckets []bucket `json:"buckets"`
}

type bucket struct {
	ThrottleGroups []throttleGroup `json:"throttleGroups"`
}

type throttleGroup struct {
	Operations []string `json:"operations"`
}

// ValidateThrottleOps parses the throttles JSON and returns any required
// operations missing from all buckets. A nil/empty return means full coverage.
func ValidateThrottleOps(content string) ([]string, error) {
	var doc throttlesDoc
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return nil, err
	}

	present := make(map[string]struct{})
	for _, b := range doc.Buckets {
		for _, g := range b.ThrottleGroups {
			for _, op := range g.Operations {
				present[op] = struct{}{}
			}
		}
	}

	var missing []string
	for _, op := range requiredOps {
		if _, ok := present[op]; !ok {
			missing = append(missing, op)
		}
	}
	sort.Strings(missing)
	return missing, nil
}
