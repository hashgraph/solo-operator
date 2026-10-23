// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// wantReason asserts err carries the given errx reason code.
func wantReason(t *testing.T, err error, reason errx.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	got, ok := errx.ReasonOf(err)
	if !ok || got != reason {
		t.Errorf("reason = %q (ok=%v), want %q", got, ok, reason)
	}
}

var (
	file150  = hiero.FileID{Shard: 0, Realm: 0, File: 150}
	file159  = hiero.FileID{Shard: 0, Realm: 0, File: 159}
	testHash = bytes.Repeat([]byte{0xAB}, sha384Len)
	startAt  = time.Date(2026, 8, 1, 14, 30, 0, 0, time.UTC)
)

// assertNoStartTime fails if the transaction carries a start time.
func assertNoStartTime(t *testing.T, tx *hiero.FreezeTransaction) {
	t.Helper()
	if !tx.GetStartTime().IsZero() {
		t.Errorf("start_time set = %v, want unset", tx.GetStartTime())
	}
}

// assertNoFile fails if the transaction carries an update file or file hash.
func assertNoFile(t *testing.T, tx *hiero.FreezeTransaction) {
	t.Helper()
	if id := tx.GetFileID(); id != nil && (id.File != 0 || id.Shard != 0 || id.Realm != 0) {
		t.Errorf("update_file set = %v, want unset", id)
	}
	if len(tx.GetFileHash()) != 0 {
		t.Errorf("file_hash set = %x, want unset", tx.GetFileHash())
	}
}

// assertNoEndTime guards the deprecated end_time field (start_hour/min etc. have
// no SDK setters, so they can never be set; end_time is the one reachable one).
func assertNoEndTime(t *testing.T, tx *hiero.FreezeTransaction) {
	t.Helper()
	if !tx.GetEndTime().IsZero() {
		t.Errorf("end_time set = %v, want unset (deprecated window fields unused)", tx.GetEndTime())
	}
}

func TestPrepareUpgrade_Fields(t *testing.T) {
	tx := PrepareUpgrade(file150, testHash)
	if tx.GetFreezeType() != hiero.FreezeTypePrepareUpgrade {
		t.Errorf("freeze_type = %v, want PREPARE_UPGRADE", tx.GetFreezeType())
	}
	if id := tx.GetFileID(); id == nil || *id != file150 {
		t.Errorf("update_file = %v, want %v", id, file150)
	}
	if !bytes.Equal(tx.GetFileHash(), testHash) {
		t.Errorf("file_hash = %x, want %x", tx.GetFileHash(), testHash)
	}
	assertNoStartTime(t, tx) // preparation is immediate
	assertNoEndTime(t, tx)
}

func TestFreezeUpgrade_Fields(t *testing.T) {
	tx := FreezeUpgrade(startAt, file150, testHash)
	if tx.GetFreezeType() != hiero.FreezeTypeFreezeUpgrade {
		t.Errorf("freeze_type = %v, want FREEZE_UPGRADE", tx.GetFreezeType())
	}
	if !tx.GetStartTime().Equal(startAt) {
		t.Errorf("start_time = %v, want %v", tx.GetStartTime(), startAt)
	}
	if id := tx.GetFileID(); id == nil || *id != file150 {
		t.Errorf("update_file = %v, want %v", id, file150)
	}
	if !bytes.Equal(tx.GetFileHash(), testHash) {
		t.Errorf("file_hash = %x, want %x", tx.GetFileHash(), testHash)
	}
	assertNoEndTime(t, tx)
}

func TestUpgradeTelemetry_Fields(t *testing.T) {
	tx := UpgradeTelemetry(startAt, file159, testHash)
	if tx.GetFreezeType() != hiero.FreezeTypeTelemetryUpgrade {
		t.Errorf("freeze_type = %v, want TELEMETRY_UPGRADE", tx.GetFreezeType())
	}
	if !tx.GetStartTime().Equal(startAt) {
		t.Errorf("start_time = %v, want %v", tx.GetStartTime(), startAt)
	}
	if id := tx.GetFileID(); id == nil || *id != file159 {
		t.Errorf("update_file = %v, want %v (0.0.159)", id, file159)
	}
	if !bytes.Equal(tx.GetFileHash(), testHash) {
		t.Errorf("file_hash = %x, want %x", tx.GetFileHash(), testHash)
	}
	assertNoEndTime(t, tx)
}

