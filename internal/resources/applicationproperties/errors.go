// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

var (
	errNS = errorx.NewNamespace("applicationproperties")

	// Transient: retried once the Orbit is created.
	errOrbitNotFound = errNS.NewType("orbit_not_found", errorx.Temporary())
)

const (
	ReasonOrbitNotFound errx.Reason = "OrbitNotFound"
)
