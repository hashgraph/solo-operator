// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/sanity"
)

// prepareTerminalPhases are the NetworkUpgradePrepare status.phase values that
// mean the prepare has finished. A CR in one of these is ignored when resolving
// the in-flight upgrade's identity, so a new upgrade's Prepare is preferred over
// a previous completed one (CRs are never deleted, so stale terminal Prepares
// linger as an audit trail).
var prepareTerminalPhases = map[string]struct{}{
	string(operatorv1alpha1.PhaseSucceeded): {},
	string(operatorv1alpha1.PhaseFailed):    {},
}

// resolveOperationID returns the operationId of the in-flight upgrade. The UC
// mints operationId exactly once, at prepare (HIP-1496); every later phase —
// Freeze, Execute, FreezeAbort — inherits it and never regenerates. This reads
// it back from the most-recent non-terminal NetworkUpgradePrepare CR, with the
// cluster as the source of truth and a fallback to a Prepare persisted on disk
// but not yet delivered (the crash window).
//
// It returns ("", nil) when no Prepare exists yet in either source: the caller
// treats that as "no identity to act on this tick". The marker persists, so a
// later tick retries once the Prepare CR appears. A query/IO failure is returned
// as an error (transient — retry next tick) rather than mistaken for "absent".
func (f *Factory) resolveOperationID(ctx context.Context) (string, error) {
	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{})
	if err != nil {
		return "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list NetworkUpgradePrepare"), "PrepareListFailed"),
			"Confirm the UC ServiceAccount has list permission on networkupgradeprepares."+operatorv1alpha1.GroupVersion.Group,
		)
	}

	var (
		best   string
		bestTS time.Time
		found  bool
	)
	for i := range list.Items {
		item := &list.Items[i]
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if _, terminal := prepareTerminalPhases[phase]; terminal {
			continue
		}
		opID := item.GetLabels()[operatorv1alpha1.LabelOperationID]
		if opID == "" {
			// The label is authoritative, but fall back to the spec field in case a
			// CR was created without it.
			opID, _, _ = unstructured.NestedString(item.Object, "spec", "operationId")
		}
		if opID == "" {
			continue
		}
		if ts := item.GetCreationTimestamp().Time; !found || ts.After(bestTS) {
			best, bestTS, found = opID, ts, true
		}
	}
	if found {
		return best, nil
	}

	// Disk fallback: a Prepare persisted before its Create was acknowledged.
	// Choose the most recent by the prepare timestamp embedded in the operationId
	// — the same notion of recency the cluster path uses (creationTimestamp), not
	// a lexical compare. Lexical order is wrong here because the version sorts
	// ahead of the timestamp (e.g. "upgrade-0.9.0-<old>" > "upgrade-0.75.0-<new>").
	persisted, err := f.store.List()
	if err != nil {
		// Already an errx-decorated persist (disk) error.
		return "", err
	}
	best, bestTS, found = "", time.Time{}, false
	for _, p := range persisted {
		if p.Step != operatorv1alpha1.StepPrepare {
			continue
		}
		ts, ok := operationIDTimestamp(p.OperationID)
		if !ok {
			// A persisted Prepare the UC wrote always carries a parseable timestamp
			// (mintOperationID guarantees it); skip anything that does not rather
			// than risk emitting a malformed identity.
			continue
		}
		if !found || ts.After(bestTS) {
			best, bestTS, found = p.OperationID, ts, true
		}
	}
	return best, nil
}

// operationIDTimestamp extracts and parses the prepare timestamp embedded in an
// operationId of the form "upgrade-<version>-<YYYYMMDDTHHmmssZ>". It splits on
// the LAST "-" so a version that itself contains a dash (e.g. a "0.75.0-rc1"
// pre-release) is handled correctly. Returns ok=false if the trailing segment is
// not a valid timestamp.
func operationIDTimestamp(operationID string) (time.Time, bool) {
	idx := strings.LastIndex(operationID, "-")
	if idx < 0 || idx == len(operationID)-1 {
		return time.Time{}, false
	}
	ts, err := time.Parse(sanity.TimestampLayout, operationID[idx+1:])
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}
