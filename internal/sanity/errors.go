// SPDX-License-Identifier: Apache-2.0

package sanity

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("sanity")

	errInvalidOperationID = errNS.NewType("invalid_operation_id")
)

const (
	ReasonOperationIDEmpty      errx.Reason = "OperationIDEmpty"
	ReasonOperationIDTraversal  errx.Reason = "OperationIDPathTraversal"
	ReasonOperationIDShellChars errx.Reason = "OperationIDShellChars"
	ReasonOperationIDInvalid    errx.Reason = "OperationIDInvalidChars"
)
