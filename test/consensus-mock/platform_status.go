// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// platformStatusWriter appends PLATFORM_STATUS lines the UC's LogMonitor tails.
type platformStatusWriter struct {
	path string

	mu  sync.Mutex
	cur string
}

func newPlatformStatusWriter(path string) (*platformStatusWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create consensus log dir: %w", err)
	}
	return &platformStatusWriter{path: path, cur: operatorv1alpha1.PlatformStatusNoValue}, nil
}

func (w *platformStatusWriter) transition(next string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("consensus-mock: platform status: open %s: %v", w.path, err)
		return
	}
	defer func() { _ = f.Close() }()

	line := fmt.Sprintf("%s PLATFORM_STATUS <main> StatusStateMachine - transition {\"oldStatus\":%q,\"newStatus\":%q}\n",
		time.Now().UTC().Format(time.RFC3339Nano), w.cur, next)
	if _, err := f.WriteString(line); err != nil {
		log.Printf("consensus-mock: platform status: write %s: %v", w.path, err)
		return
	}

	log.Printf("consensus-mock: platform status %s -> %s", w.cur, next)
	w.cur = next
}

func (w *platformStatusWriter) transitionAfter(first, second string, delay time.Duration) {
	w.transition(first)
	time.AfterFunc(delay, func() {
		w.transition(second)
	})
}

func (w *platformStatusWriter) startupSequence(activeDelay time.Duration) {
	w.transitionAfter(operatorv1alpha1.PlatformStatusStartingUp, operatorv1alpha1.PlatformStatusActive, activeDelay)
}

func (w *platformStatusWriter) freezeSequence(completeDelay time.Duration) {
	w.transitionAfter(operatorv1alpha1.PlatformStatusFreezing, operatorv1alpha1.PlatformStatusFreezeComplete, completeDelay)
}
