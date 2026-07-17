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
// its live platform state. Its value is a numeric code (see platformStatusName).
const platformStatusMetric = "platform_PlatformStatus"

// platformStatusName maps the numeric platform_PlatformStatus code to its raw
// state name. The mapping mirrors the Hedera platform PlatformStatus enum and
// the solo reference (src/core/enumerations.ts, NodeStatusCodes). Codes with no
// mapping (e.g. 3) resolve to UNKNOWN.
var platformStatusName = map[int32]string{
	0:  operatorv1alpha1.PlatformStatusNoValue,
	1:  operatorv1alpha1.PlatformStatusStartingUp,
	2:  operatorv1alpha1.PlatformStatusActive,
	4:  operatorv1alpha1.PlatformStatusBehind,
	5:  operatorv1alpha1.PlatformStatusFreezing,
	6:  operatorv1alpha1.PlatformStatusFreezeComplete,
	7:  operatorv1alpha1.PlatformStatusReplayingEvents,
	8:  operatorv1alpha1.PlatformStatusObserving,
	9:  operatorv1alpha1.PlatformStatusChecking,
	10: operatorv1alpha1.PlatformStatusReconnectComplete,
	11: operatorv1alpha1.PlatformStatusCatastrophicFailure,
}

// nameForCode returns the raw platform-status name for a numeric code, or
// UNKNOWN for an unrecognised code.
func nameForCode(code int32) string {
	if name, ok := platformStatusName[code]; ok {
		return name
	}
	return operatorv1alpha1.PlatformStatusUnknown
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
