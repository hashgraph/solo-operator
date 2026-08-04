// SPDX-License-Identifier: Apache-2.0

package commands

import "github.com/automa-saga/errx"

// Reason codes for the commands package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see CLAUDE.md "Error Handling").
const (
	reasonConfirmationRequired errx.Reason = "ConfirmationRequired"
	reasonAborted              errx.Reason = "Aborted"
)
