// SPDX-License-Identifier: Apache-2.0

package ops

import "github.com/automa-saga/errx"

// Reason codes for the ops package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see docs/beacon/design.md §8.3 and CLAUDE.md "Error Handling").
const (
	reasonInvalidStartTime errx.Reason = "InvalidStartTime"
	reasonInvalidHash      errx.Reason = "InvalidUpgradeHash"

	// Special-file upload (design §8.1).
	reasonEmptyUpload            errx.Reason = "EmptyUpload"
	reasonFileInfoFailed         errx.Reason = "FileInfoFailed"
	reasonUploadedHashMismatch   errx.Reason = "UploadedHashMismatch"
	reasonCheckpointHashMismatch errx.Reason = "CheckpointHashMismatch"
	// Log-only reasons for upload progress and resume.
	reasonUploadStarted  errx.Reason = "UploadStarted"
	reasonUploadProgress errx.Reason = "UploadProgress"
	reasonUploadResumed  errx.Reason = "UploadResumed"

	// Verification queries (design §6.2).
	reasonVersionQueryFailed errx.Reason = "VersionQueryFailed"

	// Genesis DAB registration (HIP-869 node create-from-genesis).
	reasonGenesisNetworkInvalid errx.Reason = "GenesisNetworkInvalid"
	reasonNodeIDMismatch        errx.Reason = "NodeIDMismatch"
)
