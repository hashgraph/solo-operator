// SPDX-License-Identifier: Apache-2.0

// Package keystore loads privileged account signing keys for beacon.
//
// Keys use yahcli's on-disk convention: a network's keys/ directory holds one
// account<N>.pem per account, with an optional sibling account<N>.pass carrying
// the PEM's passphrase. When no .pass is present the passphrase is read from an
// interactive, non-echoing terminal prompt. Both Ed25519 and ECDSA(secp256k1)
// keys are supported (auto-detected by the hiero SDK).
//
// This package handles privileged signing keys and forms a distinct trust
// boundary (design §12): it never logs key material — only the account number,
// the public key, and the algorithm, all of which are safe to disclose.
package keystore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// PassphraseFunc returns the passphrase for a key that has no sibling .pass
// file. prompt is a human-readable request line (e.g. "Passphrase for
// account2.pem: "). It is a field on Keystore so tests can inject a passphrase
// without a terminal.
type PassphraseFunc func(prompt string) (string, error)

// Keystore loads account<N>.pem[/.pass] key material from a single directory —
// the network's keys/ dir, i.e. dir(config)/keys per design §10.
type Keystore struct {
	// Dir is the directory holding account<N>.pem and optional account<N>.pass.
	Dir string
	// Prompt resolves the passphrase when no .pass sibling exists. New wires the
	// non-echoing terminal prompt; tests may override it.
	Prompt PassphraseFunc
}

// New returns a Keystore rooted at dir with the default non-echoing terminal
// passphrase prompt.
func New(dir string) *Keystore {
	return &Keystore{Dir: dir, Prompt: promptTerminal}
}

// Load reads account<N>.pem and returns its signing key. The passphrase comes
// from a sibling account<N>.pass if present, otherwise from Prompt. Key material
// is never logged; a debug line records only the account, algorithm, and public
// key.
func (k *Keystore) Load(account uint64) (hiero.PrivateKey, error) {
	pemPath := filepath.Join(k.Dir, fmt.Sprintf("account%d.pem", account))
	pemBytes, err := os.ReadFile(pemPath)
	if err != nil {
		return hiero.PrivateKey{}, errx.Decorate(
			errKeyIO.Wrap(err, "read key %q", pemPath),
			reasonKeyReadFailed,
			fmt.Sprintf("Confirm the signing key for account %d exists at %q", account, pemPath),
		)
	}

	passphrase, err := k.passphrase(account)
	if err != nil {
		return hiero.PrivateKey{}, err
	}

	key, err := hiero.PrivateKeyFromPem(pemBytes, passphrase)
	if err != nil {
		return hiero.PrivateKey{}, errx.Decorate(
			errKeyDecode.Wrap(err, "decode key %q", pemPath),
			reasonKeyDecodeFailed,
			"Confirm the key is a valid Ed25519 or ECDSA PEM and that the passphrase is correct",
		)
	}

	logx.As().Debug().
		Str("reason", reasonKeyLoaded.String()).
		Uint64("account", account).
		Str("algorithm", Algorithm(key)).
		Str("public_key", key.PublicKey().String()).
		Msg("loaded signing key")

	return key, nil
}

// passphrase resolves the passphrase for account<N>: the sibling account<N>.pass
// file if present (trailing newline trimmed), else the Prompt callback. A .pass
// present but unreadable is an error rather than a silent prompt.
func (k *Keystore) passphrase(account uint64) (string, error) {
	passPath := filepath.Join(k.Dir, fmt.Sprintf("account%d.pass", account))
	data, err := os.ReadFile(passPath)
	switch {
	case err == nil:
		return strings.TrimRight(string(data), "\r\n"), nil
	case errors.Is(err, os.ErrNotExist):
		if k.Prompt == nil {
			return "", errx.Decorate(
				errKeyIO.New("no passphrase source for account %d", account),
				reasonNoPassphraseSource,
				fmt.Sprintf("Provide %q, or run beacon interactively so it can prompt", passPath),
			)
		}
		return k.Prompt(fmt.Sprintf("Passphrase for account%d.pem: ", account))
	default:
		return "", errx.Decorate(
			errKeyIO.Wrap(err, "read passphrase %q", passPath),
			reasonPassReadFailed,
			fmt.Sprintf("Confirm %q is readable, or remove it to be prompted", passPath),
		)
	}
}

// Algorithm reports the key's signature scheme for diagnostics, derived from the
// public-key length (Ed25519 raw = 32 bytes, ECDSA secp256k1 compressed = 33).
// It reads only public material.
func Algorithm(key hiero.PrivateKey) string {
	if len(key.PublicKey().BytesRaw()) == 33 {
		return "ECDSA_secp256k1"
	}
	return "Ed25519"
}
