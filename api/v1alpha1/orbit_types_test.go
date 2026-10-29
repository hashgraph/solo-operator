// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestOrbit_NodeNameZeroBased(t *testing.T) {
	cases := []struct {
		name string
		gate *ConsensusFeatureGates
		want bool
	}{
		{"featureGates unset", nil, false},
		{"gate default", &ConsensusFeatureGates{}, false},
		{"gate true", &ConsensusFeatureGates{NodeNameZeroBased: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := &Orbit{}
			o.Spec.Consensus = &OrbitConsensus{FeatureGates: c.gate}
			if got := o.IsNodeNameZeroBased(); got != c.want {
				t.Errorf("IsNodeNameZeroBased() = %v, want %v", got, c.want)
			}
		})
	}

	t.Run("consensus nil", func(t *testing.T) {
		o := &Orbit{} // consensus-less (block-node-only) Orbit
		if got := o.IsNodeNameZeroBased(); got {
			t.Errorf("IsNodeNameZeroBased() = %v, want false for a consensus-less Orbit", got)
		}
	})
}
