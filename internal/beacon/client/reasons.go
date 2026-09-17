// SPDX-License-Identifier: Apache-2.0

package client

import "github.com/automa-saga/errx"

// Reason codes for the client package. They are PascalCase and mirror the logx
// reason= field so an error, its log line, and any status output share one
// identifier (see docs/beacon/design.md §5 and CLAUDE.md "Error Handling").
const (
	reasonNoNetwork       errx.Reason = "NoNetwork"
	reasonInvalidPayer    errx.Reason = "InvalidPayer"
	reasonClientInit      errx.Reason = "ClientInitFailed"
	reasonSubmitFailed    errx.Reason = "SubmitFailed"
	reasonReceiptFailed   errx.Reason = "ReceiptFailed"
	reasonTxStatusFailed  errx.Reason = "TransactionFailed"
	reasonTxSubmitted     errx.Reason = "TransactionSubmitted"
	reasonNodeUnreachable errx.Reason = "NodeUnreachable"
	reasonNodeReachable   errx.Reason = "NodeReachable"
	reasonSDKPanic        errx.Reason = "SDKPanic"
)
