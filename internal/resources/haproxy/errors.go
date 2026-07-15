// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

var (
	errNS = errorx.NewNamespace("haproxy")

	errTemplate = errNS.NewType("template_failed")
)

const (
	ReasonTemplateFailed errx.Reason = "TemplateExecutionFailed"
)
