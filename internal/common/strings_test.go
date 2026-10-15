// SPDX-License-Identifier: Apache-2.0

package common

import (
	"slices"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	tests := map[string][]string{
		"":           nil,
		" , ,":       nil,
		"a":          {"a"},
		" a, b ,,c ": {"a", "b", "c"},
		"ns-a,ns-b,": {"ns-a", "ns-b"},
	}
	for in, want := range tests {
		if got := SplitCSV(in); !slices.Equal(got, want) {
			t.Errorf("SplitCSV(%q) = %q, want %q", in, got, want)
		}
	}
}
