// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("applicationproperties")

	// Transient: retried once the Orbit is created.
	errOrbitNotFound = errNS.NewType("orbit_not_found", errorx.Temporary())
)

const (
	ReasonOrbitNotFound errx.Reason = "OrbitNotFound"
)
