// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestValidateOperationIDLabel(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		specID  string
		wantOK  bool
		wantMsg string
	}{
		{
			name:   "present and matching",
			labels: map[string]string{operatorv1alpha1.LabelOperationID: "op-001"},
			specID: "op-001",
			wantOK: true,
		},
		{
			name:    "label absent",
			labels:  map[string]string{"other": "x"},
			specID:  "op-001",
			wantOK:  false,
			wantMsg: `operation-id label "operator.solo.hedera.com/operation-id" is required but missing`,
		},
		{
			name:    "nil label map",
			labels:  nil,
			specID:  "op-001",
			wantOK:  false,
			wantMsg: `operation-id label "operator.solo.hedera.com/operation-id" is required but missing`,
		},
		{
			name:    "label present but empty",
			labels:  map[string]string{operatorv1alpha1.LabelOperationID: ""},
			specID:  "op-001",
			wantOK:  false,
			wantMsg: `operation-id label "operator.solo.hedera.com/operation-id" is required but missing`,
		},
		{
			name:    "label mismatches spec",
			labels:  map[string]string{operatorv1alpha1.LabelOperationID: "op-999"},
			specID:  "op-001",
			wantOK:  false,
			wantMsg: `operation-id label "operator.solo.hedera.com/operation-id"="op-999" does not match spec.operationId "op-001"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := validateOperationIDLabel(tt.labels, tt.specID)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
