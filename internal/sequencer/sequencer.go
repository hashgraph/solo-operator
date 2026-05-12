// SPDX-License-Identifier: Apache-2.0

package sequencer

import (
	"errors"
	"math"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// MaxReconciliationDelay is the maximum allowed reconciliation delay
	MaxReconciliationDelay = time.Hour

	// DefaultReconciliationDelay is the default reconciliation delay
	DefaultReconciliationDelay = 5 * time.Second

	// MaxEstimatedProcessingTime is the maximum allowed estimated processing time
	MaxEstimatedProcessingTime = time.Hour

	// DefaultEstimatedProcessingTime is the default estimated processing time
	DefaultEstimatedProcessingTime = 30 * time.Second

	// MaxRefreshInterval is the maximum interval between cache refreshes
	MaxRefreshInterval = time.Minute
)

// ErrNotFirstInQueue is returned when trying to complete a resource that is not first in queue
var ErrNotFirstInQueue = errors.New("resource is not the first item in the queue; it should not have been processed")

// ResourceSupplier is a function that returns the list of active resources
type ResourceSupplier[T client.Object] func() []T

// Request represents a request to the sequencer for processing a resource
type Request[T client.Object] struct {
	Resource       T
	ReconcileDelay time.Duration
	ProcessingTime time.Duration
}

// NewRequest creates a new Request with default delays
func NewRequest[T client.Object](resource T) Request[T] {
	return Request[T]{
		Resource:       resource,
		ReconcileDelay: DefaultReconciliationDelay,
		ProcessingTime: DefaultEstimatedProcessingTime,
	}
}

// NewRequestWithDelay creates a new Request with a custom reconcile delay
func NewRequestWithDelay[T client.Object](resource T, reconcileDelay time.Duration) Request[T] {
	return Request[T]{
		Resource:       resource,
		ReconcileDelay: reconcileDelay,
		ProcessingTime: DefaultEstimatedProcessingTime,
	}
}

// NewRequestFull creates a new Request with all custom parameters
func NewRequestFull[T client.Object](resource T, reconcileDelay, processingTime time.Duration) Request[T] {
	return Request[T]{
		Resource:       resource,
		ReconcileDelay: reconcileDelay,
		ProcessingTime: processingTime,
	}
}

// Response represents a response from the sequencer
type Response struct {
	// ReconcileDelay is the duration to delay reconciliation
	ReconcileDelay time.Duration

	// FirstInQueue indicates if this resource is the first in the queue
	FirstInQueue bool

	// ShouldReschedule indicates if the resource should be rescheduled
	ShouldReschedule bool
}

// Sequencer is responsible for ordering the processing of custom resources
// and computing the reconcile delay.
type Sequencer[T client.Object] struct {
	mu sync.Mutex

	cache           *Cache[T]
	activeResources ResourceSupplier[T]
	lastRefreshed   time.Time
}

// NewSequencer creates a new Sequencer with the given resource supplier
func NewSequencer[T client.Object](activeResources ResourceSupplier[T]) *Sequencer[T] {
	return &Sequencer[T]{
		cache:           NewCache[T](),
		activeResources: activeResources,
	}
}

// Refresh refreshes the cache using data from the Kubernetes API server
func (s *Sequencer[T]) Refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeItems := s.activeResources()
	if len(activeItems) == 0 {
		return
	}

	// Prune old items that are not active anymore
	s.cache.Prune(activeItems)

	// Add active items to the cache
	for _, item := range activeItems {
		s.cache.AddItem(item)
	}

	s.lastRefreshed = time.Now()
}

// FindOrRefresh finds the resource in the cache or refreshes if not found
func (s *Sequencer[T]) FindOrRefresh(resource T) *Sequenced {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Force refresh if cache is stale
	refreshed := false
	if s.lastRefreshed.IsZero() || time.Since(s.lastRefreshed) > MaxRefreshInterval {
		s.refreshLocked()
		refreshed = true
	}

	item := s.cache.FindItem(resource)
	if item != nil || refreshed {
		return item
	}

	// Refresh and try again
	s.refreshLocked()
	return s.cache.FindItem(resource)
}

// refreshLocked refreshes the cache (must hold lock)
func (s *Sequencer[T]) refreshLocked() {
	activeItems := s.activeResources()
	if len(activeItems) == 0 {
		return
	}

	s.cache.Prune(activeItems)

	for _, item := range activeItems {
		s.cache.AddItem(item)
	}

	s.lastRefreshed = time.Now()
}

