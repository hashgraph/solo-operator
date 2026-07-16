// SPDX-License-Identifier: Apache-2.0

package subreconciler

import (
	"github.com/hashgraph/solo-operator/internal/errx"
)

// Reason codes for logx log events emitted by the subreconciler chain.
const (
	ReasonSubreconcilerSkipped errx.Reason = "SubreconcilerSkipped"
	ReasonSubreconcilerRunning errx.Reason = "SubreconcilerRunning"
	ReasonSubreconcilerFailed  errx.Reason = "SubreconcilerFailed"
	ReasonSubreconcilerRequeue errx.Reason = "SubreconcilerRequeue"
	ReasonResourceCreating     errx.Reason = "ResourceCreating"
	ReasonResourceUpdating     errx.Reason = "ResourceUpdating"
)
