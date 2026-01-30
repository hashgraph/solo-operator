// SPDX-License-Identifier: Apache-2.0

package sequencer

import (
	"errors"
	"slices"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrAlreadyExecuting is returned when a resource is already being executed in the namespace
var ErrAlreadyExecuting = errors.New("resource is already executing")

// Cache is a thread-safe cache for managing Kubernetes resources that need to be processed in sequence.
// It maintains a list of items per namespace, allowing for efficient addition, removal, and retrieval of items.
type Cache[T client.Object] struct {
	mu sync.RWMutex

	// namespaceCache stores items per namespace
	namespaceCache map[string][]Sequenced

	// currentlyExecuting stores the currently executing item per namespace
	currentlyExecuting map[string]*Sequenced
}

// NewCache creates a new empty Cache
func NewCache[T client.Object]() *Cache[T] {
	return &Cache[T]{
		namespaceCache:     make(map[string][]Sequenced),
		currentlyExecuting: make(map[string]*Sequenced),
	}
}

// AddItem adds an item to the cache if it does not already exist.
// Items are ordered by creation time and then by name.
// Returns true if the item was added, false if it already exists.
func (c *Cache[T]) AddItem(resource T) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	namespace := resource.GetNamespace()

	// Check if item already exists
	if c.findItemLocked(resource) != nil {
		return false
	}

	// Add the item
	items := c.namespaceCache[namespace]
	items = append(items, FromResource(resource))

	// Sort by creation timestamp, then by name
	slices.SortFunc(items, func(a, b Sequenced) int {
		return a.CompareTo(b)
	})

	c.namespaceCache[namespace] = items
	return true
}

// RemoveItem removes an item from the cache.
// Returns the removed item, or nil if not found.
func (c *Cache[T]) RemoveItem(resource T) *Sequenced {
	c.mu.Lock()
	defer c.mu.Unlock()

	namespace := resource.GetNamespace()
	uid := string(resource.GetUID())

	item := c.findItemLocked(resource)
	if item == nil {
		return nil
	}

	// Remove from cache
	items := c.namespaceCache[namespace]
	for i, it := range items {
		if it.UID == uid {
			c.namespaceCache[namespace] = slices.Delete(items, i, i+1)
			break
		}
	}

	// Clean up empty namespace
	if len(c.namespaceCache[namespace]) == 0 {
		delete(c.namespaceCache, namespace)
	}

	// Clear currently executing if it matches
	if c.currentlyExecuting[namespace] != nil && c.currentlyExecuting[namespace].UID == uid {
		delete(c.currentlyExecuting, namespace)
	}

	return item
}

// FindFirst returns the first item in the queue for a given namespace.
// Returns nil if no items are found.
func (c *Cache[T]) FindFirst(namespace string) *Sequenced {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := c.namespaceCache[namespace]
	if len(items) == 0 {
		return nil
	}

	result := items[0].Clone()
	return &result
}

// FindLast returns the last item in the queue for a given namespace.
// Returns nil if no items are found.
func (c *Cache[T]) FindLast(namespace string) *Sequenced {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := c.namespaceCache[namespace]
	if len(items) == 0 {
		return nil
	}

	result := items[len(items)-1].Clone()
	return &result
}

// FindItem finds a specific item in the cache.
// Returns nil if not found.
func (c *Cache[T]) FindItem(resource T) *Sequenced {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.findItemLocked(resource)
}

// findItemLocked finds an item without acquiring the lock (caller must hold lock)
func (c *Cache[T]) findItemLocked(resource T) *Sequenced {
	namespace := resource.GetNamespace()
	uid := string(resource.GetUID())

	for _, item := range c.namespaceCache[namespace] {
		if item.UID == uid {
			result := item.Clone()
			return &result
		}
	}
	return nil
}

// IsFirstInQueue checks if the given resource is the first item in its queue.
func (c *Cache[T]) IsFirstInQueue(resource T) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	namespace := resource.GetNamespace()
	uid := string(resource.GetUID())

	// Check if another resource is currently executing
	if c.currentlyExecuting[namespace] != nil && c.currentlyExecuting[namespace].UID != uid {
		return false
	}

	first := c.namespaceCache[namespace]
	if len(first) == 0 {
		return false
	}

	return first[0].UID == uid
}

// IndexOf returns the index of the resource in its namespace's cache.
// Returns -1 if not found.
func (c *Cache[T]) IndexOf(resource T) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	namespace := resource.GetNamespace()
	uid := string(resource.GetUID())

	for i, item := range c.namespaceCache[namespace] {
		if item.UID == uid {
			return i
		}
	}
	return -1
}

// Contains checks if the cache contains a specific resource.
func (c *Cache[T]) Contains(resource T) bool {
	return c.FindItem(resource) != nil
}

// Items returns all items in the cache for a given namespace in order.
func (c *Cache[T]) Items(namespace string) []Sequenced {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := c.namespaceCache[namespace]
	if items == nil {
		return []Sequenced{}
	}

	// Return a copy
	result := make([]Sequenced, len(items))
	for i, item := range items {
		result[i] = item.Clone()
	}
	return result
}

// Size returns the number of items in the cache for a given namespace.
func (c *Cache[T]) Size(namespace string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.namespaceCache[namespace])
}

// Prune removes cached items that are no longer active.
func (c *Cache[T]) Prune(activeItems []T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build a set of active UIDs
	activeUIDs := make(map[string]bool)
	for _, item := range activeItems {
		activeUIDs[string(item.GetUID())] = true
	}

	// Remove inactive items from each namespace
	for namespace, items := range c.namespaceCache {
		filtered := items[:0]
		for _, item := range items {
			if activeUIDs[item.UID] {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) == 0 {
			delete(c.namespaceCache, namespace)
		} else {
			c.namespaceCache[namespace] = filtered
		}
	}

	// Remove inactive currently executing items
	for namespace, item := range c.currentlyExecuting {
		if item != nil && !activeUIDs[item.UID] {
			delete(c.currentlyExecuting, namespace)
		}
	}
}

// MarkAsExecuting marks a resource as being executed.
// Returns an error if another resource is already executing in the same namespace.
func (c *Cache[T]) MarkAsExecuting(resource T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	namespace := resource.GetNamespace()
	uid := string(resource.GetUID())

	// Check if same resource is already executing
	if c.currentlyExecuting[namespace] != nil {
		if c.currentlyExecuting[namespace].UID == uid {
			return nil
		}
		return ErrAlreadyExecuting
	}

	c.currentlyExecuting[namespace] = &Sequenced{
		UID:               uid,
		Name:              resource.GetName(),
		Namespace:         namespace,
		CreationTimestamp: resource.GetCreationTimestamp().Time,
	}
	return nil
}

// GetCurrentlyExecuting returns the currently executing resource for a namespace.
// Returns nil if none is executing.
func (c *Cache[T]) GetCurrentlyExecuting(namespace string) *Sequenced {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.currentlyExecuting[namespace] == nil {
		return nil
	}

	result := c.currentlyExecuting[namespace].Clone()
	return &result
}

// ItemSequence returns the sequence of items for a given namespace in order.
func (c *Cache[T]) ItemSequence(namespace string) []Sequenced {
	return c.Items(namespace)
}
