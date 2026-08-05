// SPDX-License-Identifier: Apache-2.0

package keystore

import "github.com/joomcode/errorx"

// keystore error taxonomy. Each error this package returns is created from one
// of these types and decorated with a reason code + remediation hints via the
// internal/errx helpers, so the base is a real errorx error rather than the
// generic "unexpected non-errorx error" fallback. Key-loading errors are all
// terminal: the operator must fix the file, passphrase, or environment and
// re-run, so none carry the errorx.Temporary() trait.
var (
	errNS = errorx.NewNamespace("beacon.keystore")

	// errKeyIO: the key PEM or its passphrase could not be read — a missing
	// file, an unreadable .pass, or no passphrase source in a non-interactive
	// context. Terminal.
	errKeyIO = errNS.NewType("io")

	// errKeyDecode: the PEM was read but could not be decoded into a signing key
	// (wrong format or wrong passphrase). Terminal.
	errKeyDecode = errNS.NewType("decode")
)
