// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/automa-saga/logx"
	"github.com/rs/zerolog"

	"github.com/hashgraph/solo-operator/internal/errx"
)

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

// LoggerFrom returns a sub-logger with reconcile fields baked in; use *L helpers
// to avoid re-extracting context on every line. For one-offs prefer Debug/Info/Warn/Err.
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

func openEvent(ctx context.Context, ev *zerolog.Event, reason errx.Reason) *zerolog.Event {
	return stampFields(ctx, ev).Str("reason", reason.String())
}

func Debug(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return openEvent(ctx, logx.As().Debug(), reason)
}

func Info(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return openEvent(ctx, logx.As().Info(), reason)
}

func Warn(ctx context.Context, reason errx.Reason) *zerolog.Event {
	return openEvent(ctx, logx.As().Warn(), reason)
}

func Err(ctx context.Context, err error, fallback errx.Reason) *zerolog.Event {
	return decorateErr(stampFields(ctx, logx.As().Error()), err, fallback)
}

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

// LogErr enriches a pre-built event with errx reason and hints; used where no reconcile context exists.
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
