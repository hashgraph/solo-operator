// SPDX-License-Identifier: Apache-2.0

package registry

import "math/rand/v2"

func SelectValid(results []RegistryResult, rng *rand.Rand) (RegistryResult, bool) {
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
