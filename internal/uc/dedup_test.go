// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	dynamicfake "k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

// countReactor returns a reactor that counts how many times a verb is invoked.
// It does not short-circuit (returns handled=false) so the default tracker still
// services the call.
func countReactor(counter *int64) k8stesting.ReactionFunc {
	return func(k8stesting.Action) (bool, runtime.Object, error) {
		atomic.AddInt64(counter, 1)
		return false, nil, nil
	}
}

// TestHandleExecuteImmediateCacheAvoidsAPIOnSteadyState verifies that after the
// CR is created once, subsequent ticks (marker still on disk) are served from
// the in-memory cache without any List or Create round-trip.
func TestHandleExecuteImmediateCacheAvoidsAPIOnSteadyState(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	var lists, creates int64
	fake := f.client.(*dynamicfake.FakeDynamicClient)
	fake.PrependReactor("list", "networkupgradeprepares", countReactor(&lists))
	fake.PrependReactor("create", "networkupgradeprepares", countReactor(&creates))

	// First tick: cache miss -> one List (dedup query) + one Create.
	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if lists != 1 || creates != 1 {
		t.Fatalf("first tick: lists=%d creates=%d, want 1/1", lists, creates)
	}

	// Subsequent ticks: cache hit -> no further API calls at all.
	for i := 0; i < 5; i++ {
		if err := f.HandleExecuteImmediate(context.Background()); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if lists != 1 {
		t.Fatalf("steady-state ticks issued %d List calls, want 1 (cache should serve them)", lists)
	}
	if creates != 1 {
		t.Fatalf("steady-state ticks issued %d Create calls, want exactly 1", creates)
	}
}

// TestDedupSeededCacheSkipsCreate verifies that a CR already present in the
// cluster at startup (seeded into the cache) means the first tick creates
// nothing — the second-detection-of-an-existing-CR acceptance case across a
// process restart.
func TestDedupSeededCacheSkipsCreate(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	// Pre-create the CR as a prior process would have, then seed the cache as a
	// restart's RecoverOnStartup does.
	cr, err := buildPrepareCR(f.cfg, wantOperationID, parseImages(t, root))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	if err := f.RecoverOnStartup(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var creates int64
	f.client.(*dynamicfake.FakeDynamicClient).
		PrependReactor("create", "networkupgradeprepares", countReactor(&creates))

	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if creates != 0 {
		t.Fatalf("create issued %d times against an already-existing CR, want 0", creates)
	}
}

// TestDedupAPIErrorInvalidatesCache verifies a failed dedup List drops the
// cache entry and propagates the error, so the next attempt re-queries.
func TestDedupAPIErrorInvalidatesCache(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	// stale entry, to prove the error path clears it
	f.markCreated(wantOperationID, operatorv1alpha1.StepFreeze, "")

	var fail atomic.Bool
	fail.Store(true)
	var lists int64
	f.client.(*dynamicfake.FakeDynamicClient).
		PrependReactor("list", "networkupgradeprepares", func(k8stesting.Action) (bool, runtime.Object, error) {
			atomic.AddInt64(&lists, 1)
			if fail.Load() {
				return true, nil, k8serrors.NewInternalError(errors.New("boom"))
			}
			return false, nil, nil
		})

	if err := f.HandleExecuteImmediate(context.Background()); err == nil {
		t.Fatal("expected dedup query error to propagate")
	}
	f.mu.Lock()
	_, present := f.cache[wantOperationID]
	f.mu.Unlock()
	if present {
		t.Fatal("cache entry survived an API error; want it invalidated")
	}

	fail.Store(false)
	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("recovery tick: %v", err)
	}
	if lists < 2 {
		t.Fatalf("expected a re-query after invalidation, got %d List calls", lists)
	}
	getPrepare(t, f, wantOperationID)
}

// TestSeedCacheExcludesArchived verifies the startup seed hydrates the dedup
// cache only from in-flight CRs, skipping archived (terminal) ones so f.cache is
// bounded to live operations rather than the full audit history.
func TestSeedCacheExcludesArchived(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	inFlight := newTestCR("NetworkUpgradePrepare", "prep-live", "solo", "upgrade-live-20260413T120000Z", operatorv1alpha1.StepPrepare)

	archived := newTestCR("NetworkUpgradePrepare", "prep-old", "solo", "upgrade-old-20260101T120000Z", operatorv1alpha1.StepPrepare)
	labels := archived.GetLabels()
	labels[common.LabelLifecycle] = common.LifecycleArchived
	archived.SetLabels(labels)

	for _, cr := range []*unstructured.Unstructured{inFlight, archived} {
		if _, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
			Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
			t.Fatalf("pre-create %s: %v", cr.GetName(), err)
		}
	}

	if err := f.seedCache(context.Background()); err != nil {
		t.Fatalf("seedCache: %v", err)
	}

	f.mu.Lock()
	_, liveSeeded := f.cache["upgrade-live-20260413T120000Z"]
	_, archivedSeeded := f.cache["upgrade-old-20260101T120000Z"]
	f.mu.Unlock()

	if !liveSeeded {
		t.Error("in-flight operation not seeded into dedup cache")
	}
	if archivedSeeded {
		t.Error("archived operation seeded into dedup cache; want it excluded by listNotArchivedSelector")
	}
}

// TestHandleExecuteImmediateConcurrentNeverDuplicates verifies that concurrent
// ticks for the same op-id converge on exactly one CR: the cache races are
// benign because the K8s API AlreadyExists check is the authoritative guard.
func TestHandleExecuteImmediateConcurrentNeverDuplicates(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.HandleExecuteImmediate(context.Background())
		}()
	}
	wg.Wait()

	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("concurrent ticks produced %d CRs, want exactly 1", len(list.Items))
	}
}

// parseImages reads the components manifest written under root and returns its
// images block, for tests that need to build a CR directly.
func parseImages(t *testing.T, root string) operatorv1alpha1.UpgradeImagesSpec {
	t.Helper()
	m, err := parseComponentsManifest(filepath.Join(root, manifestsSubdir, componentsManifestFile))
	if err != nil {
		t.Fatalf("parse images: %v", err)
	}
	return m.Images
}
