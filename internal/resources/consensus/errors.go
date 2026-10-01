// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("consensus")

	// errConfigInvalid: a capsule field required for sidecar provisioning is
	// missing or invalid. Terminal — the operator must correct the spec.
	errConfigInvalid = errNS.NewType("config_invalid")

	// errExecTransport: the pod is not found/ready or the exec stream failed to
	// establish. Retryable — the pod may still be starting or rescheduling.
	errExecTransport = errNS.NewType("exec_transport", errorx.Temporary())

	// errExecPodNotReady: the exec transport failed because the pod/container
	// does not exist yet (e.g. during a StatefulSet rollout). Retryable and
	// expected during upgrades — callers should requeue quietly.
	errExecPodNotReady = errNS.NewType("exec_pod_not_ready", errorx.Temporary())

	// errExecCommand: the s6 command ran but exited non-zero. Terminal — retrying
	// the same command will not change the outcome.
	errExecCommand = errNS.NewType("exec_command")
)

const (
	// ReasonOrbitRequired is returned when a sidecar that requires spec.orbit
	// (e.g. the UC sidecar) is enabled but the field is empty.
	ReasonOrbitRequired errx.Reason = "OrbitRequired"

	ReasonImageRequired   errx.Reason = "ImageRequired"
	ReasonBlobURLRequired errx.Reason = "BlobURLRequired"
	ReasonOrbitNotFound   errx.Reason = "OrbitNotFound"
)
