// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"time"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

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

// alreadyCreated reports whether a CR for operationID+step already exists and
// returns its current status phase (empty string when not found or unknown).
// It consults the in-memory cache first, then falls back to listCached.
func (f *Factory) alreadyCreated(_ context.Context, gvr schema.GroupVersionResource, operationID, step string) (exists bool, phase string, _ error) {
	f.mu.Lock()
	if steps, ok := f.cache[operationID]; ok {
		if phase, done := steps[step]; done {
			f.mu.Unlock()
			return true, phase, nil
		}
	}
	f.mu.Unlock()

	items, err := f.listCached(gvr, operationID)
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
	for _, item := range items {
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

// seedCache populates the in-memory cache by querying every NetworkUpgrade* CR
// in the namespace and recording its spec.operationId + step label. It is the
// startup half of dedup: after seeding, steady-state ticks for already-handled
// markers are served from cache without an API call. A query error is returned
// so startup recovery can surface it; the cache stays best-effort (partial
// seeding is harmless because the per-action query remains authoritative).
func (f *Factory) seedCache(ctx context.Context) error {
	for _, gvr := range allUpgradeGVRs {
		listCtx, cancel := context.WithTimeout(ctx, listTimeout)
		list, err := f.client.Resource(gvr).Namespace(f.cfg.Namespace).
			List(listCtx, metav1.ListOptions{})
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
