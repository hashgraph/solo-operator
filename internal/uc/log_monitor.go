// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/automa-saga/daemonkit"
	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	DefaultConsensusLogFile = "/opt/hgcapp/services-hedera/HapiApp2.0/output/swirlds.log"

	// DefaultLogPollInterval is the cadence at which the log monitor tails the
	// consensus log file for PLATFORM_STATUS transitions.
	DefaultLogPollInterval = 1 * time.Second

	logMonitorName    = "log-monitor"
	logPersistFile    = "log-monitor-status.json"
	platformStatusTag = "PLATFORM_STATUS"

	maxLogLineSize    = 1 << 20
	maxLogPersistSize = 4 << 10
)

// LogStatusSnapshot is an atomically-published view of the latest platform
// status derived from the consensus log file.
type LogStatusSnapshot struct {
	Code   int32
	Status string
}

// logPersistedState is the JSON format persisted to disk for restart survival.
type logPersistedState struct {
	Code   int32  `json:"code"`
	Status string `json:"status"`
}

// LogMonitor tails the consensus node's log file for PLATFORM_STATUS
// transitions and publishes a sticky status code. The code holds between
// transitions and persists to disk across restarts.
type LogMonitor struct {
	cfg Config

	snapshot        atomic.Pointer[LogStatusSnapshot]
	connectivityErr atomic.Pointer[daemonkit.StatusError]

	persistPath string
	offset      int64

	lineReader *bufio.Reader
	skipping   bool
}

// NewLogMonitor constructs a log monitor.
func NewLogMonitor(cfg Config) *LogMonitor {
	return &LogMonitor{
		cfg:         cfg,
		persistPath: filepath.Join(cfg.CRPersistPath, logPersistFile),
		lineReader:  bufio.NewReaderSize(nil, maxLogLineSize),
	}
}

func (m *LogMonitor) Name() string { return logMonitorName }

func (m *LogMonitor) ConnectivityError() *daemonkit.StatusError {
	return m.connectivityErr.Load()
}

// Snapshot returns the current log-derived status, or nil if no transition has
// been observed.
func (m *LogMonitor) Snapshot() *LogStatusSnapshot {
	return m.snapshot.Load()
}

func (m *LogMonitor) Run(ctx context.Context) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logx.As().Error().Str("reason", ReasonLogMonitorPanic.String()).
				Interface("panic", r).Msg("Recovering from panic in log monitor")
			retErr = errx.WithReason(errPanic.New("recovered panic in log monitor: %v", r), ReasonLogMonitorPanic)
		}
	}()

	logx.As().Info().
		Str("reason", ReasonLogMonitorStarted.String()).
		Str("file", m.cfg.ConsensusLogFile).
		Msg("Started log monitor")

	m.loadPersisted()
	m.seedFromTail()

	ticker := time.NewTicker(m.cfg.LogPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logx.As().Info().Str("reason", ReasonLogMonitorStopped.String()).Msg("Stopped log monitor")
			return nil
		case <-ticker.C:
			m.poll()
		}
	}
}

// loadPersisted restores the sticky status from the persist file.
func (m *LogMonitor) loadPersisted() {
	data, err := readFileNoFollow(m.persistPath, maxLogPersistSize)
	if err != nil {
		return
	}
	var state logPersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	if state.Status == "" {
		return
	}
	status := nameForCode(state.Code)
	m.snapshot.Store(&LogStatusSnapshot{Code: state.Code, Status: status})
	logx.As().Info().
		Str("reason", ReasonLogMonitorRestored.String()).
		Int32("code", state.Code).Str("status", status).
		Msg("Restored persisted platform status")
}

// seedFromTail scans the tail of the current log file for the most recent
// PLATFORM_STATUS transition. Best-effort: if the file is absent or unreadable
// the monitor starts with whatever was persisted (or UNKNOWN).
func (m *LogMonitor) seedFromTail() {
	f, err := os.Open(m.cfg.ConsensusLogFile)
	if err != nil {
		m.setConnErr(ReasonLogFileUnreadable, err.Error())
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return
	}

	// Scan from the end (up to 512 KB) to find the last PLATFORM_STATUS line.
	const tailSize int64 = 512 * 1024
	start := info.Size() - tailSize
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return
	}

	lastCode := operatorv1alpha1.PlatformStatusCodeUnknown
	var lastName string
	m.skipping = start > 0 // discard partial line from mid-file seek
	bytesRead := m.scanLines(f, func(line string) {
		if code, name, ok := parsePlatformStatusLog(line); ok {
			lastCode = code
			lastName = name
		}
	})

	if lastName != "" {
		m.updateStatus(lastCode, lastName)
	}

	m.offset = start + bytesRead
	m.clearConnErr()
}

