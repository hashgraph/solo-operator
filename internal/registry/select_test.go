// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"math/rand/v2"
	"testing"
)

func rng() *rand.Rand { return rand.New(rand.NewPCG(42, 0)) }

func TestSelectValid_NoneValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "a", State: StateUnreachable},
		{Image: "b", State: StateHashMismatch},
	}
	if _, ok := SelectValid(results, rng()); ok {
		t.Error("expected false when no valid registries")
	}
}

func TestSelectValid_Empty(t *testing.T) {
	if _, ok := SelectValid(nil, rng()); ok {
		t.Error("expected false for empty input")
	}
}

func TestSelectValid_SingleValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "a", State: StateUnreachable},
		{Image: "b", State: StateValid, Digest: "sha256:aaaa"},
	}
	got, ok := SelectValid(results, rng())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Image != "b" {
		t.Errorf("expected image b, got %s", got.Image)
	}
}

func TestSelectValid_OnlyPicksFromValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "valid1", State: StateValid},
		{Image: "bad", State: StateUnreachable},
		{Image: "valid2", State: StateValid},
	}
	r := rng()
	seen := map[string]int{}
	for i := 0; i < 200; i++ {
		got, ok := SelectValid(results, r)
		if !ok {
			t.Fatal("expected ok=true")
		}
		seen[got.Image]++
	}
	if _, hasBad := seen["bad"]; hasBad {
		t.Error("selected an invalid registry")
	}
	if seen["valid1"] == 0 || seen["valid2"] == 0 {
		t.Errorf("selection not uniform across valid set: %v", seen)
	}
}
