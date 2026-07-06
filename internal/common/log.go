// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/rs/zerolog"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// LogErr enriches a zerolog event (e.g. logx.As().Error()) with the structured
// reason + remediation hints carried by err via internal/errx, then attaches the
// error itself. The reason= field uses the error's own Reason when present,
// otherwise fallback — so a log line shares the same identifier as the error and
// surfaces any actionable next steps as a `hints` array. Returns the event for
// chaining (add more fields, then .Msg). errx stays logger-agnostic; this
// caller-side glue is where the zerolog dependency lives, shared across
// solo-operator packages.
func LogErr(ev *zerolog.Event, err error, fallback errx.Reason) *zerolog.Event {
	reason := fallback
	if r, ok := errx.ReasonOf(err); ok {
		reason = r
	}
	ev = ev.Err(err).Str("reason", reason.String())
	if hints, ok := errx.Hints(err); ok {
		ev = ev.Strs("hints", hints)
	}
	return ev
}
