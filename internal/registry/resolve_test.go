// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestComponentStrategy_Precedence(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		fallback string
		want     string
	}{
		{"field wins over flag", StrategySequential, StrategyRandom, StrategySequential},
		{"field wins (random) over sequential flag", StrategyRandom, StrategySequential, StrategyRandom},
		{"empty field falls back to flag", "", StrategySequential, StrategySequential},
		{"empty field and empty flag default to random", "", "", StrategyRandom},
		{"unknown field falls to random via NewStrategy", "bogus", StrategySequential, StrategyRandom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := operatorv1alpha1.ImageComponentSpec{SelectionStrategy: tt.field}
			got := componentStrategy(comp, tt.fallback, newRNG(1)).Name()
			if got != tt.want {
				t.Errorf("componentStrategy(field=%q, fallback=%q) = %q, want %q", tt.field, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestSelectComponent_SequentialHonorsOrder(t *testing.T) {
	checker := &fakeChecker{}
	comp := compWithRegistries("img-0", "img-1", "img-2")
	comp.SelectionStrategy = StrategySequential

	results, _, ok := SelectComponent(context.Background(), checker, comp, time.Second, newRNG(42), StrategyRandom)
	if !ok {
		t.Fatal("expected a valid winner")
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Registries are probed concurrently, so completion order is nondeterministic;
	// the strategy instead fixes the order of the results slice. Sequential keeps
	// the listed order regardless of the (Random) fallback.
	want := []string{"img-0", "img-1", "img-2"}
	for i, img := range want {
		if results[i].Image != img {
			t.Errorf("sequential order not honored: results[%d]=%q, want %q", i, results[i].Image, img)
		}
	}
}

func TestSelectComponent_SequentialPicksFirstAmongMultipleValid(t *testing.T) {
	// Both registries verify; Sequential must always pick the first-declared one,
	// regardless of rng seed. (Regression: SelectRandomValid ignored declared order.)
	allValid := func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		return RegistryResult{Image: spec.Image, State: StateValid, Digest: testDigest, LayerHashesVerified: true}
	}
	for seed := uint64(1); seed <= 20; seed++ {
		comp := compWithRegistries("img-primary", "img-mirror")
		comp.SelectionStrategy = StrategySequential
		_, winner, ok := SelectComponent(context.Background(), &fakeChecker{fn: allValid}, comp, time.Second, newRNG(seed), StrategyRandom)
		if !ok || winner.Image != "img-primary" {
			t.Fatalf("seed %d: Sequential winner = %q (ok=%v), want img-primary", seed, winner.Image, ok)
		}
	}
}

func TestSelectComponent_RandomSpreadsAmongValid(t *testing.T) {
	// With Random, both valid registries should win across seeds (not pinned to one).
	allValid := func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		return RegistryResult{Image: spec.Image, State: StateValid, Digest: testDigest, LayerHashesVerified: true}
	}
	seen := map[string]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		comp := compWithRegistries("img-a", "img-b")
		comp.SelectionStrategy = StrategyRandom
		_, winner, ok := SelectComponent(context.Background(), &fakeChecker{fn: allValid}, comp, time.Second, newRNG(seed), StrategySequential)
		if !ok {
			t.Fatalf("seed %d: expected a winner", seed)
		}
		seen[winner.Image]++
	}
	if seen["img-a"] == 0 || seen["img-b"] == 0 {
		t.Errorf("Random did not spread across valid registries: %v", seen)
	}
}

func TestSelectComponent_NoValidRegistry(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			return RegistryResult{Image: spec.Image, State: StateUnreachable}
		},
	}
	comp := compWithRegistries("img-a", "img-b")

	results, _, ok := SelectComponent(context.Background(), checker, comp, time.Second, newRNG(1), "")
	if ok {
		t.Fatal("expected ok=false when no registry is valid")
	}
	if len(results) != 2 {
		t.Fatalf("expected all registries probed, got %d results", len(results))
	}
}

func TestSelectComponent_PicksValidWinner(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			if spec.Image == "img-good" {
				return RegistryResult{Image: spec.Image, State: StateValid, Digest: testDigest, LayerHashesVerified: true}
			}
			return RegistryResult{Image: spec.Image, State: StateUnreachable}
		},
	}
	comp := compWithRegistries("img-bad", "img-good")

	_, winner, ok := SelectComponent(context.Background(), checker, comp, time.Second, newRNG(7), "")
	if !ok {
		t.Fatal("expected a valid winner")
	}
	if winner.Image != "img-good" {
		t.Errorf("winner = %q, want img-good", winner.Image)
	}
}

func TestSelectedSourceFromWinner_CarriesLayerHashes(t *testing.T) {
	hashes := map[string][]operatorv1alpha1.Sha256Digest{PlatformAMD64: {testDigest}}
	comp := compNonDeterministicWithHashes("img-a", hashes)
	winner := RegistryResult{Image: "img-a", Digest: altDigest, LayerHashesVerified: true}

	src := SelectedSourceFromWinner(comp, winner)
	if src.Image != "img-a" || src.Digest != altDigest || !src.LayerHashesVerified {
		t.Fatalf("unexpected selected source: %+v", src)
	}
	if got := src.LayerHashes[PlatformAMD64]; len(got) != 1 || got[0] != testDigest {
		t.Errorf("layer hashes not carried from comp.Registries: %+v", src.LayerHashes)
	}
}

func TestSelectedSourceFromWinner_CarriesVerificationSkipped(t *testing.T) {
	comp := compNonDeterministicWithHashes("img-a", nil)
	winner := RegistryResult{Image: "img-a", VerificationSkipped: true}

	src := SelectedSourceFromWinner(comp, winner)
	if !src.VerificationSkipped || src.Digest != "" {
		t.Fatalf("unexpected selected source: %+v", src)
	}
}
