// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
)

// wantReason asserts err carries the given errx reason code.
func wantReason(t *testing.T, err error, reason errx.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	got, ok := errx.ReasonOf(err)
	if !ok || got != reason {
		t.Errorf("reason = %q (ok=%v), want %q", got, ok, reason)
	}
}

func TestConfirm_YesBypasses(t *testing.T) {
	// --yes proceeds even on a non-interactive terminal, with no prompt written.
	var out bytes.Buffer
	c := confirmer{in: strings.NewReader(""), out: &out, isTTY: false}
	if err := c.confirm(true, "do the thing"); err != nil {
		t.Fatalf("confirm(yes) = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("prompt written despite --yes: %q", out.String())
	}
}

func TestConfirm_NonInteractiveRequiresYes(t *testing.T) {
	var out bytes.Buffer
	c := confirmer{in: strings.NewReader(""), out: &out, isTTY: false}
	err := c.confirm(false, "do the thing")
	wantReason(t, err, reasonConfirmationRequired)
}

func TestConfirm_TTYPromptsAndAccepts(t *testing.T) {
	for _, answer := range []string{"yes\n", "y\n", "YES\n"} {
		var out bytes.Buffer
		c := confirmer{in: strings.NewReader(answer), out: &out, isTTY: true}
		if err := c.confirm(false, "do the thing"); err != nil {
			t.Errorf("answer %q: confirm = %v, want nil", answer, err)
		}
		if !strings.Contains(out.String(), "irreversible") {
			t.Errorf("answer %q: expected a prompt mentioning irreversibility, got %q", answer, out.String())
		}
	}
}

func TestConfirm_TTYRejectsOnAnythingElse(t *testing.T) {
	for _, answer := range []string{"no\n", "\n", "nope\n"} {
		var out bytes.Buffer
		c := confirmer{in: strings.NewReader(answer), out: &out, isTTY: true}
		err := c.confirm(false, "do the thing")
		wantReason(t, err, reasonAborted)
	}
}
