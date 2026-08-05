// SPDX-License-Identifier: Apache-2.0

package keystore

import (
	"fmt"
	"os"

	"github.com/automa-saga/errx"
	"golang.org/x/term"
)

// promptTerminal reads a passphrase from the controlling terminal without
// echoing it (design §12: passphrase prompts must not echo). It refuses to run
// when stdin is not a terminal, so a passphrase is never silently consumed from
// a pipe or left echoing in a non-interactive context — the caller must supply a
// .pass file instead.
func promptTerminal(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errx.Decorate(
			errKeyIO.New("no .pass file and stdin is not a terminal"),
			reasonNoPassphraseSource,
			"Provide a sibling account<N>.pass file, or run beacon interactively so it can prompt for the passphrase",
		)
	}

	// Prompt on stderr so stdout stays clean for machine-readable output.
	fmt.Fprint(os.Stderr, prompt)
	pass, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errx.Decorate(
			errKeyIO.Wrap(err, "read passphrase"),
			reasonPassReadFailed,
			"Retry and type the passphrase, or provide a sibling account<N>.pass file",
		)
	}
	return string(pass), nil
}
