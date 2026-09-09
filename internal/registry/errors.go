// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("registry")

	errIntegrity = errNS.NewType("integrity_failed")
	errImageRef  = errNS.NewType("image_ref_invalid")
	// Transient: worth a retry / failover to another registry.
	errRegistryAPI = errNS.NewType("registry_api_failed", errorx.Temporary())
	errNoRegistry  = errNS.NewType("no_valid_registry")
	// Terminal: the operator refuses to dial this host at all (SSRF guard).
	errHostDenied = errNS.NewType("registry_host_denied")
	// Transient: reading a pull secret from the API server may be retried.
	errPullSecret = errNS.NewType("pull_secret_read_failed", errorx.Temporary())
	// Terminal: a pull secret is present but its dockerconfigjson cannot be parsed.
	errPullSecretParse = errNS.NewType("pull_secret_parse_failed")
)

const (
	ReasonNoLayerHashes      errx.Reason = "NoLayerHashes"
	ReasonLayerHashMismatch  errx.Reason = "LayerHashMismatch"
	ReasonImageRefInvalid    errx.Reason = "ImageRefInvalid"
	ReasonDigestResolve      errx.Reason = "DigestResolveFailed"
	ReasonImageRead          errx.Reason = "ImageReadFailed"
	ReasonConfigRead         errx.Reason = "ConfigReadFailed"
	ReasonNoValidRegistry    errx.Reason = "NoValidRegistry"
	ReasonRegistryHostDenied errx.Reason = "RegistryHostDenied"
	ReasonPullSecretRead     errx.Reason = "PullSecretReadFailed"
	ReasonPullSecretParse    errx.Reason = "PullSecretParseFailed"
)
