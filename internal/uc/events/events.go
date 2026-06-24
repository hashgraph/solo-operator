// SPDX-License-Identifier: Apache-2.0

// Package events is the UC sidecar's audit substrate. It writes per-operation
// JSONL event logs and applies the same startup retention policy the
// solo-weaver consensus daemon uses for its upgrade event logs.
//
// Each upgrade operation gets its own append-only file named "<operationID>.jsonl"
// (e.g. "upgrade-20260415T143000Z.jsonl"). Every marker handler appends audit
// lines to the operation's file as the upgrade progresses, so the file is a
// single forensic trail for one operation. The underlying writer
// (daemonkit/eventlog) fsyncs each line so entries survive a pod crash.
//
// Base fields on every line: ts, level, reason, msg, operationId, nodeId.
package events

import (
	"time"

	"github.com/automa-saga/daemonkit/eventlog"
	"github.com/automa-saga/daemonkit/filepruner"
	"github.com/automa-saga/logx"

	"github.com/hashgraph/solo-operator/internal/sanity"
)

// Level re-exports the daemonkit severity type so callers depend only on this
// package. Only INFO and ERROR exist — every event is either a milestone that
// happened or one that failed terminally; operational noise belongs in the logs.
type Level = eventlog.Level

const (
	LevelInfo  = eventlog.LevelInfo
	LevelError = eventlog.LevelError
)

const (
	// fileGlob matches the UC upgrade event logs in the events directory. UC only
	// writes upgrade operations, so the daemon's "consensus-" prefix is dropped;
	// the operation ID already carries an "upgrade-" prefix.
	fileGlob = "upgrade-*.jsonl"

	// retainMaxAge is the age threshold for the FilenameTimestampStrategy. It
	// mirrors the daemon's 365-day window; the hard cap is the dominant control.
	retainMaxAge = 365 * 24 * time.Hour

	// RetainCount is the hard cap on retained upgrade event logs applied at UC
	// startup: the oldest files (by embedded timestamp) beyond this many are
	// pruned. Kept at 50 to match the consensus daemon's retention policy.
	RetainCount = 50
)

// Writer appends audit events for a single upgrade operation. The operation ID
// and node ID are fixed for the lifetime of the writer; each call stamps the
// current time and the fixed base fields, so callers supply only level, reason,
// and message. Safe for concurrent use.
type Writer struct {
	logger      *eventlog.EventLogger
	operationID string
	nodeID      string
}

// Open returns a Writer that appends to "<operationID>.jsonl" in dir, creating
// the file if it does not exist. operationID is validated for path/shell safety
// (sanity.ValidateOperationID); it is NOT required to follow the timestamp
// convention, but one that omits a parseable timestamp will never be pruned by
// the retention policy, so Open logs a warning in that case. nodeID is the
// consensus node identity recorded on every event. The caller must Close the
// Writer.
func Open(dir, operationID, nodeID string) (*Writer, error) {
	if err := sanity.ValidateOperationID(operationID); err != nil {
		return nil, err
	}
	if nodeID == "" {
		return nil, eventlog.ErrInvalidEvent.New("nodeID is required")
	}
	if !hasTimestamp(operationID) {
		logx.As().Warn().
			Str("reason", "OperationIDMissingTimestamp").
			Str("operationId", operationID).
			Msg("operationId has no parseable timestamp; its event log will never be pruned by retention")
	}
	logger, err := eventlog.NewAppend(dir, operationID+".jsonl")
	if err != nil {
		return nil, err
	}
	return &Writer{logger: logger, operationID: operationID, nodeID: nodeID}, nil
}

// Path returns the absolute path of the operation's JSONL file.
func (w *Writer) Path() string { return w.logger.Path() }

// Info appends an INFO milestone line.
func (w *Writer) Info(reason, msg string) error { return w.log(LevelInfo, reason, msg) }

// Error appends an ERROR milestone line for a terminal failure.
func (w *Writer) Error(reason, msg string) error { return w.log(LevelError, reason, msg) }

func (w *Writer) log(level Level, reason, msg string) error {
	return w.logger.Log(eventlog.Event{
		Ts:          time.Now().UTC(),
		Level:       level,
		Reason:      reason,
		Msg:         msg,
		OperationID: w.operationID,
		NodeID:      w.nodeID,
	})
}

// Close flushes and closes the underlying file.
func (w *Writer) Close() error { return w.logger.Close() }

// Prune applies the startup retention policy to upgrade event logs in dir: files
// older than the max-age window are removed, then the oldest remaining files
// beyond RetainCount are removed, leaving the last RetainCount by embedded
// timestamp. Files whose timestamp cannot be parsed are never deleted. This is
// the same FilenameTimestampStrategy the consensus daemon applies. Intended to
// run once at UC startup; a failure should be logged and tolerated, not fatal.
func Prune(dir string) error {
	p := filepruner.New(filepruner.FilenameTimestampStrategy{
		Layout: sanity.TimestampLayout,
		MaxAge: retainMaxAge,
	})
	return p.Prune(dir, fileGlob, RetainCount)
}

// hasTimestamp reports whether s contains a substring parseable by
// sanity.TimestampLayout, scanning the same way filepruner's
// FilenameTimestampStrategy does so the warning and pruning agree on what counts
// as a timestamp.
func hasTimestamp(s string) bool {
	n := len(sanity.TimestampLayout)
	for i := 0; i+n <= len(s); i++ {
		if _, err := time.Parse(sanity.TimestampLayout, s[i:i+n]); err == nil {
			return true
		}
	}
	return false
}
