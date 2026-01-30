// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestUpgradeStatus_AddCondition(t *testing.T) {
	status := &UpgradeStatus{
		Phase:      operatorv1alpha1.PhasePending,
		Conditions: []metav1.Condition{},
	}

	now := metav1.Now()
	condition := metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNode,
		Message:            "Test message",
	}

	status.AddCondition(condition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(status.Conditions))
	}

	// Add another condition with same type and reason - should replace
	newCondition := metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonConsensusNode,
		Message:            "New message",
	}

	status.AddCondition(newCondition)

	if len(status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after replacement, got %d", len(status.Conditions))
	}

	// Add condition with same type but different reason - should add
	anotherCondition := metav1.Condition{
		Type:               StepConditionTypeVerifyChangeset,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonRecordStreamUploader,
		Message:            "Different reason",
	}

	status.AddCondition(anotherCondition)

	if len(status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(status.Conditions))
	}
}

func TestUpgradeStatus_HasSuccessfulCondition(t *testing.T) {
	status := &UpgradeStatus{
		Phase: operatorv1alpha1.PhaseRunning,
		Conditions: []metav1.Condition{
			{
				Type:   StepConditionTypeVerifyChangeset,
				Status: metav1.ConditionTrue,
				Reason: StepConditionReasonConsensusNode,
			},
		},
	}

	if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonConsensusNode) {
		t.Error("Expected to find successful condition")
	}

	if status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonRecordStreamUploader) {
		t.Error("Expected not to find non-existent condition")
	}

	// Test with ConditionFalse
	status.Conditions[0].Status = metav1.ConditionFalse
	if status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonConsensusNode) {
		t.Error("Expected not to find failed condition as successful")
	}
}

func TestHasStreamUploaderChangeset(t *testing.T) {
	tests := []struct {
		name         string
		op           *operatorv1alpha1.NetworkOperation
		uploaderType StreamUploaderType
		expected     bool
	}{
		{
			name:         "nil network upgrade",
			op:           &operatorv1alpha1.NetworkOperation{},
			uploaderType: StreamUploaderTypeRecordStream,
			expected:     false,
		},
		{
			name: "nil changesets",
			op: &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{},
				},
			},
			uploaderType: StreamUploaderTypeRecordStream,
			expected:     false,
		},
		{
			name: "has record stream changeset",
			op: &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
							RecordStreamUploaderChangeset: &operatorv1alpha1.StreamUploaderChangeset{},
						},
					},
				},
			},
			uploaderType: StreamUploaderTypeRecordStream,
			expected:     true,
		},
		{
			name: "no record stream changeset",
			op: &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
							EventsStreamUploaderChangeset: &operatorv1alpha1.StreamUploaderChangeset{},
						},
					},
				},
			},
			uploaderType: StreamUploaderTypeRecordStream,
			expected:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hasStreamUploaderChangeset(tc.op, tc.uploaderType)
			if result != tc.expected {
				t.Errorf("hasStreamUploaderChangeset() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestHasOtelChangeset(t *testing.T) {
	tests := []struct {
		name     string
		op       *operatorv1alpha1.NetworkOperation
		expected bool
	}{
		{
			name:     "nil network upgrade",
			op:       &operatorv1alpha1.NetworkOperation{},
			expected: false,
		},
		{
			name: "has otel changeset",
			op: &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
							OtelCollector: &operatorv1alpha1.OtelCollectorChangeset{},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "no otel changeset",
			op: &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						Changesets: operatorv1alpha1.NetworkUpgradeChangesets{},
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hasOtelChangeset(tc.op)
			if result != tc.expected {
				t.Errorf("hasOtelChangeset() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestGetStreamUploaderConditionReason(t *testing.T) {
	tests := []struct {
		uploaderType StreamUploaderType
		expected     string
	}{
		{StreamUploaderTypeRecordStream, StepConditionReasonRecordStreamUploader},
		{StreamUploaderTypeSidecarFiles, StepConditionReasonSidecarFilesUploader},
		{StreamUploaderTypeEventStream, StepConditionReasonEventStreamUploader},
		{StreamUploaderTypeBackup, StepConditionReasonBackupUploader},
	}

	for _, tc := range tests {
		t.Run(string(tc.uploaderType), func(t *testing.T) {
			result := getStreamUploaderConditionReason(tc.uploaderType)
			if result != tc.expected {
				t.Errorf("getStreamUploaderConditionReason(%s) = %s, expected %s", tc.uploaderType, result, tc.expected)
			}
		})
	}
}

func TestNewProcessor(t *testing.T) {
	p := NewProcessor(nil)

	if p == nil {
		t.Fatal("Expected processor to be created")
	}

	if p.executeImmediateTask == nil {
		t.Error("Expected executeImmediateTask to be initialized")
	}

	if p.freezeScheduledTask == nil {
		t.Error("Expected freezeScheduledTask to be initialized")
	}

	if p.nowFrozenTask == nil {
		t.Error("Expected nowFrozenTask to be initialized")
	}

	if p.freezeAbortTask == nil {
		t.Error("Expected freezeAbortTask to be initialized")
	}
}

func TestNewExecuteImmediateTask(t *testing.T) {
	task := NewExecuteImmediateTask(nil)

	if task == nil {
		t.Fatal("Expected task to be created")
	}

	// Verify the correct number of steps
	if len(task.steps) != 6 {
		t.Errorf("Expected 6 steps, got %d", len(task.steps))
	}

	// Verify step names
	expectedNames := []string{
		"VerifyConsensusNodeChangeset",
		"VerifyStreamUploaderChangeset-recordStreamUploader",
		"VerifyStreamUploaderChangeset-recordStreamSidecarFilesUploader",
		"VerifyStreamUploaderChangeset-eventsStreamUploader",
		"VerifyStreamUploaderChangeset-backupUploader",
		"VerifyOtelCollectorChangeset",
	}

	for i, step := range task.steps {
		if step.Name() != expectedNames[i] {
			t.Errorf("Step %d: expected name %q, got %q", i, expectedNames[i], step.Name())
		}
	}
}
