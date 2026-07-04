// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeDispatcher records which handlers the poll loop invokes and, optionally,
// fails a chosen marker so the scan's error tolerance can be exercised.
type fakeDispatcher struct {
	calls   map[string]int
	failOn  string
	failErr error
}

func newFakeDispatcher() *fakeDispatcher {
	return &fakeDispatcher{calls: map[string]int{}}
}

func (f *fakeDispatcher) record(marker string) error {
	f.calls[marker]++
	if f.failOn == marker {
		return f.failErr
	}
	return nil
}

func (f *fakeDispatcher) RecoverOnStartup(context.Context) error { return nil }
func (f *fakeDispatcher) HandleExecuteImmediate(context.Context) error {
	return f.record(MarkerExecuteImmediate)
}
func (f *fakeDispatcher) HandleFreezeScheduled(context.Context) error {
	return f.record(MarkerFreezeScheduled)
}
func (f *fakeDispatcher) HandleNowFrozen(context.Context) error {
	return f.record(MarkerNowFrozen)
}
func (f *fakeDispatcher) HandleFreezeAborted(context.Context) error {
	return f.record(MarkerFreezeAborted)
}

// newTestMonitor wires a MarkerMonitor against a temp marker directory and a
// fake dispatcher, with the documented default intervals.
func newTestMonitor(t *testing.T) (*MarkerMonitor, *fakeDispatcher, string) {
	t.Helper()
	dir := t.TempDir()
	disp := newFakeDispatcher()
	m := &MarkerMonitor{
		cfg: Config{
			MarkerPath:         dir,
			PollInterval:       DefaultPollInterval,
			FreezePollInterval: FreezePollInterval,
		},
		factory: disp,
	}
	return m, disp, dir
}

// touch creates an empty marker file in the monitor's marker directory.
func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
		t.Fatalf("write marker %s: %v", name, err)
	}
}

// TestPollDispatchesEachMarker asserts every recognised marker is routed to its
// matching handler exactly once per scan, and that an unrelated file is ignored.
func TestPollDispatchesEachMarker(t *testing.T) {
	for _, marker := range AllMarkers {
		t.Run(marker, func(t *testing.T) {
			m, disp, dir := newTestMonitor(t)
			touch(t, dir, marker)
			touch(t, dir, "unrelated.txt")

			m.poll(context.Background(), m.cfg.PollInterval)

			if disp.calls[marker] != 1 {
				t.Fatalf("%s dispatched %d times, want 1", marker, disp.calls[marker])
			}
			for _, other := range AllMarkers {
				if other != marker && disp.calls[other] != 0 {
					t.Fatalf("%s unexpectedly dispatched while only %s present", other, marker)
				}
			}
		})
	}
}

// TestPollIntervalTransitions covers the freeze-window cadence switch:
// 60s -> 1s on freeze_scheduled.mf, and 1s -> 60s on now_frozen.mf /
// freeze_aborted.mf. Markers persist on disk, so the next interval is recomputed
// from the full marker set on every tick.
func TestPollIntervalTransitions(t *testing.T) {
	tests := []struct {
		name    string
		markers []string
		current time.Duration
		want    time.Duration
	}{
		{
			name:    "freeze scheduled enters freeze window",
			markers: []string{MarkerFreezeScheduled},
			current: DefaultPollInterval,
			want:    FreezePollInterval,
		},
		{
			name:    "now frozen reverts to normal",
			markers: []string{MarkerFreezeScheduled, MarkerNowFrozen},
			current: FreezePollInterval,
			want:    DefaultPollInterval,
		},
		{
			name:    "freeze aborted reverts to normal",
			markers: []string{MarkerFreezeScheduled, MarkerFreezeAborted},
			current: FreezePollInterval,
			want:    DefaultPollInterval,
		},
		{
			name:    "execute immediate does not change cadence",
			markers: []string{MarkerExecuteImmediate},
			current: DefaultPollInterval,
			want:    DefaultPollInterval,
		},
		{
			name:    "no markers keeps current cadence",
			markers: nil,
			current: FreezePollInterval,
			want:    FreezePollInterval,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, _, dir := newTestMonitor(t)
			for _, marker := range tc.markers {
				touch(t, dir, marker)
			}
			if got := m.poll(context.Background(), tc.current); got != tc.want {
				t.Fatalf("next interval = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPollReDispatchesPersistentMarker documents the idempotent-friendly
// contract: markers are not deleted on detection (cleanup is #998), so the loop
// re-sees and re-dispatches the same marker every tick. Suppression of repeat
// work is the dedup cache's job (#997), not the poller's.
func TestPollReDispatchesPersistentMarker(t *testing.T) {
	m, disp, dir := newTestMonitor(t)
	touch(t, dir, MarkerExecuteImmediate)

	for i := 0; i < 3; i++ {
		m.poll(context.Background(), m.cfg.PollInterval)
	}
	if disp.calls[MarkerExecuteImmediate] != 3 {
		t.Fatalf("dispatched %d times across 3 polls, want 3", disp.calls[MarkerExecuteImmediate])
	}
}

// TestPollContinuesAfterHandlerError ensures a failing handler does not abort the
// scan: remaining markers are still detected and dispatched within the same tick.
func TestPollContinuesAfterHandlerError(t *testing.T) {
	m, disp, dir := newTestMonitor(t)
	disp.failOn = MarkerExecuteImmediate
	disp.failErr = errors.New("boom")
	touch(t, dir, MarkerExecuteImmediate)
	touch(t, dir, MarkerFreezeScheduled)

	next := m.poll(context.Background(), m.cfg.PollInterval)

	if disp.calls[MarkerExecuteImmediate] != 1 {
		t.Fatalf("failing handler called %d times, want 1", disp.calls[MarkerExecuteImmediate])
	}
	if disp.calls[MarkerFreezeScheduled] != 1 {
		t.Fatalf("subsequent marker dispatched %d times, want 1", disp.calls[MarkerFreezeScheduled])
	}
	if next != FreezePollInterval {
		t.Fatalf("next interval = %v, want %v (freeze window still entered)", next, FreezePollInterval)
	}
}

// TestPollMissingDirectoryIsTolerated verifies a missing marker directory yields
// no dispatches and no panic — the loop simply finds nothing.
func TestPollMissingDirectoryIsTolerated(t *testing.T) {
	m, disp, dir := newTestMonitor(t)
	m.cfg.MarkerPath = filepath.Join(dir, "does-not-exist")

	if got := m.poll(context.Background(), m.cfg.PollInterval); got != m.cfg.PollInterval {
		t.Fatalf("next interval = %v, want %v", got, m.cfg.PollInterval)
	}
	for _, marker := range AllMarkers {
		if disp.calls[marker] != 0 {
			t.Fatalf("%s dispatched against missing dir", marker)
		}
	}
}

// TestRunStopsOnContextCancel confirms Run returns cleanly when its context is
// cancelled rather than blocking on the timer.
func TestRunStopsOnContextCancel(t *testing.T) {
	m, _, _ := newTestMonitor(t)
	m.cfg.EventsPath = t.TempDir()
	m.cfg.CRPersistPath = t.TempDir()
	// Use a real factory so RecoverOnStartup (op-id cache seed) is exercised
	// against a fake cluster.
	m.factory = newFakeFactory(t, m.cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v, want nil on cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
