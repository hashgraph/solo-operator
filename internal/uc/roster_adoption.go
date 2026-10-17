// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// The consensus node logs exactly one of these lines at INFO, once per upgrade restart,
// from the roster schema's restart hook (V0540RosterSchema). They are the only signal of
// whether the candidate roster was adopted: a refused candidate still lets the node reach
// ACTIVE. The match is on the message, not the logger name, so a moved class does not
// break it; a reworded message does, which is why a missing line means "unknown".
var (
	rosterAdoptRe = regexp.MustCompile(`(Adopting|Rejecting) candidate roster in round (\d+)`)
	rosterNoneRe  = regexp.MustCompile(`No candidate roster to adopt in round (\d+)`)
)

// logTimestampLayout is the CN log pattern's leading date: `%d{yyyy-MM-dd HH:mm:ss.SSS}`.
// The pattern carries no zone, so the timestamp is read as UTC (the CN container's zone).
const (
	logTimestampLayout = "2006-01-02 15:04:05.000"
	logTimestampLen    = len(logTimestampLayout)
)

// RosterAdoption is the consensus node's decision about the candidate roster at its
// last upgrade restart, as read from the log.
type RosterAdoption struct {
	// Result is one of operatorv1alpha1.RosterAdoptionAdopted, RosterAdoptionRejected
	// or RosterAdoptionNone.
	Result string `json:"result"`
	// Round is the round number the CN logged.
	Round int64 `json:"round"`
	// LoggedAt is the timestamp on the log line, or the zero time when the line carried
	// no parseable timestamp.
	LoggedAt time.Time `json:"loggedAt"`
}

// equal reports whether two results describe the same log line.
func (a RosterAdoption) equal(b RosterAdoption) bool {
	return a.Result == b.Result && a.Round == b.Round && a.LoggedAt.Equal(b.LoggedAt)
}

// parseRosterAdoptionLog extracts a roster adoption result from one log line.
func parseRosterAdoptionLog(line string) (RosterAdoption, bool) {
	var result, round string
	if m := rosterAdoptRe.FindStringSubmatch(line); m != nil {
		round = m[2]
		if m[1] == "Adopting" {
			result = operatorv1alpha1.RosterAdoptionAdopted
		} else {
			result = operatorv1alpha1.RosterAdoptionRejected
		}
	} else if m := rosterNoneRe.FindStringSubmatch(line); m != nil {
		round = m[1]
		result = operatorv1alpha1.RosterAdoptionNone
	} else {
		return RosterAdoption{}, false
	}

	n, err := strconv.ParseInt(round, 10, 64)
	if err != nil {
		return RosterAdoption{}, false
	}
	a := RosterAdoption{Result: result, Round: n}
	if len(line) >= logTimestampLen {
		if t, err := time.ParseInLocation(logTimestampLayout, line[:logTimestampLen], time.UTC); err == nil {
			a.LoggedAt = t
		}
	}
	return a, true
}

// message is the human detail line carried on the pod condition.
func (a RosterAdoption) message() string {
	var what string
	switch a.Result {
	case operatorv1alpha1.RosterAdoptionAdopted:
		what = "adopted"
	case operatorv1alpha1.RosterAdoptionRejected:
		what = "rejected"
	default:
		what = "not present (nothing to adopt)"
	}
	msg := fmt.Sprintf("Candidate roster %s in round %d", what, a.Round)
	if !a.LoggedAt.IsZero() {
		msg += " (logged " + a.LoggedAt.UTC().Format(time.RFC3339) + ")"
	}
	return msg
}

// rosterAdoptionConditionPatch builds the STRATEGIC merge patch that sets the
// operator.solo.hiero.org/roster-adoption condition on the UC's own pod. Pod conditions
// merge by `type`, so this touches only that condition and leaves the kubelet-owned ones
// intact (see platformStatusConditionPatch).
func rosterAdoptionConditionPatch(a RosterAdoption, lastTransition string) []byte {
	body := map[string]any{
		"status": map[string]any{
			"conditions": []map[string]string{{
				"type":               string(operatorv1alpha1.PodConditionRosterAdoption),
				"status":             "True",
				"reason":             a.Result,
				"message":            a.message(),
				"lastTransitionTime": lastTransition,
			}},
		},
	}
	out, _ := json.Marshal(body) // a map of strings cannot fail to marshal
	return out
}
