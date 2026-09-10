// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"math/rand/v2"
	"testing"
)

func rng() *rand.Rand { return rand.New(rand.NewPCG(42, 0)) }

func TestSelectRandomValid_NoneValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "a", State: StateUnreachable},
		{Image: "b", State: StateHashMismatch},
	}
	if _, ok := SelectRandomValid(results, rng()); ok {
		t.Error("expected false when no valid registries")
	}
}

func TestSelectRandomValid_Empty(t *testing.T) {
	if _, ok := SelectRandomValid(nil, rng()); ok {
		t.Error("expected false for empty input")
	}
}

func TestSelectRandomValid_SingleValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "a", State: StateUnreachable},
		{Image: "b", State: StateValid, Digest: "sha256:aaaa"},
	}
	got, ok := SelectRandomValid(results, rng())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Image != "b" {
		t.Errorf("expected image b, got %s", got.Image)
	}
}

func TestSelectRandomValid_OnlyPicksFromValid(t *testing.T) {
	results := []RegistryResult{
		{Image: "valid1", State: StateValid},
		{Image: "bad", State: StateUnreachable},
		{Image: "valid2", State: StateValid},
	}
	r := rng()
	seen := map[string]int{}
	for i := 0; i < 200; i++ {
		got, ok := SelectRandomValid(results, r)
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

func TestSelectFirstValid(t *testing.T) {
	t.Run("first valid in order wins", func(t *testing.T) {
		results := []RegistryResult{
			{Image: "bad", State: StateUnreachable},
			{Image: "valid1", State: StateValid},
			{Image: "valid2", State: StateValid},
		}
		got, ok := SelectFirstValid(results)
		if !ok || got.Image != "valid1" {
			t.Fatalf("got (%q, %v), want (valid1, true)", got.Image, ok)
		}
	})
	t.Run("none valid", func(t *testing.T) {
		results := []RegistryResult{
			{Image: "a", State: StateUnreachable},
			{Image: "b", State: StateHashMismatch},
		}
		if _, ok := SelectFirstValid(results); ok {
			t.Error("expected ok=false when no valid registries")
		}
	})
	t.Run("empty", func(t *testing.T) {
		if _, ok := SelectFirstValid(nil); ok {
			t.Error("expected ok=false for empty input")
		}
	})
}
