// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProcessorStep encapsulates the data and logic to execute a single step of a task.
// A ProcessorStep is a single unit of work that can be executed by a Processor.
// A ProcessorStep must not panic. If a ProcessorStep fails, it should handle errors
// and return the status with appropriate metadata.
type ProcessorStep[T client.Object, S any] interface {
	// Name returns the name of this step for logging purposes
	Name() string

	// Run executes the step and returns the updated status.
	// This method must never panic.
	Run(ctx context.Context, resource T, status S) S

	// IsSuccessful checks if the step completed successfully.
	IsSuccessful(resource T, status S) bool

	// ShouldSkip checks if the step should be skipped.
	// Step's Run method must not skip processing based on response from this method
	// as it is left to be handled by the Task and RetryStep to control the flow.
	ShouldSkip(resource T, status S) bool
}

// Processor encapsulates the data and logic to execute a single step of business process.
type Processor[T client.Object, S any] interface {
	// Process runs the business logic for the given resource and context.
	// Each processor can run a specific task at a time, where task has one or more steps.
	Process(ctx context.Context, resource T) (*Result[S], error)
}

// Result represents the outcome of processing a resource
type Result[S any] struct {
	// Status is the updated status after processing
	Status S

	// ReconcileDelay is the recommended delay before next reconciliation
	ReconcileDelay time.Duration

	// EstimatedProcessingTime is the estimated time to process this resource
	EstimatedProcessingTime time.Duration

	// DeferredProcessor is set if processing should be deferred to another processor
	DeferredProcessor interface{}
}

// NewResult creates a new Result with the given parameters
func NewResult[S any](status S, reconcileDelay, estimatedProcessingTime time.Duration, deferred interface{}) *Result[S] {
	return &Result[S]{
		Status:                  status,
		ReconcileDelay:          reconcileDelay,
		EstimatedProcessingTime: estimatedProcessingTime,
		DeferredProcessor:       deferred,
	}
}

// Task represents a task composed of multiple steps
type Task[T client.Object, S any] interface {
	// Steps returns the list of steps to execute
	Steps() []ProcessorStep[T, S]

	// ComputeReconcileDelay computes the recommended delay before the next reconciliation
	ComputeReconcileDelay(resource T, status S) time.Duration

	// ComputeEstimatedProcessingTime computes the estimated processing time
	ComputeEstimatedProcessingTime(resource T, status S) time.Duration

	// ComputeDeferredProcessor checks if the task should be deferred to another processor
	ComputeDeferredProcessor(ctx context.Context, resource T) Processor[T, S]

	// Execute runs all steps and returns the result
	Execute(ctx context.Context, resource T) (*Result[S], error)
}
