// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("genesis")

	// Transient: worth a retry.
	errK8sAPI     = errNS.NewType("k8s_api_failed", errorx.Temporary())
	errValidation = errNS.NewType("validation_failed")
	errMaterial   = errNS.NewType("material_failed")
)

const (
	ReasonCapsuleListFailed   errx.Reason = "CapsuleListFailed"
	ReasonSecretGetFailed     errx.Reason = "SecretGetFailed"
	ReasonSecretNameEmpty     errx.Reason = "SecretNameEmpty"
	ReasonSecretKeyMissing    errx.Reason = "SecretKeyMissing"
	ReasonSecretsMissing      errx.Reason = "SecretsMissing"
	ReasonExternalAddrInvalid errx.Reason = "ExternalAddressInvalid"
	ReasonRosterEmpty         errx.Reason = "RosterEmpty"
	ReasonAccountIDInvalid    errx.Reason = "AccountIDInvalid"
	ReasonGenesisJSONInvalid  errx.Reason = "GenesisJSONInvalid"
	ReasonCertHexInvalid      errx.Reason = "CertHexInvalid"
	ReasonGenesisJSONMarshal  errx.Reason = "GenesisJSONMarshalFailed"
	ReasonAddressBookMarshal  errx.Reason = "AddressBookMarshalFailed"
)
