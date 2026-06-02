// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const maxConcurrentChecks = 4

// CheckAll checks all of a component's registries in rng-randomised order with
// no early exit, each under its own timeout, and returns one result per
// registry in that order. checker and rng must be non-nil.
func CheckAll(
	ctx context.Context,
	checker ImageChecker,
	comp operatorv1alpha1.UpgradeComponentSpec,
	timeout time.Duration,
	rng *rand.Rand,
) []RegistryResult {
	registries := make([]operatorv1alpha1.UpgradeRegistrySpec, len(comp.Registries))
	copy(registries, comp.Registries)
	rng.Shuffle(len(registries), func(i, j int) {
		registries[i], registries[j] = registries[j], registries[i]
	})

	// each goroutine writes only its own index — no mutex needed
	results := make([]RegistryResult, len(registries))

	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup

	for i := range registries {
		wg.Add(1)
		go func(idx int, spec operatorv1alpha1.UpgradeRegistrySpec) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			results[idx] = checker.Check(cctx, spec, comp.Deterministic)
		}(i, registries[i])
	}

	wg.Wait()
	return results
}
