// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func boolPtr(b bool) *bool { return &b }

func TestOrbit_IsProvisionerDaemonEnabled(t *testing.T) {
	cases := []struct {
		name      string
		topLevel  *bool
		consensus bool
		want      bool
	}{
		{"unset both", nil, false, false},
		{"fallback to consensus true", nil, true, true},
		{"top-level true wins", boolPtr(true), false, true},
		{"top-level false wins over consensus true", boolPtr(false), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := &Orbit{Spec: OrbitSpec{
				ProvisionerDaemonEnabled: c.topLevel,
				Consensus:                OrbitConsensus{ProvisionerDaemonEnabled: c.consensus},
			}}
			if got := o.IsProvisionerDaemonEnabled(); got != c.want {
				t.Errorf("IsProvisionerDaemonEnabled() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestOrbit_NodeNameZeroBased(t *testing.T) {
	o := &Orbit{}
	if o.NodeNameZeroBased() {
		t.Error("default should be false when featureGates unset")
	}

	o.Spec.Consensus.FeatureGates = &ConsensusFeatureGates{NodeNameZeroBased: true}
	if !o.NodeNameZeroBased() {
		t.Error("expected true when gate set")
	}
}
