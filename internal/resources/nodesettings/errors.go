// SPDX-License-Identifier: Apache-2.0

package nodesettings

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("nodesettings")

	errValidation = errNS.NewType("validation_failed")
)

const (
	ReasonContentEmpty errx.Reason = "ContentEmpty"
	ReasonNotKeyValue  errx.Reason = "NotKeyValueFormat"
)
