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
	errACLSource     = errNS.NewType("acl_source_unavailable", errorx.Temporary())
	errACLInvalid    = errNS.NewType("acl_invalid")
)

const (
	ReasonCORSValueInvalid errx.Reason = "CORSValueInvalid"
	ReasonTemplateFailed   errx.Reason = "TemplateExecutionFailed"
	ReasonConfigTooLarge   errx.Reason = "ConfigTooLarge"
	ReasonACLSourceMissing errx.Reason = "ACLSourceMissing"
	ReasonACLGetFailed     errx.Reason = "ACLGetFailed"
	ReasonACLEntryInvalid  errx.Reason = "ACLEntryInvalid"
)
