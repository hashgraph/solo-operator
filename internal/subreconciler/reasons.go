// SPDX-License-Identifier: Apache-2.0

package subreconciler

import (
	"github.com/automa-saga/errx"
)

// Reason codes for logx log events emitted by the subreconciler chain.
const (
	ReasonSubreconcilerSkipped  errx.Reason = "SubreconcilerSkipped"
	ReasonSubreconcilerRunning  errx.Reason = "SubreconcilerRunning"
	ReasonSubreconcilerFailed   errx.Reason = "SubreconcilerFailed"
	ReasonSubreconcilerConflict errx.Reason = "SubreconcilerConflict"
	ReasonSubreconcilerRequeue  errx.Reason = "SubreconcilerRequeue"
	ReasonResourceCreating      errx.Reason = "ResourceCreating"
	ReasonResourceUpdating      errx.Reason = "ResourceUpdating"
)
