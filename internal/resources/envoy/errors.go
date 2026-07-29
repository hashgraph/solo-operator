// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("envoy")

	errConfigInvalid = errNS.NewType("config_invalid")
	errTemplate      = errNS.NewType("template_failed")
)

const (
	ReasonCORSValueInvalid errx.Reason = "CORSValueInvalid"
	ReasonTemplateFailed   errx.Reason = "TemplateExecutionFailed"
)
