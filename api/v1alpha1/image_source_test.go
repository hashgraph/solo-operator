// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"reflect"
	"testing"
)

func TestImageRepository_Ref(t *testing.T) {
	tests := []struct {
		name string
		repo ImageRepository
		want string
	}{
		{"full", ImageRepository{Repository: "ghcr.io/hashgraph", ImageName: "consensus-node", ImageTag: "0.75.0"}, "ghcr.io/hashgraph/consensus-node:0.75.0"},
		{"no repository", ImageRepository{ImageName: "consensus-node", ImageTag: "0.75.0"}, "consensus-node:0.75.0"},
		{"no tag", ImageRepository{Repository: "ghcr.io/hashgraph", ImageName: "consensus-node"}, "ghcr.io/hashgraph/consensus-node"},
		{"name only", ImageRepository{ImageName: "consensus-node"}, "consensus-node"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.Ref(); got != tt.want {
				t.Errorf("Ref() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToImageComponentSpec(t *testing.T) {
	src := &SoftwareVersionSource{
		ImageRepositories: []ImageRepository{
			{Repository: "ghcr.io/hashgraph", ImageName: "consensus-node", ImageTag: "0.75.0"},
			{Repository: "registry.example.com/mirror", ImageName: "consensus-node", ImageTag: "0.75.0"},
		},
		ImageVerificationSpec: []ImageVerificationSpec{
			{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:aaa", "sha256:bbb"}},
			{OS: "linux", Architecture: "arm64", LayerHashes: []string{"sha256:ccc"}},
		},
		SelectionStrategy: "Sequential",
	}

	comp := src.ToImageComponentSpec()

	if !comp.Deterministic.Supported {
		t.Error("expected Deterministic.Supported = true")
	}
	if comp.SelectionStrategy != "Sequential" {
		t.Errorf("SelectionStrategy = %q, want Sequential", comp.SelectionStrategy)
	}
	wantRegs := []ImageRegistrySpec{
		{Image: "ghcr.io/hashgraph/consensus-node:0.75.0"},
		{Image: "registry.example.com/mirror/consensus-node:0.75.0"},
	}
	if !reflect.DeepEqual(comp.Registries, wantRegs) {
		t.Errorf("Registries = %+v, want %+v", comp.Registries, wantRegs)
	}
	wantHashes := map[string][]Sha256Digest{
		"linux/amd64": {"sha256:aaa", "sha256:bbb"},
		"linux/arm64": {"sha256:ccc"},
	}
	if !reflect.DeepEqual(comp.Deterministic.LayerHashes, wantHashes) {
		t.Errorf("LayerHashes = %+v, want %+v", comp.Deterministic.LayerHashes, wantHashes)
	}
}

func TestToImageComponentSpec_SkipsEntriesMissingPlatform(t *testing.T) {
	src := &SoftwareVersionSource{
		ImageRepositories: []ImageRepository{{Repository: "r", ImageName: "n", ImageTag: "t"}},
		ImageVerificationSpec: []ImageVerificationSpec{
			{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:aaa"}},
			{OS: "linux", LayerHashes: []string{"sha256:bbb"}},         // missing arch → skipped
			{Architecture: "arm64", LayerHashes: []string{"sha256:c"}}, // missing os → skipped
		},
	}
	comp := src.ToImageComponentSpec()
	if len(comp.Deterministic.LayerHashes) != 1 {
		t.Fatalf("expected 1 usable platform, got %d: %+v", len(comp.Deterministic.LayerHashes), comp.Deterministic.LayerHashes)
	}
	if _, ok := comp.Deterministic.LayerHashes["linux/amd64"]; !ok {
		t.Error("expected linux/amd64 to be present")
	}
}
