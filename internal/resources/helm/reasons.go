// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"github.com/automa-saga/errx"
)

// Reason codes for logx log events emitted by the helm subreconciler.
// Error-decoration reason codes (ReasonChartFetch, ReasonValuesBuild, etc.)
// live alongside their typed errors in errors.go.
const (
	ReasonReleaseUpToDate    errx.Reason = "ReleaseUpToDate"
	ReasonReleaseRetry       errx.Reason = "ReleaseRetry"
	ReasonReleaseReconciling errx.Reason = "ReleaseReconciling"
	ReasonReleaseReconciled  errx.Reason = "ReleaseReconciled"
	ReasonReleasePending     errx.Reason = "ReleasePending"
	ReasonReleaseUnlocked    errx.Reason = "ReleaseUnlocked"

	ReasonAddressBookReadFailed errx.Reason = "AddressBookReadFailed"
	ReasonCapsuleSkipped        errx.Reason = "CapsuleSkipped"
)
