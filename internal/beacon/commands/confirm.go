// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/automa-saga/errx"
	"golang.org/x/term"
)

// confirmer gates irreversible commands (design §15 Q2). Fields are injectable so
// the gate is unit-testable without a real terminal.
type confirmer struct {
	in    io.Reader
	out   io.Writer
	isTTY bool
}

// defaultConfirmer prompts on stderr, reads stdin, and treats stdin's terminal
// status as the interactivity signal.
func defaultConfirmer() confirmer {
	return confirmer{
		in:    os.Stdin,
		out:   os.Stderr,
		isTTY: term.IsTerminal(int(os.Stdin.Fd())),
	}
}

// confirm gates an irreversible action described by action (e.g. "prepare the
// network for upgrade"). When yes is true it proceeds silently. Otherwise it
// requires an interactive "y"/"yes"; on a non-TTY it refuses and directs the
// caller to pass --yes, so CI can never trip an irreversible command by omission.
func (c confirmer) confirm(yes bool, action string) error {
	if yes {
		return nil
	}
	if !c.isTTY {
		return errx.Decorate(
			errConfirm.New("refusing to %s without confirmation on a non-interactive terminal", action),
			reasonConfirmationRequired,
			"Re-run with --yes to proceed non-interactively (required in CI)",
		)
	}

	_, _ = fmt.Fprintf(c.out, "About to %s.\nThis is irreversible. Type 'yes' to continue: ", action)
	line, _ := bufio.NewReader(c.in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return errx.Decorate(
			errConfirm.New("aborted: %s", action),
			reasonAborted,
			"Nothing was submitted; re-run and confirm, or pass --yes",
		)
	}
}
