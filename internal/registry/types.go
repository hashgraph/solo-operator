// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

type RegistryResult struct {
	Image               string
	Digest              operatorv1alpha1.Sha256Digest
	Reachable           bool
	LayerHashesVerified bool
	// recorded, never propagated, so one failure can't abort sibling checks
	Err error
}

type ImageChecker interface {
	Check(ctx context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, deterministic operatorv1alpha1.UpgradeDeterministicSpec) RegistryResult
}
