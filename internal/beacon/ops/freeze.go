// SPDX-License-Identifier: Apache-2.0

// Package ops builds the individual HAPI operations beacon submits. Each builder
// returns a ready-to-submit hiero transaction (or a query) with exactly the
// fields the operation requires and nothing more; submission and outcome
// verification are the client package's job.
//
// The freeze family (design §6.2, §9) is the core upgrade trigger: every variant
// is a single FreezeService.freeze call differing only in which of
// {freeze_type, start_time, update_file, file_hash} it sets. The deprecated
// start_hour/min and end_hour/min fields are never used.
package ops

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// StartTimeLayout is yahcli's UTC start-time format (Go reference layout). The
// parsed instant is in UTC, which is what FreezeTransaction.SetStartTime takes.
const StartTimeLayout = "2006-01-02.15:04:05"

// sha384Len is the byte length of a SHA-384 digest — the upgrade zip hash
// contract (§7). A hex --upgrade-zip-hash must decode to exactly this.
const sha384Len = 48

// PrepareUpgrade builds a PREPARE_UPGRADE freeze: it stages the software on the
// nodes from updateFile, pinned to hash. No start time — preparation is
// immediate. hash must be the SHA-384 of the staged updateFile contents, the
// same value later passed to FreezeUpgrade.
func PrepareUpgrade(updateFile hiero.FileID, hash []byte) *hiero.FreezeTransaction {
	return hiero.NewFreezeTransaction().
		SetFreezeType(hiero.FreezeTypePrepareUpgrade).
		SetFileID(updateFile).
		SetFileHash(hash)
}

// FreezeUpgrade builds a FREEZE_UPGRADE freeze: at start the network halts and
// applies the software previously staged by PrepareUpgrade. updateFile and hash
// must match the PrepareUpgrade that staged them.
func FreezeUpgrade(start time.Time, updateFile hiero.FileID, hash []byte) *hiero.FreezeTransaction {
	return hiero.NewFreezeTransaction().
		SetFreezeType(hiero.FreezeTypeFreezeUpgrade).
		SetStartTime(start).
		SetFileID(updateFile).
		SetFileHash(hash)
}

// UpgradeTelemetry builds a TELEMETRY_UPGRADE freeze: at start the network
// applies the telemetry/observability bundle staged in telemetryFile (0.0.159 by
// default), pinned to hash.
func UpgradeTelemetry(start time.Time, telemetryFile hiero.FileID, hash []byte) *hiero.FreezeTransaction {
	return hiero.NewFreezeTransaction().
		SetFreezeType(hiero.FreezeTypeTelemetryUpgrade).
		SetStartTime(start).
		SetFileID(telemetryFile).
		SetFileHash(hash)
}

// FreezeOnly builds a FREEZE_ONLY freeze: the network halts at start without any
// software change. No update file or hash.
func FreezeOnly(start time.Time) *hiero.FreezeTransaction {
	return hiero.NewFreezeTransaction().
		SetFreezeType(hiero.FreezeTypeFreezeOnly).
		SetStartTime(start)
}

// FreezeAbort builds a FREEZE_ABORT freeze: it cancels a scheduled freeze/upgrade
// before it fires. No start time, update file, or hash.
func FreezeAbort() *hiero.FreezeTransaction {
	return hiero.NewFreezeTransaction().
		SetFreezeType(hiero.FreezeTypeFreezeAbort)
}

// ParseStartTime parses yahcli's UTC start-time literal (yyyy-MM-dd.HH:mm:ss)
// into a time.Time. The result is in UTC.
func ParseStartTime(s string) (time.Time, error) {
	t, err := time.Parse(StartTimeLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, errx.Decorate(
			fmt.Errorf("parse start time %q: %w", s, err),
			reasonInvalidStartTime,
			fmt.Sprintf("Use a UTC time in the form %s, e.g. 2026-08-01.14:30:00", "yyyy-MM-dd.HH:mm:ss"),
		)
	}
	return t, nil
}

// ParseUpgradeHash decodes a hex SHA-384 upgrade-zip hash into raw bytes for
// FreezeTransaction.SetFileHash. It requires a well-formed 48-byte (SHA-384)
// digest, catching a truncated or wrong-algorithm hash before submission.
func ParseUpgradeHash(s string) ([]byte, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, errx.Decorate(
			fmt.Errorf("empty upgrade hash"),
			reasonInvalidHash,
			"Pass --upgrade-zip-hash with the SHA-384 hex printed by `beacon package software-zip`",
		)
	}
	raw, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, errx.Decorate(
			fmt.Errorf("decode upgrade hash %q: %w", s, err),
			reasonInvalidHash,
			"The hash must be hex (SHA-384); copy it from `beacon file info software-zip` or `package software-zip`",
		)
	}
	if len(raw) != sha384Len {
		return nil, errx.Decorate(
			fmt.Errorf("upgrade hash is %d bytes, want %d (SHA-384)", len(raw), sha384Len),
			reasonInvalidHash,
			"Confirm the value is a full SHA-384 digest (96 hex characters)",
		)
	}
	return raw, nil
}
