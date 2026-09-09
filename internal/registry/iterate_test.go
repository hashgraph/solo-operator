// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// fakeChecker records every image it is asked to check and delegates the result
// to fn. It is safe for concurrent use.
type fakeChecker struct {
	mu   sync.Mutex
	seen []string
	fn   func(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult
}

func (c *fakeChecker) Check(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec, _ operatorv1alpha1.ImageDeterministicSpec) RegistryResult {
	c.mu.Lock()
	c.seen = append(c.seen, spec.Image)
	c.mu.Unlock()
	if c.fn != nil {
		return c.fn(ctx, spec)
	}
	return RegistryResult{Image: spec.Image, State: StateValid}
}

func componentWith(n int) operatorv1alpha1.ImageComponentSpec {
	regs := make([]operatorv1alpha1.ImageRegistrySpec, n)
	for i := range regs {
		regs[i] = operatorv1alpha1.ImageRegistrySpec{Image: fmt.Sprintf("img-%d", i)}
	}
	return operatorv1alpha1.ImageComponentSpec{Registries: regs}
}

func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

func randStrategy(seed uint64) RandomStrategy {
	return RandomStrategy{Rand: newRNG(seed)}
}

func resultImages(results []RegistryResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Image
	}
	return out
}

// The first registry succeeding must not stop the rest from being checked.
func TestCheckAll_NoEarlyExit(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			// Everything "succeeds" — a naive early-exit impl would stop after one.
			return RegistryResult{Image: spec.Image, State: StateValid}
		},
	}

	comp := componentWith(5)
	results := CheckAll(context.Background(), checker, comp, time.Second, randStrategy(1))

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	if len(checker.seen) != 5 {
		t.Fatalf("expected all 5 registries checked, got %d", len(checker.seen))
	}

	// Every registry from the spec must appear exactly once in the results.
	got := resultImages(results)
	sort.Strings(got)
	for i := 0; i < 5; i++ {
		want := fmt.Sprintf("img-%d", i)
		if got[i] != want {
			t.Errorf("missing registry result: want %q at %d, got %q", want, i, got[i])
		}
	}
}

// A failing registry is recorded with its error, and siblings are unaffected.
func TestCheckAll_RecordsFailuresWithoutAborting(t *testing.T) {
	boom := fmt.Errorf("unreachable")
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			if spec.Image == "img-2" {
				return RegistryResult{Image: spec.Image, State: StateUnreachable, Err: boom}
			}
			return RegistryResult{Image: spec.Image, State: StateValid}
		},
	}

	results := CheckAll(context.Background(), checker, componentWith(4), time.Second, randStrategy(7))

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	var failures, successes int
	for _, r := range results {
		if r.Image == "img-2" {
			if r.State != StateUnreachable || r.Err == nil {
				t.Errorf("img-2 should be recorded as failed, got %+v", r)
			}
			failures++
		} else if r.State == StateValid {
			successes++
		}
	}
	if failures != 1 || successes != 3 {
		t.Errorf("expected 1 failure and 3 successes, got %d/%d", failures, successes)
	}
}

// Iteration order is randomised: a fixed seed is deterministic, and across many
// seeds we observe more than one ordering.
func TestCheckAll_RandomisedOrder(t *testing.T) {
	comp := componentWith(6)

	// Same seed → same order (deterministic, testable).
	a := resultImages(CheckAll(context.Background(), &fakeChecker{}, comp, time.Second, randStrategy(42)))
	b := resultImages(CheckAll(context.Background(), &fakeChecker{}, comp, time.Second, randStrategy(42)))
	if !slices.Equal(a, b) {
		t.Fatalf("same seed produced different orders:\n%v\n%v", a, b)
	}

	// Different seeds → at least two distinct orderings observed.
	orders := map[string]struct{}{}
	for seed := uint64(0); seed < 20; seed++ {
		order := resultImages(CheckAll(context.Background(), &fakeChecker{}, comp, time.Second, randStrategy(seed)))
		orders[fmt.Sprint(order)] = struct{}{}
	}
	if len(orders) < 2 {
		t.Errorf("expected randomised order across seeds, only saw %d distinct ordering(s)", len(orders))
	}
}

// A slow registry is bounded by the per-check timeout and recorded as failed,
// while fast siblings still succeed.
func TestCheckAll_PerCheckTimeout(t *testing.T) {
	checker := &fakeChecker{
		fn: func(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			if spec.Image == "img-1" {
				<-ctx.Done() // never returns until the timeout fires
				return RegistryResult{Image: spec.Image, State: StateUnreachable, Err: ctx.Err()}
			}
			return RegistryResult{Image: spec.Image, State: StateValid}
		},
	}

	start := time.Now()
	results := CheckAll(context.Background(), checker, componentWith(3), 50*time.Millisecond, randStrategy(3))
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("expected timeout to bound the slow check, took %v", elapsed)
	}
	for _, r := range results {
		if r.Image == "img-1" {
			if r.State != StateUnreachable || r.Err == nil {
				t.Errorf("slow registry should be recorded as failed, got %+v", r)
			}
		} else if r.State != StateValid {
			t.Errorf("fast registry %q should succeed, got %+v", r.Image, r)
		}
	}
}

// Checks run concurrently: total wall time is far less than the sum of the
// individual checks.
func TestCheckAll_RunsConcurrently(t *testing.T) {
	const per = 80 * time.Millisecond
	checker := &fakeChecker{
		fn: func(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
			select {
			case <-time.After(per):
			case <-ctx.Done():
			}
			return RegistryResult{Image: spec.Image, State: StateValid}
		},
	}

	// 4 registries == the concurrency limit, so they run in one wave.
	start := time.Now()
	CheckAll(context.Background(), checker, componentWith(4), time.Second, randStrategy(1))
	elapsed := time.Since(start)

	if elapsed >= 3*per {
		t.Errorf("expected concurrent execution (~%v), serial-looking elapsed %v", per, elapsed)
	}
}
