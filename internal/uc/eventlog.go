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
)

// eventSink writes the UC's per-operation JSONL audit trail (HIP-1496
// "Channel 1"). It lazily opens one events.Writer per operationId under a single
// directory and caches it, so every audit line for an operation lands in that
// operation's "<operationId>.jsonl" file.
//
// JSONL emission is best-effort observability, never a correctness dependency:
// a bad path or unsafe operationId disables the log for that operation on Open
// (logged once) but must never block an upgrade. Individual write failures are
// logged and dropped — writes only happen on a milestone/phase transition (each
// deduped, with the tracking state advanced before the write), so a failed write
// is never retried on later ticks and cannot spam. All methods are nil-safe, so
// a Factory or monitor with no sink wired (tests, or an events directory that
// could not be created) simply no-ops.
type eventSink struct {
	dir    string
	nodeID string

	mu      sync.Mutex
	writers map[string]*events.Writer
	failed  map[string]struct{} // operationIds whose Open failed — don't retry per line
}

// newEventSink returns a sink writing under dir with the given nodeID stamped on
// every line. An empty dir yields a sink that no-ops (used when events are off).
func newEventSink(dir, nodeID string) *eventSink {
	return &eventSink{
		dir:     dir,
		nodeID:  nodeID,
		writers: make(map[string]*events.Writer),
		failed:  make(map[string]struct{}),
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
		// Log and drop. Writes only occur on a milestone/phase transition (each
		// deduped, tracking state advanced before the write), so this is not retried
		// on later ticks — no per-tick spam, and no need to disable the log.
		logx.As().Warn().Err(err).Str("reason", ReasonEventLogWriteFailed.String()).
			Str("operation_id", operationID).Msg("Failed to append UC event-log line")
	}
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
	var openOpts []events.OpenOption
	if operationID == platformStatusOperationID {
		openOpts = append(openOpts, events.WithoutTimestampWarning())
	}
	w, err := events.Open(s.dir, operationID, s.nodeID, openOpts...)
	if err != nil {
		s.failed[operationID] = struct{}{}
		logx.As().Warn().Err(err).Str("reason", ReasonEventLogOpenFailed.String()).
			Str("operation_id", operationID).
			Msg("Failed to open UC event log; JSONL audit disabled for this operation")
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
			logx.As().Warn().Err(err).Str("operation_id", operationID).Msg("Failed to close UC event log")
		}
		delete(s.writers, operationID)
	}
	// Drop any disabled-marker too, so an operation's cached state is fully
	// released (terminal cleanup, and the stale-operation safety-net sweep).
	delete(s.failed, operationID)
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
			logx.As().Warn().Err(err).Str("operation_id", id).Msg("Failed to close UC event log")
		}
		delete(s.writers, id)
	}
}
