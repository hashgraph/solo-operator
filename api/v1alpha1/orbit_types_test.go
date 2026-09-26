// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestOrbit_NodeNameZeroBased(t *testing.T) {
	o := &Orbit{}
	if o.IsNodeNameZeroBased() {
		t.Error("default should be false when featureGates unset")
	}

	o.Spec.Consensus.FeatureGates = &ConsensusFeatureGates{NodeNameZeroBased: true}
	if !o.IsNodeNameZeroBased() {
		t.Error("expected true when gate set")
	}
}
