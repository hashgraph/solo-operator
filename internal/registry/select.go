// SPDX-License-Identifier: Apache-2.0

package registry

import "math/rand/v2"

// SelectFirstValid returns the first StateValid result in slice order. Used by the
// Sequential strategy: CheckAll returns results in the registries' declared order, so
// the first configured registry that verifies wins — a deterministic preference order
// (e.g. list a primary registry first and it is always chosen when reachable).
func SelectFirstValid(results []RegistryResult) (RegistryResult, bool) {
	for _, r := range results {
		if r.State == StateValid {
			return r, true
		}
	}
	return RegistryResult{}, false
}

// SelectRandomValid reservoir-samples one StateValid result uniformly at random. Used by
// the Random strategy to spread pull traffic across the valid registries.
func SelectRandomValid(results []RegistryResult, rng *rand.Rand) (RegistryResult, bool) {
	var winner RegistryResult
	count := 0
	for _, r := range results {
		if r.State == StateValid {
			count++
			if rng.IntN(count) == 0 {
				winner = r
			}
		}
	}
	return winner, count > 0
}
