// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("haproxy")

	errTemplate = errNS.NewType("template_failed")
)

const (
	ReasonTemplateFailed errx.Reason = "TemplateExecutionFailed"
)
