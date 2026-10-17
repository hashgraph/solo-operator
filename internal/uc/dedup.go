// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// specOperationIDField is a selectableField (k8s >=1.32) the UC filters on; it's
// the source of truth over the operation-id label, which is just a convenience.
const specOperationIDField = "spec.operationId"

// Operation-id deduplication (HIP-1496 §"Operation-id Deduplication").
//
// Markers persist on disk, so a handler re-sees the same marker on every poll
// tick. Without dedup it would re-create the CR each tick. Two layers prevent
// that:
//
//   - Authoritative guard: a cluster query for an existing CR carrying the same
//     spec.operationId. This is crash-safe — a restart re-queries and finds
//     the CR the previous process created. Backed by AlreadyExists on Create as
//     the ultimate, race-proof guard (first-detection-wins), so a stale or
//     missing cache entry can never cause a duplicate CR.
//   - Performance cache: an in-memory operationId -> set-of-steps map that lets
//     steady-state ticks skip the API round-trip. It is a hint only: seeded by a
//     startup query, populated on every successful create/observe, and dropped
//     for an operationId on ANY API error so the next action re-queries. A cache
//     hit short-circuits the query, but its accuracy is maintained by the query
//     and create paths above — it is never the sole basis for correctness.

// cachedCreated reports whether the in-memory cache already records a CR for
// operationID+step. It never calls the API, so it is cheap enough for a handler to
// use to skip optional work (such as reading a file) on steady-state ticks. A false
// result only means "not known to exist"; it is never proof of absence.
func (f *Factory) cachedCreated(operationID, step string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, done := f.cache[operationID][step]
	return done
}

// alreadyCreated reports whether a CR for operationID+step already exists and
// its current status phase (empty when not found or unknown).
func (f *Factory) alreadyCreated(ctx context.Context, gvr schema.GroupVersionResource, operationID, step string) (exists bool, phase string, _ error) {
	f.mu.Lock()
	if steps, ok := f.cache[operationID]; ok {
		if phase, done := steps[step]; done {
			f.mu.Unlock()
			return true, phase, nil
		}
	}
	f.mu.Unlock()

	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	list, err := f.client.Resource(gvr).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(specOperationIDField, operationID).String()})
	if err != nil {
		// Cache accuracy is suspect after an API failure — drop this operationId so
		// the next tick re-queries instead of trusting a stale (in)existence.
		f.invalidate(operationID)
		return false, "", errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "query existing %s for operation %q", gvr.Resource, operationID), ReasonDedupQueryFailed),
			"Confirm the UC ServiceAccount has list permission on "+gvr.Resource+"."+gvr.Group,
			"Check API server connectivity",
		)
	}

	found := false
	foundPhase := ""
	for i := range list.Items {
		item := &list.Items[i]
		// re-verify: a fake client in tests may not honour the field selector.
		if obsOpID, _, _ := unstructured.NestedString(item.Object, "spec", "operationId"); obsOpID != operationID {
			continue
		}
		observed := item.GetLabels()[operatorv1alpha1.LabelStep]
		if observed == "" {
			continue
		}
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		f.markCreated(operationID, observed, phase)
		if observed == step {
			found = true
			foundPhase = phase
		}
	}
	return found, foundPhase, nil
}

// inRejectCooldown reports whether operationID+step was rejected at admission
// (#1133) within the last admissionRejectRetryCooldown, so deliverCR should skip
// the persist/create cycle on this tick rather than re-send a CR the API will
// reject again.
func (f *Factory) inRejectCooldown(operationID, step string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	last, ok := f.rejected[rejectKey{operationID, step}]
	return ok && time.Since(last) < admissionRejectRetryCooldown
}

// recordRejected timestamps an admission rejection for operationID+step so the
// next admissionRejectRetryCooldown of ticks back off before retrying (#1133).
func (f *Factory) recordRejected(operationID, step string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejected[rejectKey{operationID, step}] = time.Now()
}

// clearRejected drops any admission-rejection backoff for operationID+step once
// the CR is created (e.g. after a mis-configured VAP is fixed).
func (f *Factory) clearRejected(operationID, step string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rejected, rejectKey{operationID, step})
}

// markCreated records that operationID+step now exists in the cluster and caches
// its current phase so future ticks can log correctly without an API call.
// phase may be empty when the phase is not yet known (e.g. immediately after Create).
func (f *Factory) markCreated(operationID, step, phase string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	steps := f.cache[operationID]
	if steps == nil {
		steps = make(map[string]string)
		f.cache[operationID] = steps
	}
	steps[step] = phase
}

// invalidate drops the cached steps for operationID, forcing the next
// alreadyCreated to re-query the cluster. Called on any K8s API error.
func (f *Factory) invalidate(operationID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, operationID)
}

// seedCache populates the in-memory cache by querying every in-flight
// NetworkUpgrade* CR in the namespace and recording its spec.operationId + step
// label. It is the startup half of dedup: after seeding, steady-state ticks for
// already-handled markers are served from cache without an API call. A query
// error is returned so startup recovery can surface it; the cache stays
// best-effort (partial seeding is harmless because the per-action query remains
// authoritative).
//
// The List excludes archived (terminal) CRs via listNotArchivedSelector so the
// seed — and the resulting f.cache — is bounded to in-flight operations rather
// than the full audit history (CRs are never deleted, only archived), which would
// otherwise grow the startup List O(total history). The archive filter is safe
// HERE because it only shrinks a best-effort hint: the per-action live query in
// alreadyCreated stays unfiltered and authoritative, so a terminal-but-still-
// actionable op (e.g. a Failed Execute whose marker persists on disk) is still
// found there and re-populates the cache on its first re-check. The filter must
// never be applied to that authoritative query.
func (f *Factory) seedCache(ctx context.Context) error {
	for _, gvr := range allUpgradeGVRs {
		listCtx, cancel := context.WithTimeout(ctx, listTimeout)
		list, err := f.client.Resource(gvr).Namespace(f.cfg.Namespace).
			List(listCtx, metav1.ListOptions{LabelSelector: listNotArchivedSelector})
		cancel()
		if err != nil {
			return errx.WithHints(
				errx.WithReason(errK8sAPI.Wrap(err, "seed op-id cache from %s", gvr.Resource), ReasonCacheSeedFailed),
				"Confirm the UC ServiceAccount has list permission on "+gvr.Resource+"."+gvr.Group,
			)
		}
		for i := range list.Items {
			item := &list.Items[i]
			// spec.operationId is the source of truth; the label is only a fallback.
			operationID, _, _ := unstructured.NestedString(item.Object, "spec", "operationId")
			if operationID == "" {
				operationID = item.GetLabels()[operatorv1alpha1.LabelOperationID]
			}
			step := item.GetLabels()[operatorv1alpha1.LabelStep]
			if operationID == "" || step == "" {
				continue
			}
			phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
			f.markCreated(operationID, step, phase)
		}
	}
	return nil
}
