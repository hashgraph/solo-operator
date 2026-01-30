// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultRetryInterval is the default interval between retries
	DefaultRetryInterval = 2 * time.Second

	// DefaultMaxRetries is the default maximum number of retries
	DefaultMaxRetries = 30
)

// RetryStep wraps a step to execute it multiple times with a delay between each attempt.
type RetryStep[T client.Object, S any] struct {
	step          ProcessorStep[T, S]
	retryInterval time.Duration
	maxRetries    int
}

// NewRetryStep creates a new RetryStep with default settings (2 second interval, 30 retries = ~1 minute total).
func NewRetryStep[T client.Object, S any](step ProcessorStep[T, S]) *RetryStep[T, S] {
	return &RetryStep[T, S]{
		step:          step,
		retryInterval: DefaultRetryInterval,
		maxRetries:    DefaultMaxRetries,
	}
}

// NewRetryStepWithConfig creates a new RetryStep with custom retry settings.
func NewRetryStepWithConfig[T client.Object, S any](
	step ProcessorStep[T, S],
	retryInterval time.Duration,
	maxRetries int,
) *RetryStep[T, S] {
	if retryInterval <= 0 {
		retryInterval = DefaultRetryInterval
	}
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	return &RetryStep[T, S]{
		step:          step,
		retryInterval: retryInterval,
		maxRetries:    maxRetries,
	}
}

// Name returns the name of the wrapped step
func (r *RetryStep[T, S]) Name() string {
	return r.step.Name()
}

// Run executes the step with retry logic
func (r *RetryStep[T, S]) Run(ctx context.Context, resource T, status S) S {
	logger := log.FromContext(ctx)

	updated := r.step.Run(ctx, resource, status)

	retries := 0
	for !r.step.IsSuccessful(resource, updated) && retries < r.maxRetries {
		retries++

		logger.Info("Step failed, retrying",
			"step", r.step.Name(),
			"retryInterval", r.retryInterval.Seconds(),
			"retry", retries,
			"maxRetries", r.maxRetries,
			"namespace", resource.GetNamespace(),
			"name", resource.GetName(),
		)

		// Apply the retry interval
		select {
		case <-ctx.Done():
			logger.Error(ctx.Err(), "Context cancelled during retry wait")
			return updated
		case <-time.After(r.retryInterval):
			// Continue with retry
		}

		// Pass the updated status to the next run so that a step can store info
		// in status object for subsequent runs
		updated = r.step.Run(ctx, resource, updated)
	}

	return updated
}

// IsSuccessful delegates to the wrapped step
func (r *RetryStep[T, S]) IsSuccessful(resource T, status S) bool {
	return r.step.IsSuccessful(resource, status)
}

// ShouldSkip delegates to the wrapped step
func (r *RetryStep[T, S]) ShouldSkip(resource T, status S) bool {
	return r.step.ShouldSkip(resource, status)
}
