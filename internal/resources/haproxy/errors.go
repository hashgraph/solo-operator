// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("haproxy")

	errTemplate = errNS.NewType("template_failed")
	errK8sAPI   = errNS.NewType("k8s_api_failed", errorx.Temporary())
)

const (
	ReasonTemplateFailed  errx.Reason = "TemplateExecutionFailed"
	ReasonSourceGetFailed errx.Reason = "SourceGetFailed"
	ReasonSourceMissing   errx.Reason = "SourceMissing"
)
