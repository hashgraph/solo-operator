// SPDX-License-Identifier: Apache-2.0

// Package errx defines the solo-operator standard for structured errors on top
// of github.com/joomcode/errorx. It owns two cross-cutting error properties —
// a machine-readable reason code and a list of human remediation hints — plus
// helpers to attach and extract them.
//
// # Convention
//
// Each subsystem package declares its own errorx namespace and a small set of
// typed errors in a local errors.go, e.g.:
//
//	var (
//		errNS              = errorx.NewNamespace("uc")
//		errManifestInvalid = errNS.NewType("manifest_invalid")
//		errK8sAPI          = errNS.NewType("k8s_api_failed", errorx.Temporary())
//	)
//
// Errors are created from those types and decorated with a reason + hints:
//
//		return errx.WithHints(
//			errx.WithReason(errManifestInvalid.Wrap(err, "parse %s", path), "ManifestInvalid"),
//			"Regenerate the deployment package",
//			"Confirm manifests/consensus-node-components.yaml has schemaVersion: 1",
//		)
//
//	  - The reason code is PascalCase and mirrors the logx `reason=` field so a
//	    log line, an error, and any /status output line up on the same identifier.
//	  - Hints are concrete next steps an operator can take; attach them only when
//	    a meaningful remediation exists (config/disk/RBAC), not for internal bugs.
//	  - Mark transient errors (worth a retry) with the errorx.Temporary() trait on
//	    the type; terminal errors carry no such trait.
package errx

import (
	"errors"
	"strings"

	"github.com/joomcode/errorx"
)

// Reason is a stable, machine-readable error/condition code. Callers define
// their own vocabulary as typed constants (e.g. `const ReasonX errx.Reason =
// "X"`) so reason codes are enumerable in one place and typo-safe at call
// sites. The convention is PascalCase, mirroring the logx `reason=` field so an
// error, its log line, and any status output share one identifier.
type Reason string

// String returns the raw reason code, for use as a log field value
// (`ev.Str("reason", r.String())`).
func (r Reason) String() string { return string(r) }

var (
	// PropertyReason carries the Reason code. It is printable, so it renders in
	// the error string for at-a-glance correlation.
	PropertyReason = errorx.RegisterPrintableProperty("reason")

	// PropertyResolution carries a []string of remediation hints — concrete next
	// steps an operator can take. It is surfaced by logging / diagnostic tooling
	// rather than embedded in the error message.
	PropertyResolution = errorx.RegisterProperty("resolution")
)

// WithReason attaches a Reason code to err. err should already be an errorx
// error (from a Type.New/.Wrap); a plain error is wrapped so the reason is never
// silently dropped. Returns nil when err is nil.
func WithReason(err error, reason Reason) error {
	if err == nil {
		return nil
	}
	return ensure(err).WithProperty(PropertyReason, reason.String())
}

// WithHints attaches remediation hints to err (err is wrapped if it is not
// already an errorx error). A call with no hints returns err unchanged.
func WithHints(err error, hints ...string) error {
	if err == nil || len(hints) == 0 {
		return err
	}
	return ensure(err).WithProperty(PropertyResolution, hints)
}

// Decorate attaches a reason code and optional hints in one call — the common
// case, equivalent to WithHints(WithReason(err, reason), hints...). Returns nil
// when err is nil.
func Decorate(err error, reason Reason, hints ...string) error {
	return WithHints(WithReason(err, reason), hints...)
}

// ReasonOf returns the Reason code attached anywhere in err's cause chain.
func ReasonOf(err error) (Reason, bool) {
	if v, ok := extract(err, PropertyReason); ok {
		if s, ok := v.(string); ok {
			return Reason(s), true
		}
	}
	return "", false
}

// Hints returns the remediation hints attached anywhere in err's cause chain.
func Hints(err error) ([]string, bool) {
	if v, ok := extract(err, PropertyResolution); ok {
		if s, ok := v.([]string); ok {
			return s, true
		}
	}
	return nil, false
}

// Format renders err for a human surface (a CLI failure, a fatal startup log):
// the full error message followed by a "Resolution:" block listing any attached
// remediation hints. When no hints are attached it returns err.Error()
// unchanged. Returns "" for a nil error. This is the doctor-style renderer; for
// structured logging use ReasonOf/Hints to attach fields instead.
func Format(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	hints, ok := Hints(err)
	if !ok || len(hints) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\nResolution:")
	for _, h := range hints {
		b.WriteString("\n  - ")
		b.WriteString(h)
	}
	return b.String()
}

// ensure returns err as an *errorx.Error, wrapping a non-errorx error so a
// property is never dropped. Converted code always passes errorx errors, so the
// wrap branch is a defensive fallback only.
func ensure(err error) *errorx.Error {
	if ex := errorx.Cast(err); ex != nil {
		return ex
	}
	return errorx.InternalError.Wrap(err, "unexpected non-errorx error")
}

// extract walks the cause chain for key (errorx.ExtractProperty inspects only the
// outermost error). It follows both the errorx cause chain and errors.Unwrap, so
// a reason/hint survives an intervening plain fmt.Errorf("...: %w") wrapper.
func extract(err error, key errorx.Property) (interface{}, bool) {
	for e := err; e != nil; {
		if v, ok := errorx.ExtractProperty(e, key); ok {
			return v, true
		}
		if ex := errorx.Cast(e); ex != nil {
			e = ex.Cause()
			continue
		}
		e = errors.Unwrap(e)
	}
	return nil, false
}
