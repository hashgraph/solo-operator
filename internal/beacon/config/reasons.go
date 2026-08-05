// SPDX-License-Identifier: Apache-2.0

package config

import "github.com/automa-saga/errx"

// Reason codes for the config package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see docs/beacon/design.md §5.2 and CLAUDE.md "Error Handling").
const (
	reasonConfigRead           errx.Reason = "ConfigReadFailed"
	reasonConfigParse          errx.Reason = "ConfigInvalid"
	reasonUnknownNode          errx.Reason = "UnknownNode"
	reasonNoNodes              errx.Reason = "NoNodes"
	reasonNodeIPWithoutAccount errx.Reason = "NodeIPWithoutAccount"
	reasonUnknownPayer         errx.Reason = "UnknownPayer"
	reasonInvalidAccountID     errx.Reason = "InvalidAccountID"
)
