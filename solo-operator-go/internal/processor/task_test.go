// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockStep is a mock implementation of ProcessorStep for testing
type mockStep struct {
	name       string
	successful bool
	skip       bool
	runCalled  bool
}

func (m *mockStep) Name() string {
	return m.name
}

func (m *mockStep) Run(ctx context.Context, resource *corev1.ConfigMap, status string) string {
	m.runCalled = true
	if m.successful {
		return "success"
	}
	return "failed"
}

func (m *mockStep) IsSuccessful(resource *corev1.ConfigMap, status string) bool {
	return m.successful
}

func (m *mockStep) ShouldSkip(resource *corev1.ConfigMap, status string) bool {
	return m.skip
}

func TestBaseTask_Execute(t *testing.T) {
	tests := []struct {
		name           string
		steps          []ProcessorStep[*corev1.ConfigMap, string]
		expectedStatus string
		stepsRun       int
	}{
		{
			name: "all steps succeed",
			steps: []ProcessorStep[*corev1.ConfigMap, string]{
				&mockStep{name: "step1", successful: true},
				&mockStep{name: "step2", successful: true},
			},
			expectedStatus: "success",
			stepsRun:       2,
		},
		{
			name: "first step fails",
			steps: []ProcessorStep[*corev1.ConfigMap, string]{
				&mockStep{name: "step1", successful: false},
				&mockStep{name: "step2", successful: true},
			},
			expectedStatus: "failed",
			stepsRun:       1,
		},
		{
			name: "skip step",
			steps: []ProcessorStep[*corev1.ConfigMap, string]{
				&mockStep{name: "step1", successful: true, skip: true},
				&mockStep{name: "step2", successful: true},
			},
			expectedStatus: "success",
			stepsRun:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewBaseTask(
				tt.steps,
				func(cm *corev1.ConfigMap) string { return "initial" },
			)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}

			result, err := task.Execute(context.Background(), cm)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %q, got %q", tt.expectedStatus, result.Status)
			}

			runCount := 0
			for _, step := range tt.steps {
				if step.(*mockStep).runCalled {
					runCount++
				}
			}

			if runCount != tt.stepsRun {
				t.Errorf("Expected %d steps to run, got %d", tt.stepsRun, runCount)
			}
		})
	}
}

func TestBaseTask_ComputeReconcileDelay(t *testing.T) {
	task := NewBaseTask[*corev1.ConfigMap, string](nil, nil)

	cm := &corev1.ConfigMap{}
	delay := task.ComputeReconcileDelay(cm, "status")

	if delay != DefaultReconcileDelay {
		t.Errorf("Expected %v, got %v", DefaultReconcileDelay, delay)
	}
}

func TestBaseTask_ComputeEstimatedProcessingTime(t *testing.T) {
	task := NewBaseTask[*corev1.ConfigMap, string](nil, nil)

	cm := &corev1.ConfigMap{}
	processingTime := task.ComputeEstimatedProcessingTime(cm, "status")

	if processingTime != DefaultEstimatedProcessingTime {
		t.Errorf("Expected %v, got %v", DefaultEstimatedProcessingTime, processingTime)
	}
}

func TestNewResult(t *testing.T) {
	result := NewResult("status", 5*time.Second, 30*time.Second, nil)

	if result.Status != "status" {
		t.Errorf("Expected status 'status', got %q", result.Status)
	}
	if result.ReconcileDelay != 5*time.Second {
		t.Errorf("Expected reconcile delay 5s, got %v", result.ReconcileDelay)
	}
	if result.EstimatedProcessingTime != 30*time.Second {
		t.Errorf("Expected processing time 30s, got %v", result.EstimatedProcessingTime)
	}
	if result.DeferredProcessor != nil {
		t.Error("Expected nil deferred processor")
	}
}
