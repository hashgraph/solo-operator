// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultReconcileDelay is the default delay between reconciliations
	DefaultReconcileDelay = 5 * time.Second

	// DefaultEstimatedProcessingTime is the default estimated processing time
	DefaultEstimatedProcessingTime = 30 * time.Second
)

// StatusGetter is a function that gets the current status from a resource
type StatusGetter[T client.Object, S any] func(T) S

// BaseTask provides common functionality for processor tasks.
type BaseTask[T client.Object, S any] struct {
	steps        []ProcessorStep[T, S]
	getStatus    StatusGetter[T, S]
	deferredFunc func(ctx context.Context, resource T) Processor[T, S]
}

// NewBaseTask creates a new BaseTask with the given steps and status getter
func NewBaseTask[T client.Object, S any](
	steps []ProcessorStep[T, S],
	getStatus StatusGetter[T, S],
) *BaseTask[T, S] {
	return &BaseTask[T, S]{
		steps:     steps,
		getStatus: getStatus,
	}
}

// WithDeferredFunc sets the deferred processor function
func (t *BaseTask[T, S]) WithDeferredFunc(fn func(ctx context.Context, resource T) Processor[T, S]) *BaseTask[T, S] {
	t.deferredFunc = fn
	return t
}

// Steps returns the list of steps to execute
func (t *BaseTask[T, S]) Steps() []ProcessorStep[T, S] {
	return t.steps
}

// ComputeReconcileDelay computes the recommended delay before the next reconciliation.
// This default implementation returns 5 seconds.
func (t *BaseTask[T, S]) ComputeReconcileDelay(resource T, status S) time.Duration {
	return DefaultReconcileDelay
}

// ComputeEstimatedProcessingTime computes the estimated processing time.
// This default implementation returns 30 seconds.
func (t *BaseTask[T, S]) ComputeEstimatedProcessingTime(resource T, status S) time.Duration {
	return DefaultEstimatedProcessingTime
}

// ComputeDeferredProcessor checks if the task should be deferred to another processor.
// Returns nil if processing should continue with the current processor.
func (t *BaseTask[T, S]) ComputeDeferredProcessor(ctx context.Context, resource T) Processor[T, S] {
	if t.deferredFunc != nil {
		return t.deferredFunc(ctx, resource)
	}
	return nil
}

// Execute orchestrates the execution of steps and determines the new status based on the results.
func (t *BaseTask[T, S]) Execute(ctx context.Context, resource T) (*Result[S], error) {
	logger := log.FromContext(ctx)
	current := t.getStatus(resource)

	// Check if processing should be deferred
	deferred := t.ComputeDeferredProcessor(ctx, resource)
	if deferred != nil {
		return NewResult(
			current,
			t.ComputeReconcileDelay(resource, current),
			t.ComputeEstimatedProcessingTime(resource, current),
			deferred,
		), nil
	}

	// Process the steps and determine the new status
	for _, step := range t.steps {
		if step.ShouldSkip(resource, current) {
			logger.V(1).Info("Skipping step", "step", step.Name())
			continue
		}

		logger.V(1).Info("Running step", "step", step.Name())
		current = step.Run(ctx, resource, current)

		if !step.IsSuccessful(resource, current) {
			logger.Info("Step not successful, stopping execution", "step", step.Name())
			break
		}
	}

	return NewResult(
		current,
		t.ComputeReconcileDelay(resource, current),
		t.ComputeEstimatedProcessingTime(resource, current),
		nil,
	), nil
}

// Process implements the Processor interface
func (t *BaseTask[T, S]) Process(ctx context.Context, resource T) (*Result[S], error) {
	return t.Execute(ctx, resource)
}
