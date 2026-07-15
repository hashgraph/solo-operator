// SPDX-License-Identifier: Apache-2.0

package errx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

var (
	testNS   = errorx.NewNamespace("errx_test")
	testType = testNS.NewType("boom")
)

func TestWithReasonAndHints(t *testing.T) {
	err := errx.WithHints(
		errx.WithReason(testType.New("something broke"), "SomethingBroke"),
		"Try turning it off and on again",
		"Check the logs",
	)

	reason, ok := errx.ReasonOf(err)
	if !ok || reason != "SomethingBroke" {
		t.Fatalf("Reason() = %q, %v; want SomethingBroke, true", reason, ok)
	}
	hints, ok := errx.Hints(err)
	if !ok || len(hints) != 2 || hints[0] != "Try turning it off and on again" {
		t.Fatalf("Hints() = %v, %v", hints, ok)
	}
}

func TestReasonWalksCauseChain(t *testing.T) {
	// A reason set on an inner error must survive outer errorx wrapping.
	inner := errx.WithReason(testType.New("inner"), "InnerReason")
	outer := testType.Wrap(inner, "outer context")

	reason, ok := errx.ReasonOf(outer)
	if !ok || reason != "InnerReason" {
		t.Fatalf("Reason() through chain = %q, %v; want InnerReason, true", reason, ok)
	}
}

func TestReasonSurvivesFmtErrorfWrapper(t *testing.T) {
	// A reason/hint on an inner errx error must survive an intervening plain
	// fmt.Errorf("...: %w", err) propagation wrapper — the walk follows
	// errors.Unwrap through non-errorx layers.
	inner := errx.WithHints(errx.WithReason(testType.New("inner"), "InnerReason"), "do the thing")
	wrapped := fmt.Errorf("adding context: %w", inner)

	reason, ok := errx.ReasonOf(wrapped)
	if !ok || reason != "InnerReason" {
		t.Fatalf("Reason() through fmt.Errorf = %q, %v; want InnerReason, true", reason, ok)
	}
	hints, ok := errx.Hints(wrapped)
	if !ok || len(hints) != 1 || hints[0] != "do the thing" {
		t.Fatalf("Hints() through fmt.Errorf = %v, %v", hints, ok)
	}
}

func TestNilAndAbsent(t *testing.T) {
	if errx.WithReason(nil, "x") != nil {
		t.Fatal("WithReason(nil) should be nil")
	}
	if errx.WithHints(nil) != nil {
		t.Fatal("WithHints(nil) should be nil")
	}
	// A plain (non-errx) error has neither reason nor hints.
	plain := errors.New("plain")
	if _, ok := errx.ReasonOf(plain); ok {
		t.Fatal("plain error should have no reason")
	}
	if _, ok := errx.Hints(plain); ok {
		t.Fatal("plain error should have no hints")
	}
}

func TestWithHintsNoHintsIsPassthrough(t *testing.T) {
	base := testType.New("x")
	if got := errx.WithHints(base); got != error(base) {
		t.Fatal("WithHints with no hints should return the error unchanged")
	}
}
