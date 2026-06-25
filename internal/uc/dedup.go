// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// Operation-id deduplication (HIP-1496 §"Operation-id Deduplication").
//
// Markers persist on disk, so a handler re-sees the same marker on every poll
// tick. Without dedup it would re-create the CR each tick. Two layers prevent
// that:
//
//   - Authoritative guard: a cluster query for an existing CR carrying the same
//     operation-id label. This is crash-safe — a restart re-queries and finds
//     the CR the previous process created. Correctness rests here alone.
//   - Performance cache: an in-memory operationId -> set-of-steps map that lets
//     steady-state ticks skip the API round-trip. It is a hint only: seeded by a
//     startup query, populated on every successful create/observe, and dropped
//     for an operationId on ANY API error so the next action re-queries. The
//     cache is never trusted for correctness, only to avoid a redundant call.

// alreadyCreated reports whether a CR for operationID+step already exists. It
// consults the in-memory cache first; on a miss it falls back to the
// authoritative cluster query (by operation-id label on gvr) and records every
// step it observes. A K8s API error invalidates the cache for operationID and
// is returned to the caller so the action is retried on the next tick rather
// than proceeding on stale state.
func (f *Factory) alreadyCreated(ctx context.Context, gvr schema.GroupVersionResource, operationID, step string) (bool, error) {
	f.mu.Lock()
	if steps, ok := f.cache[operationID]; ok {
		if _, done := steps[step]; done {
			f.mu.Unlock()
			return true, nil
		}
	}
	f.mu.Unlock()

	listCtx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	selector := labels.SelectorFromSet(labels.Set{operatorv1alpha1.LabelOperationID: operationID}).String()
	list, err := f.client.Resource(gvr).Namespace(f.cfg.Namespace).
		List(listCtx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		// Cache accuracy is suspect after an API failure — drop this operationId so
		// the next tick re-queries instead of trusting a stale (in)existence.
		f.invalidate(operationID)
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "query existing %s for operation %q", gvr.Resource, operationID), "DedupQueryFailed"),
			"Confirm the UC ServiceAccount has list permission on "+gvr.Resource+"."+gvr.Group,
			"Check API server connectivity",
		)
	}

	found := false
	for i := range list.Items {
		l := list.Items[i].GetLabels()
		// Defend against a server/fake-client that does not honour the label
		// selector: only credit CRs whose operation-id label actually matches.
		if l[operatorv1alpha1.LabelOperationID] != operationID {
			continue
		}
		observed := l[operatorv1alpha1.LabelStep]
		if observed == "" {
			continue
		}
		f.markCreated(operationID, observed)
		if observed == step {
			found = true
		}
	}
	return found, nil
}

// markCreated records that operationID+step now exists in the cluster so future
// ticks skip the API query. Safe to call after a successful create or after
// observing the CR via a query.
func (f *Factory) markCreated(operationID, step string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	steps := f.cache[operationID]
	if steps == nil {
		steps = make(map[string]struct{})
		f.cache[operationID] = steps
	}
	steps[step] = struct{}{}
}

// invalidate drops the cached steps for operationID, forcing the next
// alreadyCreated to re-query the cluster. Called on any K8s API error.
func (f *Factory) invalidate(operationID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, operationID)
}

// seedCache populates the in-memory cache by querying every NetworkUpgrade* CR
// in the namespace and recording its operation-id + step labels. It is the
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
				errx.WithReason(errK8sAPI.Wrap(err, "seed op-id cache from %s", gvr.Resource), "CacheSeedFailed"),
				"Confirm the UC ServiceAccount has list permission on "+gvr.Resource+"."+gvr.Group,
			)
		}
		for i := range list.Items {
			l := list.Items[i].GetLabels()
			operationID, step := l[operatorv1alpha1.LabelOperationID], l[operatorv1alpha1.LabelStep]
			if operationID == "" || step == "" {
				continue
			}
			f.markCreated(operationID, step)
		}
	}
	return nil
}
