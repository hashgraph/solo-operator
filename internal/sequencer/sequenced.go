// SPDX-License-Identifier: Apache-2.0

package sequencer

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Sequenced represents an item in the sequencer cache with ordering information
type Sequenced struct {
	// UID is the unique identifier for the resource
	UID string

	// Name is the resource name
	Name string

	// Namespace is the resource namespace
	Namespace string

	// CreationTimestamp is when the resource was created
	CreationTimestamp time.Time
}

// FromResource creates a Sequenced item from a Kubernetes resource
func FromResource(resource client.Object) Sequenced {
	return Sequenced{
		UID:               string(resource.GetUID()),
		Name:              resource.GetName(),
		Namespace:         resource.GetNamespace(),
		CreationTimestamp: resource.GetCreationTimestamp().Time,
	}
}

// Clone creates a copy of the Sequenced item
func (s Sequenced) Clone() Sequenced {
	return Sequenced{
		UID:               s.UID,
		Name:              s.Name,
		Namespace:         s.Namespace,
		CreationTimestamp: s.CreationTimestamp,
	}
}

// CompareTo compares this Sequenced item to another for ordering.
// Items are ordered by creation timestamp first, then by name.
// Returns negative if this < other, positive if this > other, 0 if equal.
func (s Sequenced) CompareTo(other Sequenced) int {
	if s.CreationTimestamp.Before(other.CreationTimestamp) {
		return -1
	}
	if s.CreationTimestamp.After(other.CreationTimestamp) {
		return 1
	}

	// Same timestamp, compare by name
	if s.Name < other.Name {
		return -1
	}
	if s.Name > other.Name {
		return 1
	}

	return 0
}
