// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// The first four reason codes reuse the vocabulary the HelmCapsule controller
// surfaces on /status, so recordFailure can classify via errx.ReasonOf.
var (
	errNS = errorx.NewNamespace("helm")

	errChartFetch          = errNS.NewType("chart_fetch_failed")
	errValuesBuild         = errNS.NewType("values_build_failed")
	errReleaseNameConflict = errNS.NewType("release_name_conflict")
	errHelm                = errNS.NewType("helm_failed")
	// Transient: worth a retry.
	errK8sAPI = errNS.NewType("k8s_api_failed", errorx.Temporary())
)

const (
	ReasonChartFetch          errx.Reason = "ChartFetchFailed"
	ReasonValuesBuild         errx.Reason = "ValuesBuildFailed"
	ReasonReleaseNameConflict errx.Reason = "ReleaseNameConflict"
	ReasonHelm                errx.Reason = "HelmFailed"

	ReasonUnsupportedKind errx.Reason = "UnsupportedValuesKind"
	ReasonKeyNotFound     errx.Reason = "ValuesKeyNotFound"
	ReasonOrbitNotFound   errx.Reason = "OrbitNotFound"
	ReasonCredentials     errx.Reason = "ChartCredentialsInvalid"
	ReasonListFailed      errx.Reason = "ListFailed"
	ReasonGetFailed       errx.Reason = "GetFailed"
)
