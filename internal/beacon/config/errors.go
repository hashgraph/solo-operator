// SPDX-License-Identifier: Apache-2.0

package config

import "github.com/joomcode/errorx"

// config error taxonomy. Each error this package returns is created from one of
// these types and decorated with a reason code + remediation hints via the
// internal/errx helpers, so the base error is a real errorx error (never the
// generic "unexpected non-errorx error" fallback). Config errors are all
// terminal: re-reading the same file or re-parsing the same flags cannot fix a
// bad config, so none carry the errorx.Temporary() trait.
var (
	errNS = errorx.NewNamespace("beacon.config")

	// errConfigIO: config.yaml could not be read from disk. Terminal — the
	// operator must fix the path/permissions and re-run.
	errConfigIO = errNS.NewType("io")

	// errConfigInvalid: config.yaml is malformed, empty of nodes, or the
	// requested payer/node cannot be resolved against it. Terminal.
	errConfigInvalid = errNS.NewType("invalid")
)
