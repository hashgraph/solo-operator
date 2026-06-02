// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"math/rand/v2"
	"strings"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	StrategyRandom     = "Random"
	StrategySequential = "Sequential"
)

// IterationStrategy decides the order a component's registries are checked in.
type IterationStrategy interface {
	Name() string
	// Order returns a new slice; the input is never mutated.
	Order(in []operatorv1alpha1.UpgradeRegistrySpec) []operatorv1alpha1.UpgradeRegistrySpec
}

type SequentialStrategy struct{}

func (SequentialStrategy) Name() string { return StrategySequential }

func (SequentialStrategy) Order(in []operatorv1alpha1.UpgradeRegistrySpec) []operatorv1alpha1.UpgradeRegistrySpec {
	out := make([]operatorv1alpha1.UpgradeRegistrySpec, len(in))
	copy(out, in)
	return out
}

// RandomStrategy spreads pull traffic across registries. Rand must be non-nil.
type RandomStrategy struct {
	Rand *rand.Rand
}

func (RandomStrategy) Name() string { return StrategyRandom }

func (s RandomStrategy) Order(in []operatorv1alpha1.UpgradeRegistrySpec) []operatorv1alpha1.UpgradeRegistrySpec {
	out := make([]operatorv1alpha1.UpgradeRegistrySpec, len(in))
	copy(out, in)
	s.Rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// NewStrategy maps a flag value to a strategy; unset or unknown → Random (case-insensitive).
func NewStrategy(name string, rng *rand.Rand) IterationStrategy {
	if strings.EqualFold(name, StrategySequential) {
		return SequentialStrategy{}
	}
	return RandomStrategy{Rand: rng}
}
