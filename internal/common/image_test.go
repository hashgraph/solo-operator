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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildImage(tt.sv)
			if result != tt.expected {
				t.Errorf("BuildImage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildImageOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		sv           *operatorv1alpha1.SoftwareVersion
		defaultImage string
		expected     string
	}{
		{
			name: "with software version",
			sv: &operatorv1alpha1.SoftwareVersion{
				Repository: "gcr.io/hedera-registry",
				ImageName:  "consensus-node",
				ImageTag:   "0.54.0",
			},
			defaultImage: "default:latest",
			expected:     "gcr.io/hedera-registry/consensus-node:0.54.0",
		},
		{
			name:         "nil software version",
			sv:           nil,
			defaultImage: "default:latest",
			expected:     "default:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildImageOrDefault(tt.sv, tt.defaultImage)
			if result != tt.expected {
				t.Errorf("BuildImageOrDefault() = %v, want %v", result, tt.expected)
			}
		})
	}
}
