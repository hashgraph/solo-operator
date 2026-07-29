// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

// All errors here are terminal — the operator must fix the offending content.
var (
	errNS = errorx.NewNamespace("common")

	errValidation  = errNS.NewType("validation_failed")
	errCertInvalid = errNS.NewType("cert_invalid")
)

const (
	ReasonContentEmpty errx.Reason = "ContentEmpty"
	ReasonNotKeyValue  errx.Reason = "NotKeyValueFormat"
	ReasonInvalidJSON  errx.Reason = "InvalidJSON"
	ReasonNoPEMBlock   errx.Reason = "NoPEMBlock"
	ReasonInvalidCert  errx.Reason = "InvalidCertificate"
)
