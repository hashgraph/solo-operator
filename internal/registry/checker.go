// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// OCIChecker is a stub: it reports every registry reachable with a digest
// derived from the image. TODO(a38_5): resolve real digests and verify layer
// hashes (stories 5.2/5.3).
type OCIChecker struct{}

func (OCIChecker) Check(ctx context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, _ operatorv1alpha1.UpgradeDeterministicSpec) RegistryResult {
	if err := ctx.Err(); err != nil {
		return RegistryResult{Image: spec.Image, Reachable: false, Err: err}
	}

	sum := sha256.Sum256([]byte(spec.Image))
	return RegistryResult{
		Image:     spec.Image,
		Digest:    operatorv1alpha1.Sha256Digest("sha256:" + hex.EncodeToString(sum[:])),
		Reachable: true,
	}
}
