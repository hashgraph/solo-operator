// SPDX-License-Identifier: Apache-2.0

// Package registry probes OCI registries to validate component images used by
// the NetworkUpgrade* reconcilers. Real validation runs over the OCI v2 HTTP
// distribution API; a fake implementation is provided for tests.
package registry

import (
	"context"
	"errors"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ErrUnreachable is returned by Validator when a registry cannot be reached or
// returns a non-2xx response on manifest fetch. Layer-hash mismatches are NOT
// errors: the validator reports them via the layerHashesVerified return value.
var ErrUnreachable = errors.New("registry unreachable")

// Validator probes a single registry for one component image, returning the
// resolved digest and whether expected layer hashes (if any were configured)
// matched what the registry served.
type Validator interface {
	// Validate fetches the image manifest for the configured target platform
	// and compares observed layer digests against the expected list.
	//
	// Returns:
	//  - digest: the manifest digest (e.g. "sha256:..."); empty on error.
	//  - layerHashesVerified: true when expected hashes were configured and
	//    matched what the registry served. False when no hashes were configured
	//    OR when there was a mismatch.
	//  - err: non-nil only on unreachable / transport / auth errors. A hash
	//    mismatch is not an error.
	Validate(
		ctx context.Context,
		comp *operatorv1alpha1.UpgradeComponentSpec,
		reg *operatorv1alpha1.UpgradeRegistrySpec,
	) (digest string, layerHashesVerified bool, err error)
}

// ExpectedLayerHashes returns the platform-keyed layer hash list that should
// be verified for the given component+registry pair. For deterministic builds
// the hashes live on UpgradeComponentSpec.Deterministic; for non-deterministic
// builds (e.g. backup uploader) they may live per-registry. Returns nil when
// no hashes are configured for the platform.
func ExpectedLayerHashes(comp *operatorv1alpha1.UpgradeComponentSpec, reg *operatorv1alpha1.UpgradeRegistrySpec, platform string) []string {
	if comp != nil && comp.Deterministic.Supported {
		if h, ok := comp.Deterministic.LayerHashes[platform]; ok {
			return h
		}
		return nil
	}
	if reg != nil {
		if h, ok := reg.LayerHashes[platform]; ok {
			return h
		}
	}
	return nil
}
