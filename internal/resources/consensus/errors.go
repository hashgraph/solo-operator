// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

var (
	errNS = errorx.NewNamespace("consensus")

	// errConfigInvalid: a capsule field required for sidecar provisioning is
	// missing or invalid. Terminal — the operator must correct the spec.
	errConfigInvalid = errNS.NewType("config_invalid")

	// Transient: retried once the Orbit is created.
	errOrbitNotFound = errNS.NewType("orbit_not_found", errorx.Temporary())

	// errExecTransport: the pod is not found/ready or the exec stream failed to
	// establish. Retryable — the pod may still be starting or rescheduling.
	errExecTransport = errNS.NewType("exec_transport", errorx.Temporary())

	// errExecCommand: the s6 command ran but exited non-zero. Terminal — retrying
	// the same command will not change the outcome.
	errExecCommand = errNS.NewType("exec_command")
)

const (
	// ReasonOrbitRequired is returned when a sidecar that requires spec.orbit
	// (e.g. the UC sidecar) is enabled but the field is empty.
	ReasonOrbitRequired errx.Reason = "OrbitRequired"

	ReasonImageRequired    errx.Reason = "ImageRequired"
	ReasonBlobURLRequired  errx.Reason = "BlobURLRequired"
	ReasonOrbitNameMissing errx.Reason = "OrbitNameMissing"
	ReasonOrbitNotFound    errx.Reason = "OrbitNotFound"
)
