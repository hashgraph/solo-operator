// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestValidateStepLabel(t *testing.T) {
	tests := []struct {
		name         string
		labels       map[string]string
		expectedStep string
		wantOK       bool
		wantMsg      string
	}{
		{
			name:         "present and matching",
			labels:       map[string]string{operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
			expectedStep: operatorv1alpha1.StepPrepare,
			wantOK:       true,
		},
		{
			name:         "label absent",
			labels:       map[string]string{"other": "x"},
			expectedStep: operatorv1alpha1.StepFreeze,
			wantOK:       false,
			wantMsg:      `step label "operator.solo.hiero.org/step" is required but missing`,
		},
		{
			name:         "nil label map",
			labels:       nil,
			expectedStep: operatorv1alpha1.StepExecute,
			wantOK:       false,
			wantMsg:      `step label "operator.solo.hiero.org/step" is required but missing`,
		},
		{
			name:         "label present but empty",
			labels:       map[string]string{operatorv1alpha1.LabelStep: ""},
			expectedStep: operatorv1alpha1.StepAbort,
			wantOK:       false,
			wantMsg:      `step label "operator.solo.hiero.org/step" is required but missing`,
		},
		{
			name:         "label mismatches expected step",
			labels:       map[string]string{operatorv1alpha1.LabelStep: operatorv1alpha1.StepFreeze},
			expectedStep: operatorv1alpha1.StepPrepare,
			wantOK:       false,
			wantMsg:      `step label "operator.solo.hiero.org/step"="freeze" does not match expected step "prepare"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := validateStepLabel(tt.labels, tt.expectedStep)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
