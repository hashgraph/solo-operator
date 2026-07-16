// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/automa-saga/logx"
	"github.com/rs/zerolog"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// stampFields appends the reconcile-scoped fields from ctx onto ev. Stamping on
// the event (not via Logger.With) keeps disabled lines free: Event.Str no-ops on
// a nil, level-filtered event, so no sub-logger is allocated.
func stampFields(ctx context.Context, ev *zerolog.Event) *zerolog.Event {
	f, ok := logFieldsFrom(ctx)
	if !ok {
		return ev
	}
	ev = ev.Str("controller", f.Controller).
		Str("kind", f.Kind).
		Str("namespace", f.Namespace).
		Str("name", f.Name).
		Str("reconcileID", f.ReconcileID)
	if f.OperationID != "" {
		ev = ev.Str("operation_id", f.OperationID)
	}
	return ev
}

// LoggerFrom returns a logx sub-logger with the reconcile-scoped fields baked
// in, for the build-once-reuse pattern via the *L helpers. For one-off lines
// prefer Debug/Info/Warn/Err.
func LoggerFrom(ctx context.Context) *zerolog.Logger {
	base := logx.As()
	f, ok := logFieldsFrom(ctx)
	if !ok {
		return base
	}
	c := base.With().
		Str("controller", f.Controller).
		Str("kind", f.Kind).
		Str("namespace", f.Namespace).
		Str("name", f.Name).
		Str("reconcileID", f.ReconcileID)
	if f.OperationID != "" {
		c = c.Str("operation_id", f.OperationID)
	}
	l := c.Logger()
	return &l
}

// Debug opens a debug event stamped with reason= and the ctx reconcile fields;
// chain fields and end with .Msg. Info and Warn are the same at their levels.
func Debug(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return stampFields(ctx, logx.As().Debug()).Str("reason", reason.String())
}

func Info(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return stampFields(ctx, logx.As().Info()).Str("reason", reason.String())
}

func Warn(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return stampFields(ctx, logx.As().Warn()).Str("reason", reason.String())
}

// Err opens an error event stamped with the ctx reconcile fields and err's errx
// reason and hints (falling back to fallback when err carries no reason).
func Err(ctx context.Context, err error, fallback errx.Reason) *zerolog.Event {
	return decorateErr(stampFields(ctx, logx.As().Error()), err, fallback)
}

// DebugL stamps reason= on a caller-provided sub-logger (see LoggerFrom).
// InfoL, WarnL, and ErrL are the same at their levels.
func DebugL(l *zerolog.Logger, reason errx.Reason) *zerolog.Event {
	return l.Debug().Str("reason", reason.String())
}

func InfoL(l *zerolog.Logger, reason errx.Reason) *zerolog.Event {
	return l.Info().Str("reason", reason.String())
}

func WarnL(l *zerolog.Logger, reason errx.Reason) *zerolog.Event {
	return l.Warn().Str("reason", reason.String())
}

func ErrL(l *zerolog.Logger, err error, fallback errx.Reason) *zerolog.Event {
	return decorateErr(l.Error(), err, fallback)
}

// LogErr enriches a pre-built event with err's errx reason and hints. Used by
// the uc daemon (no reconcile context); prefer Err/ErrL elsewhere.
func LogErr(ev *zerolog.Event, err error, fallback errx.Reason) *zerolog.Event {
	return decorateErr(ev, err, fallback)
}

func decorateErr(ev *zerolog.Event, err error, fallback errx.Reason) *zerolog.Event {
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
