// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"sync"

	"github.com/automa-saga/logx"

	"github.com/hashgraph/solo-operator/internal/uc/events"
)

// Placeholder values used in audit lines when an expected value is absent, so
// records stay parseable without angle-bracket sentinels. Lowercase to match
// real step/marker/phase values.
const (
	// unknownLabel stands in for a missing marker or step label.
	unknownLabel = "unknown"
	// nonePhase stands in for the "no previous phase" case on the first transition.
	nonePhase = "none"

	// maxWriteFailures is how many *consecutive* write failures an operation's log
	// tolerates before it is disabled. A single successful write resets the count,
	// so transient IO blips are retried rather than permanently silencing the log;
	// only a sustained failure (full disk, revoked permissions) disables it — and
	// then it is logged once, not on every watch tick.
	maxWriteFailures = 3
)

// eventSink writes the UC's per-operation JSONL audit trail (HIP-1496
// "Channel 1"). It lazily opens one events.Writer per operationId under a single
// directory and caches it, so every audit line for an operation lands in that
// operation's "<operationId>.jsonl" file.
//
// JSONL emission is best-effort observability, never a correctness dependency:
// a bad path, full disk, or unsafe operationId disables the log for that
// operation (logged once) but must never block an upgrade. All methods are
// nil-safe, so a Factory or monitor with no sink wired (tests, or an events
// directory that could not be created) simply no-ops.
type eventSink struct {
	dir    string
	nodeID string

	mu       sync.Mutex
	writers  map[string]*events.Writer
	failed   map[string]struct{} // operationIds disabled (Open failed, or too many write failures)
	writeErr map[string]int      // consecutive write-failure count per operationId; reset on success
}

// newEventSink returns a sink writing under dir with the given nodeID stamped on
// every line. An empty dir yields a sink that no-ops (used when events are off).
func newEventSink(dir, nodeID string) *eventSink {
	return &eventSink{
		dir:      dir,
		nodeID:   nodeID,
		writers:  make(map[string]*events.Writer),
		failed:   make(map[string]struct{}),
		writeErr: make(map[string]int),
	}
}

// info appends an INFO audit line to operationId's log.
func (s *eventSink) info(operationID, reason, msg string) {
	s.write(false, operationID, reason, msg)
}

// error appends an ERROR audit line to operationId's log.
func (s *eventSink) error(operationID, reason, msg string) {
	s.write(true, operationID, reason, msg)
}

func (s *eventSink) write(isErr bool, operationID, reason, msg string) {
	w := s.writer(operationID)
	if w == nil {
		return
	}
	var err error
	if isErr {
		err = w.Error(reason, msg)
	} else {
		err = w.Info(reason, msg)
	}
	if err != nil {
		s.onWriteError(operationID, err)
		return
	}
	// A successful write clears any prior consecutive-failure streak.
	s.mu.Lock()
	delete(s.writeErr, operationID)
	s.mu.Unlock()
}

// onWriteError records a failed write. It retries (keeps the writer open) until
// maxWriteFailures consecutive failures, then disables the operation's log so a
// sustained IO error is logged once rather than on every watch tick.
func (s *eventSink) onWriteError(operationID string, werr error) {
	s.mu.Lock()
	s.writeErr[operationID]++
	n := s.writeErr[operationID]
	s.mu.Unlock()

	if n < maxWriteFailures {
		logx.As().Warn().Err(werr).Str("reason", ReasonEventLogWriteFailed.String()).
			Str("operation_id", operationID).Int("failures", n).
			Msg("could not append UC event-log line; will retry")
		return
	}
	logx.As().Warn().Err(werr).Str("reason", ReasonEventLogWriteFailed.String()).
		Str("operation_id", operationID).Int("failures", n).
		Msgf("disabling UC event log for this operation after %d consecutive write failures", maxWriteFailures)
	s.disable(operationID)
}

// disable closes and drops the cached Writer for operationId and marks it failed
// so writer() returns nil for it thereafter. Used to stop retrying (and
// re-logging) a persistently failing per-operation log.
func (s *eventSink) disable(operationID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.writers[operationID]; ok {
		_ = w.Close()
		delete(s.writers, operationID)
	}
	delete(s.writeErr, operationID)
	s.failed[operationID] = struct{}{}
}

// writer returns the cached Writer for operationId, opening it on first use.
// Returns nil (a no-op) when the sink is disabled, the operationId is empty, or
// a prior Open failed — so callers never need a nil check of their own.
func (s *eventSink) writer(operationID string) *events.Writer {
	if s == nil || s.dir == "" || operationID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.writers[operationID]; ok {
		return w
	}
	if _, bad := s.failed[operationID]; bad {
		return nil
	}
	w, err := events.Open(s.dir, operationID, s.nodeID)
	if err != nil {
		s.failed[operationID] = struct{}{}
		logx.As().Warn().Err(err).Str("reason", ReasonEventLogOpenFailed.String()).
			Str("operation_id", operationID).
			Msg("could not open UC event log; JSONL audit disabled for this operation")
		return nil
	}
	s.writers[operationID] = w
	return w
}

// closeOp flushes and closes the Writer for a single operation and drops it from
// the cache, releasing the file handle once the operation is terminal. A later
// write for the same operationId simply reopens in append mode (harmless), so
// this only bounds the number of concurrently-open writers to the set of
// in-flight operations rather than every operation seen for the UC's lifetime.
// Nil-safe.
func (s *eventSink) closeOp(operationID string) {
	if s == nil || operationID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.writers[operationID]; ok {
		if err := w.Close(); err != nil {
			logx.As().Warn().Err(err).Str("operation_id", operationID).Msg("error closing UC event log")
		}
		delete(s.writers, operationID)
	}
}

// Close flushes and closes every open per-operation Writer. Nil-safe; intended to
// run once as the UC shuts down.
func (s *eventSink) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, w := range s.writers {
		if err := w.Close(); err != nil {
			logx.As().Warn().Err(err).Str("operation_id", id).Msg("error closing UC event log")
		}
		delete(s.writers, id)
	}
}
