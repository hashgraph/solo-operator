// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// ReasonFrom returns the reason code carried by err, or fallback if it has none.
// Controllers use it to derive a Condition.Reason / Event.reason from an error so
// the identifier matches the error and its log line.
func ReasonFrom(err error, fallback errx.Reason) string {
	if r, ok := errx.ReasonOf(err); ok {
		return r.String()
	}
	return fallback.String()
}

// MessageWithHints renders err's message plus any remediation hints on a single
// line, for a Condition.Message or Event message (where errx.Format's multi-line
// block is undesirable).
func MessageWithHints(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	hints, ok := errx.Hints(err)
	if !ok || len(hints) == 0 {
		return msg
	}
	return msg + " (resolution: " + strings.Join(hints, "; ") + ")"
}
