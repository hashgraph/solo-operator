// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestBuildImage(t *testing.T) {
	tests := []struct {
		name     string
		sv       operatorv1alpha1.SoftwareVersion
		expected string
	}{
		{
			name: "standard image",
			sv: operatorv1alpha1.SoftwareVersion{
				Repository: "gcr.io/hedera-registry",
				ImageName:  "consensus-node",
				ImageTag:   "0.54.0",
			},
			expected: "gcr.io/hedera-registry/consensus-node:0.54.0",
		},
		{
			name: "docker hub image",
			sv: operatorv1alpha1.SoftwareVersion{
				Repository: "docker.io/hashgraph",
				ImageName:  "hedera-node",
				ImageTag:   "latest",
			},
			expected: "docker.io/hashgraph/hedera-node:latest",
		},
		{
			name: "digest pins the reference alongside the tag",
			sv: operatorv1alpha1.SoftwareVersion{
				Repository: "gcr.io/hedera-registry",
				ImageName:  "consensus-node",
				ImageTag:   "0.54.0",
				Digest:     "sha256:abc123",
			},
			expected: "gcr.io/hedera-registry/consensus-node:0.54.0@sha256:abc123",
		},
		{
			name: "digest without repository",
			sv: operatorv1alpha1.SoftwareVersion{
				ImageName: "node",
				ImageTag:  "v1",
				Digest:    "sha256:deadbeef",
			},
			expected: "node:v1@sha256:deadbeef",
		},
		{
			name:     "without repository",
			sv:       operatorv1alpha1.SoftwareVersion{ImageName: "node", ImageTag: "v1"},
			expected: "node:v1",
		},
		{
			name:     "without tag",
			sv:       operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "node"},
			expected: "gcr.io/node",
		},
		{
			name:     "image name only",
			sv:       operatorv1alpha1.SoftwareVersion{ImageName: "node"},
			expected: "node",
		},
		{
			name:     "empty image name",
			sv:       operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageTag: "v1"},
			expected: "",
		},
		{
			name:     "all empty",
			sv:       operatorv1alpha1.SoftwareVersion{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := tt.sv
			result := BuildImage(&sv)
			if result != tt.expected {
				t.Errorf("BuildImage() = %v, want %v", result, tt.expected)
			}
		})
	}

	// The operator ships no default images, so a nil SoftwareVersion yields "".
	if got := BuildImage(nil); got != "" {
		t.Errorf("BuildImage(nil) = %q, want \"\"", got)
	}
}

func TestPinnedRef(t *testing.T) {
	tests := []struct {
		name     string
		src      operatorv1alpha1.SelectedSource
		expected string
	}{
		{
			name: "digest pins the tagged reference",
			src: operatorv1alpha1.SelectedSource{
				Image:  "gcr.io/hedera-registry/consensus-node:0.54.0",
				Digest: "sha256:abc123",
			},
			expected: "gcr.io/hedera-registry/consensus-node:0.54.0@sha256:abc123",
		},
		{
			name:     "no digest falls back to the tag",
			src:      operatorv1alpha1.SelectedSource{Image: "gcr.io/hedera-registry/consensus-node:0.54.0"},
			expected: "gcr.io/hedera-registry/consensus-node:0.54.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PinnedRef(&tt.src)
			if result != tt.expected {
				t.Errorf("PinnedRef() = %v, want %v", result, tt.expected)
			}
		})
	}
}
