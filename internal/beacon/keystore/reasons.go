// SPDX-License-Identifier: Apache-2.0

package keystore

import "github.com/automa-saga/errx"

// Reason codes for the keystore package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see docs/beacon/design.md §10, §12 and CLAUDE.md "Error Handling").
const (
	reasonKeyReadFailed      errx.Reason = "KeyReadFailed"
	reasonPassReadFailed     errx.Reason = "PassphraseReadFailed"
	reasonKeyDecodeFailed    errx.Reason = "KeyDecodeFailed"
	reasonNoPassphraseSource errx.Reason = "NoPassphraseSource"
	reasonKeyLoaded          errx.Reason = "KeyLoaded"
)
