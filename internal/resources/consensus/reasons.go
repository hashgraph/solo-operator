// SPDX-License-Identifier: Apache-2.0

package consensus

import "github.com/automa-saga/errx"

// Consensus-specific reason codes. Generic K8s lifecycle codes live in internal/common.
const (
	ReasonOrbitDefaultsResolved errx.Reason = "OrbitDefaultsResolved"
	ReasonUCModeResolved        errx.Reason = "UCModeResolved"

	// ReasonExecTransportFailed: the pods/exec stream could not be established
	// (pod not found/ready, or transport error).
	ReasonExecTransportFailed errx.Reason = "ExecTransportFailed"

	// ReasonExecCommandFailed: the s6 command exited non-zero.
	ReasonExecCommandFailed errx.Reason = "ExecCommandFailed"
)
