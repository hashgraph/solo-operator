// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// platformStatusMetric is the Prometheus metric the consensus node exports for
// its live platform state. Its value is a numeric code (see nameForCode).
const platformStatusMetric = "platform_PlatformStatus"

// platformStatusCodeUnknown is the sentinel used for the numeric code when the
// metric endpoint is unreachable or the sample is absent. It aliases the shared
// api constant so the UC's condition and the operator's proxy agree on the value.
const platformStatusCodeUnknown = operatorv1alpha1.PlatformStatusCodeUnknown

// nameForCode returns the raw platform-status name for a numeric code, or UNKNOWN
// for an unrecognised code. It delegates to the canonical api mapping — the single
// source of truth shared with the operator, which derives the code back from the name.
func nameForCode(code int32) string {
	return operatorv1alpha1.PlatformStatusNameForCode(code)
}

// parsePlatformStatus scans a Prometheus exposition body for the
// platform_PlatformStatus sample and returns its numeric code. The sample format
// is `metric{labels} value [timestamp]`; the value is always the second
// whitespace-separated field (fields[1]) — using the last field would
// accidentally grab an optional trailing timestamp.
func parsePlatformStatus(r io.Reader) (code int32, found bool) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, platformStatusMetric) {
			continue
		}
		rest := line[len(platformStatusMetric):]
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' && rest[0] != '{' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		return int32(val), true
	}
	return 0, false
}