// poll reads new data from the log file since the last offset.
func (m *LogMonitor) poll() {
	f, err := os.Open(m.cfg.ConsensusLogFile)
	if err != nil {
		m.setConnErr(ReasonLogFileUnreadable, err.Error())
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		m.setConnErr(ReasonLogFileUnreadable, err.Error())
		return
	}
	m.clearConnErr()

	// Detect rotation: file shrank.
	if info.Size() < m.offset {
		m.offset = 0
		m.skipping = false
	}

	if info.Size() == m.offset {
		return
	}

	if _, err := f.Seek(m.offset, io.SeekStart); err != nil {
		return
	}

	m.offset += m.scanLines(f, func(line string) {
		if code, name, ok := parsePlatformStatusLog(line); ok {
			m.updateStatus(code, name)
		}
	})
}

func (m *LogMonitor) scanLines(r io.Reader, fn func(line string)) int64 {
	m.lineReader.Reset(r)
	var consumed int64
	for {
		chunk, err := m.lineReader.ReadSlice('\n')
		switch {
		case err == nil:
			consumed += int64(len(chunk))
			if m.skipping {
				m.skipping = false
				continue
			}
			fn(strings.TrimRight(string(chunk), "\r\n"))
		case errors.Is(err, bufio.ErrBufferFull):
			consumed += int64(len(chunk))
			if !m.skipping {
				m.skipping = true
				logx.As().Warn().
					Str("reason", ReasonLogLineTooLong.String()).
					Str("file", m.cfg.ConsensusLogFile).
					Int("limit", maxLogLineSize).
					Msg("Skipping consensus log line longer than the limit")
			}
		default:
			if m.skipping {
				consumed += int64(len(chunk))
			}
			return consumed
		}
	}
}

// updateStatus publishes the new status and persists it.
func (m *LogMonitor) updateStatus(code int32, name string) {
	prev := m.snapshot.Load()
	m.snapshot.Store(&LogStatusSnapshot{Code: code, Status: name})

	prevName := ""
	if prev != nil {
		prevName = prev.Status
	}
	if prevName != name {
		from := prevName
		if from == "" {
			from = nonePhase
		}
		logx.As().Info().
			Str("reason", ReasonLogStatusChanged.String()).
			Str("from", prevName).Str("to", name).Int32("code", code).
			Msgf("Detected platform status transition: %s → %s (code %d)", from, name, code)
	}

	m.persist(code, name)
}

func (m *LogMonitor) persist(code int32, name string) {
	data, err := json.Marshal(logPersistedState{Code: code, Status: name})
	if err != nil {
		return
	}
	_ = os.WriteFile(m.persistPath, data, 0o644)
}

// LogFileMtime returns the mtime of the configured consensus log file in unix
// seconds, or 0 if the file is absent.
func (m *LogMonitor) LogFileMtime() float64 {
	info, err := os.Stat(m.cfg.ConsensusLogFile)
	if err != nil {
		return 0
	}
	return float64(info.ModTime().Unix())
}

func (m *LogMonitor) setConnErr(reason errx.Reason, message string) {
	m.connectivityErr.Store(&daemonkit.StatusError{
		Reason:     reason.String(),
		Message:    message,
		Resolution: "Confirm the consensus log file exists at " + m.cfg.ConsensusLogFile,
		Since:      time.Now().UTC().Format(time.RFC3339),
	})
}

func (m *LogMonitor) clearConnErr() {
	m.connectivityErr.Store(nil)
}

// platformStatusLogJSON is the JSON payload embedded in PLATFORM_STATUS lines.
type platformStatusLogJSON struct {
	OldStatus string `json:"oldStatus"`
	NewStatus string `json:"newStatus"`
}

// parsePlatformStatusLog extracts the platform status from a log line containing
// PLATFORM_STATUS. It anchors on the JSON {"oldStatus":...,"newStatus":...}
// payload, not the human-readable sentence (robust to log format changes).
func parsePlatformStatusLog(line string) (code int32, name string, ok bool) {
	if !strings.Contains(line, platformStatusTag) {
		return 0, "", false
	}

	jsonStart := strings.Index(line, "{\"oldStatus\"")
	if jsonStart < 0 {
		jsonStart = strings.Index(line, "{\"newStatus\"")
	}
	if jsonStart < 0 {
		return 0, "", false
	}

	jsonEnd := strings.Index(line[jsonStart:], "}") + jsonStart + 1
	if jsonEnd <= jsonStart {
		return 0, "", false
	}

	var payload platformStatusLogJSON
	if err := json.Unmarshal([]byte(line[jsonStart:jsonEnd]), &payload); err != nil {
		return 0, "", false
	}

	if payload.NewStatus == "" {
		return 0, "", false
	}

	// Never pass a raw log-supplied name on to the pod-condition patch.
	code = operatorv1alpha1.PlatformStatusCodeForName(payload.NewStatus)
	return code, nameForCode(code), true
}
