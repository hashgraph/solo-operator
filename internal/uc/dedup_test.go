// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	dynamicfake "k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
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

// TestHandleExecuteImmediateCacheAvoidsAPIOnSteadyState verifies the dedup query never issues a live List.
func TestHandleExecuteImmediateCacheAvoidsAPIOnSteadyState(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	var lists, creates int64
	fake := f.client.(*dynamicfake.FakeDynamicClient)
	fake.PrependReactor("list", "networkupgradeprepares", countReactor(&lists))
	fake.PrependReactor("create", "networkupgradeprepares", countReactor(&creates))

	// First tick: dedup reads the informer cache (no live List) -> one Create.
	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if lists != 0 || creates != 1 {
		t.Fatalf("first tick: lists=%d creates=%d, want 0/1", lists, creates)
	}

	// Subsequent ticks: cache hit -> no further API calls at all.
	for i := 0; i < 5; i++ {
		if err := f.HandleExecuteImmediate(context.Background()); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if lists != 0 {
		t.Fatalf("steady-state ticks issued %d List calls, want 0 (cache should serve them)", lists)
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
