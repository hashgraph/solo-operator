// SPDX-License-Identifier: Apache-2.0

package sequencer

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCache_AddItem(t *testing.T) {
	cache := NewCache[*corev1.ConfigMap]()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "default",
			UID:               types.UID("test-uid"),
			CreationTimestamp: metav1.Now(),
		},
	}

	// First add should succeed
	added := cache.AddItem(cm)
	if !added {
		t.Error("Expected item to be added")
	}

	// Second add should fail (duplicate)
	added = cache.AddItem(cm)
	if added {
		t.Error("Expected item not to be added (duplicate)")
	}

	// Verify size
	if cache.Size("default") != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size("default"))
	}
}

func TestCache_RemoveItem(t *testing.T) {
	cache := NewCache[*corev1.ConfigMap]()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "default",
			UID:               types.UID("test-uid"),
			CreationTimestamp: metav1.Now(),
		},
	}

	cache.AddItem(cm)

	// Remove should succeed
	removed := cache.RemoveItem(cm)
	if removed == nil {
		t.Error("Expected item to be removed")
	}

	// Verify size
	if cache.Size("default") != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size("default"))
	}
}

func TestCache_IsFirstInQueue(t *testing.T) {
	cache := NewCache[*corev1.ConfigMap]()
	now := time.Now()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "first",
			Namespace:         "default",
			UID:               types.UID("first-uid"),
			CreationTimestamp: metav1.NewTime(now),
		},
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "second",
			Namespace:         "default",
			UID:               types.UID("second-uid"),
			CreationTimestamp: metav1.NewTime(now.Add(1 * time.Second)),
		},
	}

	cache.AddItem(cm1)
	cache.AddItem(cm2)

	// First item should be first in queue
	if !cache.IsFirstInQueue(cm1) {
		t.Error("Expected cm1 to be first in queue")
	}

	// Second item should not be first
	if cache.IsFirstInQueue(cm2) {
		t.Error("Expected cm2 not to be first in queue")
	}
}

func TestCache_Ordering(t *testing.T) {
	cache := NewCache[*corev1.ConfigMap]()
	now := time.Now()

	// Add items in reverse order
	cm3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "third",
			Namespace:         "default",
			UID:               types.UID("third-uid"),
			CreationTimestamp: metav1.NewTime(now.Add(2 * time.Second)),
		},
	}
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "first",
			Namespace:         "default",
			UID:               types.UID("first-uid"),
			CreationTimestamp: metav1.NewTime(now),
		},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "second",
			Namespace:         "default",
			UID:               types.UID("second-uid"),
			CreationTimestamp: metav1.NewTime(now.Add(1 * time.Second)),
		},
	}

	cache.AddItem(cm3)
	cache.AddItem(cm1)
	cache.AddItem(cm2)

	items := cache.Items("default")
	if len(items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(items))
	}

	// Verify ordering
	if items[0].Name != "first" {
		t.Errorf("Expected first item to be 'first', got %q", items[0].Name)
	}
	if items[1].Name != "second" {
		t.Errorf("Expected second item to be 'second', got %q", items[1].Name)
	}
	if items[2].Name != "third" {
		t.Errorf("Expected third item to be 'third', got %q", items[2].Name)
	}
}

func TestSequencer_ComputeReconcileDelay(t *testing.T) {
	seq := NewSequencer(func() []*corev1.ConfigMap { return nil })
	now := time.Now()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "default",
			UID:               types.UID("test-uid"),
			CreationTimestamp: metav1.NewTime(now),
		},
	}

	request := NewRequest(cm)
	delay := seq.ComputeReconcileDelay(request)

	// When not in cache (index <= 0), should return the recommended delay
	if delay != DefaultReconciliationDelay {
		t.Errorf("Expected %v, got %v", DefaultReconciliationDelay, delay)
	}
}

func TestSequencer_CheckNext(t *testing.T) {
	now := time.Now()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "first",
			Namespace:         "default",
			UID:               types.UID("first-uid"),
			CreationTimestamp: metav1.NewTime(now),
		},
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "second",
			Namespace:         "default",
			UID:               types.UID("second-uid"),
			CreationTimestamp: metav1.NewTime(now.Add(1 * time.Second)),
		},
	}

	seq := NewSequencer(func() []*corev1.ConfigMap {
		return []*corev1.ConfigMap{cm1, cm2}
	})

	// First item should be first in queue
	resp1 := seq.CheckNext(NewRequest(cm1))
	if !resp1.FirstInQueue {
		t.Error("Expected cm1 to be first in queue")
	}

	// Second item should not be first
	resp2 := seq.CheckNext(NewRequest(cm2))
	if resp2.FirstInQueue {
		t.Error("Expected cm2 not to be first in queue")
	}
}

func TestSequencer_MarkAsCompleted(t *testing.T) {
	now := time.Now()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "first",
			Namespace:         "default",
			UID:               types.UID("first-uid"),
			CreationTimestamp: metav1.NewTime(now),
		},
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "second",
			Namespace:         "default",
			UID:               types.UID("second-uid"),
			CreationTimestamp: metav1.NewTime(now.Add(1 * time.Second)),
		},
	}

	seq := NewSequencer(func() []*corev1.ConfigMap {
		return []*corev1.ConfigMap{cm1, cm2}
	})

	// Check first to add to cache
	seq.CheckNext(NewRequest(cm1))

	// Mark first as completed
	completed, err := seq.MarkAsCompleted(NewRequest(cm1))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !completed {
		t.Error("Expected item to be marked as completed")
	}

	// Now second should be first
	resp := seq.CheckNext(NewRequest(cm2))
	if !resp.FirstInQueue {
		t.Error("Expected cm2 to be first in queue after cm1 completed")
	}
}

func TestSequenced_CompareTo(t *testing.T) {
	now := time.Now()

	s1 := Sequenced{
		UID:               "1",
		Name:              "alpha",
		CreationTimestamp: now,
	}

	s2 := Sequenced{
		UID:               "2",
		Name:              "beta",
		CreationTimestamp: now.Add(1 * time.Second),
	}

	s3 := Sequenced{
		UID:               "3",
		Name:              "gamma",
		CreationTimestamp: now, // Same timestamp as s1
	}

	// s1 should be before s2 (earlier timestamp)
	if s1.CompareTo(s2) >= 0 {
		t.Error("Expected s1 < s2")
	}

	// s2 should be after s1
	if s2.CompareTo(s1) <= 0 {
		t.Error("Expected s2 > s1")
	}

	// s1 should be before s3 (same timestamp, but alpha < gamma)
	if s1.CompareTo(s3) >= 0 {
		t.Error("Expected s1 < s3 (by name)")
	}
}