func TestFreezeOnly_Fields(t *testing.T) {
	tx := FreezeOnly(startAt)
	if tx.GetFreezeType() != hiero.FreezeTypeFreezeOnly {
		t.Errorf("freeze_type = %v, want FREEZE_ONLY", tx.GetFreezeType())
	}
	if !tx.GetStartTime().Equal(startAt) {
		t.Errorf("start_time = %v, want %v", tx.GetStartTime(), startAt)
	}
	assertNoFile(t, tx) // freeze-only changes no software
	assertNoEndTime(t, tx)
}

func TestFreezeAbort_Fields(t *testing.T) {
	tx := FreezeAbort()
	if tx.GetFreezeType() != hiero.FreezeTypeFreezeAbort {
		t.Errorf("freeze_type = %v, want FREEZE_ABORT", tx.GetFreezeType())
	}
	assertNoStartTime(t, tx)
	assertNoFile(t, tx)
	assertNoEndTime(t, tx)
}

// TestPrepareAndFreezeUpgrade_ShareHash pins the §7 contract: the hash staged by
// prepare-upgrade must be the exact hash freeze-upgrade later commits to.
func TestPrepareAndFreezeUpgrade_ShareHash(t *testing.T) {
	prep := PrepareUpgrade(file150, testHash)
	frz := FreezeUpgrade(startAt, file150, testHash)
	if !bytes.Equal(prep.GetFileHash(), frz.GetFileHash()) {
		t.Errorf("prepare/freeze hashes differ: %x vs %x", prep.GetFileHash(), frz.GetFileHash())
	}
	if *prep.GetFileID() != *frz.GetFileID() {
		t.Errorf("prepare/freeze update files differ: %v vs %v", prep.GetFileID(), frz.GetFileID())
	}
}

func TestParseStartTime(t *testing.T) {
	now := startAt.Add(-time.Minute)
	got, err := ParseStartTime("2026-08-01.14:30:00", now)
	if err != nil {
		t.Fatalf("ParseStartTime: %v", err)
	}
	if !got.Equal(startAt) {
		t.Errorf("parsed = %v, want %v", got, startAt)
	}
	if got.Location() != time.UTC {
		t.Errorf("parsed location = %v, want UTC", got.Location())
	}

	t.Run("rejects wrong format", func(t *testing.T) {
		_, err := ParseStartTime("2026-08-01T14:30:00Z", now)
		wantReason(t, err, reasonInvalidStartTime)
	})

	for name, now := range map[string]time.Time{
		"rejects past":  startAt.Add(time.Minute),
		"rejects equal": startAt,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseStartTime("2026-08-01.14:30:00", now)
			wantReason(t, err, reasonStartTimeNotFuture)
		})
	}
}

func TestParseUpgradeHash(t *testing.T) {
	hexStr := strings.Repeat("ab", sha384Len) // 48 bytes
	got, err := ParseUpgradeHash(hexStr)
	if err != nil {
		t.Fatalf("ParseUpgradeHash: %v", err)
	}
	if !bytes.Equal(got, testHash) {
		t.Errorf("decoded = %x, want %x", got, testHash)
	}

	t.Run("rejects empty", func(t *testing.T) {
		_, err := ParseUpgradeHash("")
		wantReason(t, err, reasonInvalidHash)
	})
	t.Run("rejects non-hex", func(t *testing.T) {
		_, err := ParseUpgradeHash("zzzz")
		wantReason(t, err, reasonInvalidHash)
	})
	t.Run("rejects wrong length", func(t *testing.T) {
		_, err := ParseUpgradeHash(strings.Repeat("ab", 32)) // 32 bytes, not 48
		wantReason(t, err, reasonInvalidHash)
	})
}
