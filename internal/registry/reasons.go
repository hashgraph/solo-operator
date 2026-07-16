// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/hashgraph/solo-operator/internal/errx"
)

// Reason codes for logx log events emitted during component revalidation.
// Error-decoration reason codes live alongside their typed errors in errors.go.
const (
	ReasonNoSelectedSource errx.Reason = "NoSelectedSource"
	ReasonComponentInvalid errx.Reason = "ComponentInvalid"
)
