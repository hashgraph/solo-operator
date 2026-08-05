// SPDX-License-Identifier: Apache-2.0

package commands

import "github.com/joomcode/errorx"

// commands error taxonomy. Each error this package returns is created from one
// of these types and decorated with a reason code + remediation hints via the
// internal/errx helpers, so the base is a real errorx error rather than the
// generic "unexpected non-errorx error" fallback. All are terminal: bad input
// or a declined confirmation is fixed by the operator, not by a retry.
var (
	errNS = errorx.NewNamespace("beacon.commands")

	// errInput: a command's local input is unusable — an unreadable/mismatched
	// package zip, a malformed expected hash, or a non-privileged payer for a
	// special-file write. Terminal.
	errInput = errNS.NewType("input")

	// errConfirm: an irreversible command was not confirmed (non-interactive
	// without --yes, or the operator declined). Terminal.
	errConfirm = errNS.NewType("confirm")
)
