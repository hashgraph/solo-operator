// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

type FailoverResult struct {
	Selected    operatorv1alpha1.SelectedSource
	DidFailover bool
	FailedImage string
}

func RevalidateAndFailover(
	ctx context.Context,
	checker ImageChecker,
	comp operatorv1alpha1.UpgradeComponentSpec,
	current operatorv1alpha1.SelectedSource,
	timeout time.Duration,
	rng *rand.Rand,
	strategyName string,
) (FailoverResult, error) {
	// Use stored hashes so non-deterministic components don't get StateNoHashes
	// from an empty spec and trigger a spurious failover.
	spec := specForSource(comp, current)

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res := checker.Check(cctx, spec, comp.Deterministic)
	if res.State == StateValid {
		return FailoverResult{Selected: current}, nil
	}

	strategy := NewStrategy(strategyName, rng)
	results := CheckAll(ctx, checker, comp, timeout, strategy)

	winner, ok := SelectValid(results, rng)
	if !ok {
		return FailoverResult{}, fmt.Errorf("no valid registry available for component (original %s unreachable)", current.Image)
	}

	winnerSpec := SpecForImage(comp, winner.Image)
	return FailoverResult{
		Selected: operatorv1alpha1.SelectedSource{
			Image:               winner.Image,
			Digest:              winner.Digest,
			LayerHashesVerified: winner.LayerHashesVerified,
			LayerHashes:         winnerSpec.LayerHashes,
		},
		DidFailover: true,
		FailedImage: current.Image,
	}, nil
}

// specForSource builds the spec for re-checking the current source. Prefers
// hashes stored on current; falls back to comp.Registries for older objects
// that predate the LayerHashes field.
func specForSource(comp operatorv1alpha1.UpgradeComponentSpec, current operatorv1alpha1.SelectedSource) operatorv1alpha1.UpgradeRegistrySpec {
	if len(current.LayerHashes) > 0 {
		return operatorv1alpha1.UpgradeRegistrySpec{Image: current.Image, LayerHashes: current.LayerHashes}
	}
	return SpecForImage(comp, current.Image)
}

// SpecForImage returns the matching UpgradeRegistrySpec from comp.Registries,
// or a bare spec if no match is found.
func SpecForImage(comp operatorv1alpha1.UpgradeComponentSpec, image string) operatorv1alpha1.UpgradeRegistrySpec {
	for _, r := range comp.Registries {
		if r.Image == image {
			return r
		}
	}
	return operatorv1alpha1.UpgradeRegistrySpec{Image: image}
}