// ComputeReconcileDelay computes the reconcile delay based on queue position
func (s *Sequencer[T]) ComputeReconcileDelay(request Request[T]) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	resource := request.Resource

	// Normalize processing time
	processingTime := request.ProcessingTime
	if processingTime <= 0 {
		processingTime = DefaultEstimatedProcessingTime
	} else if processingTime > MaxEstimatedProcessingTime {
		processingTime = MaxEstimatedProcessingTime
	}

	// Normalize reconcile delay
	reconcileDelay := request.ReconcileDelay
	if reconcileDelay < 0 {
		reconcileDelay = DefaultReconciliationDelay
	} else if reconcileDelay > MaxReconciliationDelay {
		reconcileDelay = MaxReconciliationDelay
	}

	index := s.cache.IndexOf(resource)
	if index <= 0 {
		return reconcileDelay
	}

	// Take 30% of the total execution time of all items before this item
	totalExecutionTime := processingTime * time.Duration(index)
	computedDelay := time.Duration(math.Floor(float64(totalExecutionTime.Milliseconds())*0.3)) * time.Millisecond

	// Return the smaller of computed and recommended delay
	if computedDelay < reconcileDelay {
		return computedDelay
	}

	return reconcileDelay
}

// CheckNext checks whether the resource is first in queue to be processed.
// Reconciler must call this before processing the resource.
func (s *Sequencer[T]) CheckNext(request Request[T]) Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := log.Log.WithName("sequencer")

	resource := request.Resource

	// Refresh on cache miss so brand-new resources don't have to wait out the
	// MaxRefreshInterval before being picked up.
	s.refreshIfNeededLocked()
	sequencedItem := s.cache.FindItem(resource)
	if sequencedItem == nil {
		s.refreshLocked()
		sequencedItem = s.cache.FindItem(resource)
	}
	if sequencedItem == nil {
		logger.V(1).Info("Resource not found in cache",
			"namespace", resource.GetNamespace(),
			"name", resource.GetName(),
		)
		return Response{
			ReconcileDelay:   0,
			FirstInQueue:     false,
			ShouldReschedule: false,
		}
	}

	reconcileDelay := s.computeReconcileDelayLocked(request)
	isFirst := s.cache.IsFirstInQueue(resource)

	if isFirst {
		if err := s.cache.MarkAsExecuting(resource); err != nil {
			logger.Error(err, "Failed to mark as executing",
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
			)
			isFirst = false
		}
	}

	return Response{
		ReconcileDelay:   reconcileDelay,
		FirstInQueue:     isFirst,
		ShouldReschedule: true,
	}
}

// refreshIfNeededLocked refreshes if cache is stale (must hold lock)
func (s *Sequencer[T]) refreshIfNeededLocked() {
	if s.lastRefreshed.IsZero() || time.Since(s.lastRefreshed) > MaxRefreshInterval {
		s.refreshLocked()
	}
}

// computeReconcileDelayLocked computes delay (must hold lock)
func (s *Sequencer[T]) computeReconcileDelayLocked(request Request[T]) time.Duration {
	resource := request.Resource

	processingTime := request.ProcessingTime
	if processingTime <= 0 {
		processingTime = DefaultEstimatedProcessingTime
	} else if processingTime > MaxEstimatedProcessingTime {
		processingTime = MaxEstimatedProcessingTime
	}

	reconcileDelay := request.ReconcileDelay
	if reconcileDelay < 0 {
		reconcileDelay = DefaultReconciliationDelay
	} else if reconcileDelay > MaxReconciliationDelay {
		reconcileDelay = MaxReconciliationDelay
	}

	index := s.cache.IndexOf(resource)
	if index <= 0 {
		return reconcileDelay
	}

	totalExecutionTime := processingTime * time.Duration(index)
	computedDelay := time.Duration(math.Floor(float64(totalExecutionTime.Milliseconds())*0.3)) * time.Millisecond

	if computedDelay < reconcileDelay {
		return computedDelay
	}

	return reconcileDelay
}

// MarkAsCompleted marks the resource as processed and removes it from the cache.
// Reconciler must call this after successfully reconciling the resource.
// Returns true if the resource was found and removed.
func (s *Sequencer[T]) MarkAsCompleted(request Request[T]) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := log.Log.WithName("sequencer")
	resource := request.Resource

	first := s.cache.FindFirst(resource.GetNamespace())
	if first == nil {
		logger.Info("Cannot remove resource: cache is empty",
			"namespace", resource.GetNamespace(),
			"name", resource.GetName(),
		)
		return false, nil
	}

	if first.UID != string(resource.GetUID()) {
		return false, ErrNotFirstInQueue
	}

	item := s.cache.RemoveItem(resource)
	return item != nil, nil
}

// MarkAsDeleted marks the resource as deleted and removes it from the cache.
// Reconciler must call this if it detects a resource is deleted.
func (s *Sequencer[T]) MarkAsDeleted(request Request[T]) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.cache.RemoveItem(request.Resource)
	return item != nil
}

// ItemSequence returns the sequence of items in the cache for a given namespace
func (s *Sequencer[T]) ItemSequence(namespace string) []Sequenced {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.ItemSequence(namespace)
}
