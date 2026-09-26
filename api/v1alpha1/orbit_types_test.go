// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestOrbit_NodeNameZeroBased(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	cases := []struct {
		name string
		gate *ConsensusFeatureGates
		want bool
	}{
		{"featureGates unset", nil, false},
		{"gate unset (nil override)", &ConsensusFeatureGates{}, false},
		{"explicit false", &ConsensusFeatureGates{NodeNameZeroBased: boolPtr(false)}, false},
		{"explicit true", &ConsensusFeatureGates{NodeNameZeroBased: boolPtr(true)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := &Orbit{}
			o.Spec.Consensus.FeatureGates = c.gate
			if got := o.IsNodeNameZeroBased(); got != c.want {
				t.Errorf("IsNodeNameZeroBased() = %v, want %v", got, c.want)
			}
		})
	}
}
