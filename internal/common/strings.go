// SPDX-License-Identifier: Apache-2.0

package common

import "strings"

// SplitCSV splits a comma-separated list, trimming spaces and dropping empty entries.
func SplitCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
