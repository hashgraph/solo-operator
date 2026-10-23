// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import corev1 "k8s.io/api/core/v1"

// PodConditionPlatformStatus is the custom pod condition type the UC sidecar
// maintains on its OWN pod (pods/status) to report the consensus node's live
// platform status. It is informational only — deliberately NOT listed in
// spec.readinessGates — so it never affects pod readiness ("detection is active
// polling, not a probe").
//
// Encoding: reason = the raw platform-status name (ACTIVE / REPLAYING_EVENTS /
// CATASTROPHIC_FAILURE / … — machine-readable); message = a human detail line
// including the numeric code. The operator reads it off the current pod and
// proxies the raw value into ConsensusCapsuleStatus.PlatformStatus, so the
// operator stays the sole writer of the ConsensusCapsule status.
const PodConditionPlatformStatus corev1.PodConditionType = "operator.solo.hiero.org/platform-status"

// PlatformStatusCodeUnknown is the sentinel PlatformStatusCode carries when the
// platform status is UNKNOWN (metrics endpoint unreachable, sample absent, or an
// unrecognised code). -1 avoids collision with the real enum (code 0 = NO_VALUE).
const PlatformStatusCodeUnknown int32 = -1

// platformStatusCodeToName is the canonical mapping between the numeric
// platform_PlatformStatus code the consensus node exports and its raw state name.
// It is the single source of truth shared by the UC sidecar (which parses the
// numeric code from metrics into a name) and the operator (which derives the code
// back from the name it proxies off the pod condition). The mapping mirrors the
// Hedera platform PlatformStatus enum and the solo reference
// (src/core/enumerations.ts, NodeStatusCodes); codes with no entry resolve to UNKNOWN.
var platformStatusCodeToName = map[int32]string{
	0:  PlatformStatusNoValue,
	1:  PlatformStatusStartingUp,
	2:  PlatformStatusActive,
	4:  PlatformStatusBehind,
	5:  PlatformStatusFreezing,
	6:  PlatformStatusFreezeComplete,
	7:  PlatformStatusReplayingEvents,
	8:  PlatformStatusObserving,
	9:  PlatformStatusChecking,
	10: PlatformStatusReconnectComplete,
	11: PlatformStatusCatastrophicFailure,
}

// platformStatusNameToCode is the reverse of platformStatusCodeToName, built once
// so PlatformStatusCodeForName is an O(1) lookup rather than a linear scan on every
// reconcile. Names in platformStatusCodeToName are unique, so the inversion is total.
var platformStatusNameToCode = func() map[string]int32 {
	m := make(map[string]int32, len(platformStatusCodeToName))
	for code, name := range platformStatusCodeToName {
		m[name] = code
	}
	return m
}()

// PlatformStatusNameForCode returns the raw platform-status name for a numeric
// platform_PlatformStatus code, or UNKNOWN for an unrecognised code.
func PlatformStatusNameForCode(code int32) string {
	if name, ok := platformStatusCodeToName[code]; ok {
		return name
	}
	return PlatformStatusUnknown
}

// PlatformStatusCodeForName returns the numeric platform_PlatformStatus code for a
// raw state name, or PlatformStatusCodeUnknown for UNKNOWN / an unrecognised name.
// It is the inverse of PlatformStatusNameForCode and lets the operator re-derive
// the code from the name it reads off the pod condition — the condition carries
// the name in its reason, and the code is observability-only.
func PlatformStatusCodeForName(name string) int32 {
	if code, ok := platformStatusNameToCode[name]; ok {
		return code
	}
	return PlatformStatusCodeUnknown
}

// PodConditionRosterAdoption is the custom pod condition type the UC sidecar
// maintains on its OWN pod to report what the consensus node decided about the
// candidate roster at its last upgrade restart. Like PodConditionPlatformStatus it is
// informational only (not a readiness gate) and the operator proxies it; the UC never
// patches the ConsensusCapsule.
//
// Encoding: reason = RosterAdoptionAdopted, RosterAdoptionRejected or
// RosterAdoptionNone (machine-readable); message = a human detail line with the log
// round and time; lastTransitionTime = when the UC observed it. The result comes from
// the consensus node's log, so a condition that is absent means "unknown", never
// "adopted".
const PodConditionRosterAdoption corev1.PodConditionType = "operator.solo.hiero.org/roster-adoption"

// Roster adoption results carried in the reason of PodConditionRosterAdoption.
const (
	// RosterAdoptionAdopted: the CN replaced its active roster with the candidate.
	RosterAdoptionAdopted = "Adopted"
	// RosterAdoptionRejected: the CN kept the old roster because its TSS readiness
	// check refused the candidate.
	RosterAdoptionRejected = "Rejected"
	// RosterAdoptionNone: the CN had no candidate roster to adopt.
	RosterAdoptionNone = "NoCandidate"
)
