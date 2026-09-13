// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"slices"
	"testing"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestSequentialStrategy_PreservesOrderAndCopies(t *testing.T) {
	in := componentWith(4).Registries
	got := SequentialStrategy{}.Order(in)

	if !slices.Equal(resultImagesFromSpecs(got), []string{"img-0", "img-1", "img-2", "img-3"}) {
		t.Errorf("sequential order changed: %v", resultImagesFromSpecs(got))
	}
	// Mutating the output must not affect the input.
	got[0].Image = "mutated"
	if in[0].Image != "img-0" {
		t.Error("Order must not alias the input slice")
	}
}

func TestRandomStrategy_ShufflesAndCopies(t *testing.T) {
	in := componentWith(6).Registries
	got := RandomStrategy{Rand: newRNG(42)}.Order(in)

	if len(got) != len(in) {
		t.Fatalf("expected %d registries, got %d", len(in), len(got))
	}
	// Deterministic for a fixed seed.
	again := RandomStrategy{Rand: newRNG(42)}.Order(in)
	if !slices.Equal(resultImagesFromSpecs(got), resultImagesFromSpecs(again)) {
		t.Error("random strategy not deterministic for a fixed seed")
	}
	// Input untouched.
	if !slices.Equal(resultImagesFromSpecs(in), []string{"img-0", "img-1", "img-2", "img-3", "img-4", "img-5"}) {
		t.Error("Order must not mutate the input slice")
	}
}

func TestNewStrategy(t *testing.T) {
	cases := map[string]string{
		"Sequential": StrategySequential,
		"sequential": StrategySequential, // case-insensitive
		"Random":     StrategyRandom,
		"":           StrategyRandom, // default
		"bogus":      StrategyRandom, // unrecognised → default
	}
	for input, want := range cases {
		if got := NewStrategy(input, newRNG(1)).Name(); got != want {
			t.Errorf("NewStrategy(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCheckAll_SequentialStrategyKeepsListedOrder(t *testing.T) {
	results := CheckAll(context.Background(), &fakeChecker{}, componentWith(4), time.Second, SequentialStrategy{})
	if !slices.Equal(resultImages(results), []string{"img-0", "img-1", "img-2", "img-3"}) {
		t.Errorf("sequential CheckAll order = %v", resultImages(results))
	}
}

func resultImagesFromSpecs(specs []operatorv1alpha1.ImageRegistrySpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Image
	}
	return out
}
