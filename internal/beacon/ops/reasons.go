// SPDX-License-Identifier: Apache-2.0

package ops

import "github.com/automa-saga/errx"

// Reason codes for the ops package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see docs/beacon/design.md §8.3 and CLAUDE.md "Error Handling").
const (
	reasonInvalidStartTime errx.Reason = "InvalidStartTime"
	reasonInvalidHash      errx.Reason = "InvalidUpgradeHash"
)
