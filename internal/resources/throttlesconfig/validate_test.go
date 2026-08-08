// SPDX-License-Identifier: Apache-2.0

package throttlesconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fullThrottlesJSON(ops ...string) string {
	doc := throttlesDoc{
		Buckets: []bucket{{
			ThrottleGroups: []throttleGroup{{
				Operations: ops,
			}},
		}},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func TestValidateThrottleOps_AllPresent(t *testing.T) {
	content := fullThrottlesJSON(requiredOps...)
	missing, err := ValidateThrottleOps(content)
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestValidateThrottleOps_MissingQuery(t *testing.T) {
	ops := make([]string, 0, len(requiredOps)-1)
	for _, op := range requiredOps {
		if op != "TransactionGetReceipt" {
			ops = append(ops, op)
		}
	}
	missing, err := ValidateThrottleOps(fullThrottlesJSON(ops...))
	require.NoError(t, err)
	assert.Equal(t, []string{"TransactionGetReceipt"}, missing)
}

func TestValidateThrottleOps_MissingFileTx(t *testing.T) {
	ops := make([]string, 0, len(requiredOps)-1)
	for _, op := range requiredOps {
		if op != "FileCreate" {
			ops = append(ops, op)
		}
	}
	missing, err := ValidateThrottleOps(fullThrottlesJSON(ops...))
	require.NoError(t, err)
	assert.Equal(t, []string{"FileCreate"}, missing)
}

func TestValidateThrottleOps_NoBuckets(t *testing.T) {
	missing, err := ValidateThrottleOps(`{}`)
	require.NoError(t, err)
	assert.Len(t, missing, len(requiredOps))
}

func TestValidateThrottleOps_EmptyBuckets(t *testing.T) {
	missing, err := ValidateThrottleOps(`{"buckets":[]}`)
	require.NoError(t, err)
	assert.Len(t, missing, len(requiredOps))
}

func TestValidateThrottleOps_SpreadAcrossBuckets(t *testing.T) {
	half := len(requiredOps) / 2
	doc := throttlesDoc{
		Buckets: []bucket{
			{ThrottleGroups: []throttleGroup{{Operations: requiredOps[:half]}}},
			{ThrottleGroups: []throttleGroup{{Operations: requiredOps[half:]}}},
		},
	}
	b, _ := json.Marshal(doc)
	missing, err := ValidateThrottleOps(string(b))
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestValidateThrottleOps_InvalidJSON(t *testing.T) {
	_, err := ValidateThrottleOps(`not json`)
	assert.Error(t, err)
}
