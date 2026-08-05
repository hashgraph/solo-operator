// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"testing"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

func TestSemverString(t *testing.T) {
	cases := []struct {
		name string
		in   hiero.SemanticVersion
		want string
	}{
		{"plain", hiero.SemanticVersion{Major: 0, Minor: 58, Patch: 3}, "0.58.3"},
		{"pre", hiero.SemanticVersion{Major: 0, Minor: 58, Patch: 0, Pre: "rc1"}, "0.58.0-rc1"},
		{"build", hiero.SemanticVersion{Major: 1, Minor: 0, Patch: 2, Build: "abcdef"}, "1.0.2+abcdef"},
		{"pre and build", hiero.SemanticVersion{Major: 0, Minor: 59, Patch: 1, Pre: "alpha", Build: "sha"}, "0.59.1-alpha+sha"},
		{"zero", hiero.SemanticVersion{}, "0.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SemverString(tc.in); got != tc.want {
				t.Errorf("SemverString(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
