// SPDX-License-Identifier: Apache-2.0

package ops

import "github.com/joomcode/errorx"

// ops error taxonomy. Each error this package returns is created from one of
// these types and decorated with a reason code + remediation hints via the
// internal/errx helpers, so the base is a real errorx error rather than the
// generic "unexpected non-errorx error" fallback. The errorx.Temporary() trait
// marks a network read worth retrying; validation errors are terminal because
// the same input will fail the same way.
var (
	errNS = errorx.NewNamespace("beacon.ops")

	// errQuery: a read-back query (network version, file info) failed at the
	// transport/consensus layer. Transient — worth a retry once the node settles.
	errQuery = errNS.NewType("query", errorx.Temporary())

	// errValidation: a local input failed validation before or after a network
	// round-trip — an empty upload, a malformed start time or hash, or a staged
	// hash that does not match. Terminal.
	errValidation = errNS.NewType("validation")

	// errNodeIDMismatch: the network assigned a NodeCreateTransaction a
	// different node ID than the classic address book expects. Terminal — the
	// mismatch cannot be retried away.
	errNodeIDMismatch = errNS.NewType("node_id_mismatch")
)
