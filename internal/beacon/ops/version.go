// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"fmt"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// NetworkVersion queries a running network for its active build: the Hedera
// services version and the HAPI protobuf version (HAPI NetworkGetVersionInfo,
// design §6.2). It is the post-restart confirmation that a software upgrade is
// live (§7 step 5).
func NetworkVersion(hc *hiero.Client) (hiero.NetworkVersionInfo, error) {
	info, err := hiero.NewNetworkVersionQuery().Execute(hc)
	if err != nil {
		return hiero.NetworkVersionInfo{}, errx.Decorate(
			errQuery.Wrap(err, "query network version"),
			reasonVersionQueryFailed,
			"Confirm the node is reachable and the payer can pay the query fee",
		)
	}
	return info, nil
}

// SemverString renders a HAPI SemanticVersion as canonical semver
// (major.minor.patch, with optional -pre and +build), e.g. "0.58.3" or
// "0.58.0-rc1+abcdef".
func SemverString(v hiero.SemanticVersion) string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}
