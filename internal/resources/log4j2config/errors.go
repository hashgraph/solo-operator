// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("log4j2config")

	errValidation = errNS.NewType("validation_failed")
)

const (
	ReasonMalformedXML     errx.Reason = "MalformedXML"
	ReasonWrongRootElement errx.Reason = "WrongRootElement"
	ReasonWrongNamespace   errx.Reason = "WrongNamespace"
)
