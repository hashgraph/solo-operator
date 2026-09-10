// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"math/rand/v2"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// componentStrategy resolves the iteration strategy for a component, applying the
// precedence: the component's own SelectionStrategy field, then the operator-wide
// fallback (the --registry-order flag value), then Random. This is the single
// place strategy precedence is decided, shared by initial selection and failover.
func componentStrategy(comp operatorv1alpha1.ImageComponentSpec, fallback string, rng *rand.Rand) IterationStrategy {
	name := comp.SelectionStrategy
	if name == "" {
		name = fallback
	}
	return NewStrategy(name, rng)
}

// SelectComponent probes all of a component's registries (in the component's
// strategy order) and picks the winner per that strategy: Sequential selects the
// first valid registry in the declared order (deterministic preference), Random
// picks a valid one at random. It is the shared core of image selection: the same
// routine backs first-time capsule deployment and the network-upgrade Prepare
// phase, so install-time and upgrade-time select and verify identically.
//
// It returns every registry's result (so callers can emit their own audit
// events/metrics over the full set), the selected winner, and whether any valid
// registry was found. It has no side effects of its own.
func SelectComponent(
	ctx context.Context,
	checker ImageChecker,
	comp operatorv1alpha1.ImageComponentSpec,
	timeout time.Duration,
	rng *rand.Rand,
	fallbackStrategy string,
) (results []RegistryResult, winner RegistryResult, ok bool) {
	strategy := componentStrategy(comp, fallbackStrategy, rng)
	results = CheckAll(ctx, checker, comp, timeout, strategy)

	// The strategy governs the winner, not just the probe order: Sequential picks the
	// first valid registry in the declared order (deterministic preference); Random
	// spreads across the valid ones. CheckAll already ordered results per the strategy.
	if strategy.Name() == StrategySequential {
		winner, ok = SelectFirstValid(results)
	} else {
		winner, ok = SelectRandomValid(results, rng)
	}
	return results, winner, ok
}

// SelectedSourceFromWinner maps a winning RegistryResult to the SelectedSource
// recorded in status, carrying the component's per-registry layer hashes so
// later revalidation need not re-scan comp.Registries. Shared by initial
// selection and failover so both persist the same shape.
func SelectedSourceFromWinner(comp operatorv1alpha1.ImageComponentSpec, winner RegistryResult) operatorv1alpha1.SelectedSource {
	return operatorv1alpha1.SelectedSource{
		Image:               winner.Image,
		Digest:              winner.Digest,
		LayerHashesVerified: winner.LayerHashesVerified,
		LayerHashes:         SpecForImage(comp, winner.Image).LayerHashes,
	}
}
