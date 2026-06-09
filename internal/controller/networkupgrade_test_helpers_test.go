// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// fakeUpgradeChecker is a shared test double for all upgrade reconciler tests.
// Valid results carry digest; if empty, defaultDigest is used.
type fakeUpgradeChecker struct {
	unreachable   map[string]bool
	mismatch      map[string]bool
	digest        operatorv1alpha1.Sha256Digest
	defaultDigest operatorv1alpha1.Sha256Digest
}

func (c fakeUpgradeChecker) Check(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, _ operatorv1alpha1.UpgradeDeterministicSpec) registry.RegistryResult {
	d := c.digest
	if d == "" {
		d = c.defaultDigest
	}
	switch {
	case c.unreachable[spec.Image]:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable}
	case c.mismatch[spec.Image]:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateHashMismatch}
	default:
		return registry.RegistryResult{Image: spec.Image, State: registry.StateValid, Digest: d, LayerHashesVerified: true}
	}
}
