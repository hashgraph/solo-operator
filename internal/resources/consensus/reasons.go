// SPDX-License-Identifier: Apache-2.0

package consensus

import "github.com/hashgraph/solo-operator/internal/errx"

// Consensus-specific reason codes. Generic K8s lifecycle codes live in internal/common.
const (
	ReasonOrbitDefaultsResolved errx.Reason = "OrbitDefaultsResolved"
	ReasonUCModeResolved        errx.Reason = "UCModeResolved"
)
