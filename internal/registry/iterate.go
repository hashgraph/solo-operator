// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"sync"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const maxConcurrentChecks = 4

// CheckAll checks all of a component's registries in strategy order with no
// early exit, each under its own timeout. checker and strategy must be non-nil.
func CheckAll(
	ctx context.Context,
	checker ImageChecker,
	comp operatorv1alpha1.ImageComponentSpec,
	timeout time.Duration,
	strategy IterationStrategy,
) []RegistryResult {
	registries := strategy.Order(comp.Registries)

	// each goroutine writes only its own index — no mutex needed
	results := make([]RegistryResult, len(registries))

	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup

	for i := range registries {
		wg.Add(1)
		go func(idx int, spec operatorv1alpha1.ImageRegistrySpec) {
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
