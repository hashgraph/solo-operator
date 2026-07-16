// SPDX-License-Identifier: Apache-2.0

package log4j2config

import (
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
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
