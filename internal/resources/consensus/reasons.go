// SPDX-License-Identifier: Apache-2.0

package consensus

import "github.com/automa-saga/errx"

// Consensus-specific reason codes. Generic K8s lifecycle codes live in internal/common.
const (
	ReasonOrbitDefaultsResolved errx.Reason = "OrbitDefaultsResolved"

	ReasonHostPathVolume errx.Reason = "HostPathVolume"

	// ReasonExecTransportFailed: the pods/exec stream could not be established
	// (pod not found/ready, or transport error).
	ReasonExecTransportFailed errx.Reason = "ExecTransportFailed"

	// ReasonExecForbidden: the API server rejected the pods/exec request with a
	// 403. The WebSocket exec transport authorizes against pods/exec with the
	// `get` verb; a forbidden here means the operator ServiceAccount lacks it.
	ReasonExecForbidden errx.Reason = "ExecForbidden"

	// ReasonExecCommandFailed: the s6 command exited non-zero.
	ReasonExecCommandFailed errx.Reason = "ExecCommandFailed"

	// ReasonExecPodNotReady: the exec transport failed because the pod or
	// container does not exist yet (e.g. during a StatefulSet rollout).
	ReasonExecPodNotReady errx.Reason = "ExecPodNotReady"
)
