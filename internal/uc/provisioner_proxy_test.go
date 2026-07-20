// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// TestProvisionerProxy_TransitionPanicRecovered verifies the transition goroutine
// spawned by Handle recovers a panic in the execute workflow. The supervisor's
// Run-level recover cannot reach into a goroutine a monitor starts itself, so the
// proxy owns this recover: the process must survive (an unrecovered panic here
// would crash the whole UC), and the single execution slot (activeOpID) must be
// released so a later transition is not blocked forever. The op must also stay
// OUT of completedOpIDs — a panic means the transition never advanced the CR, so
// it must remain retryable rather than being recorded as done.
func TestProvisionerProxy_TransitionPanicRecovered(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := fake.NewSimpleDynamicClient(scheme)
	proxy := NewProvisionerProxyWithClient(ProvisionerProxyConfig{Namespace: "solo"}, client)

	// Panic the instant the goroutine enters the transition workflow.
	proxy.onTransition = func(string) { panic("boom in transition") }

	opID := "upgrade-0.75.0-20260413T120000Z"
	cr := buildExecuteCR(opID, crCreatedAt)
	proxy.Handle(context.Background(), cr)

	// Handle dispatches the transition on a goroutine whose deferred recover must
	// keep the process alive and clear the slot. Synchronise on the slot being
	// released; reaching this assertion at all proves the panic did not unwind to
	// the top of the goroutine and abort the test binary.
	deadline := time.Now().Add(2 * time.Second)
	for {
		proxy.mu.Lock()
		cleared := proxy.activeOpID == ""
		proxy.mu.Unlock()
		if cleared {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("activeOpID not cleared after a recovered transition panic")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// A panicked transition must be treated as a failure, not a success: the op
	// must NOT be recorded as completed, so a later Handle can retry it instead of
	// skipping a transition that never actually advanced the CR.
	proxy.mu.Lock()
	_, marked := proxy.completedOpIDs[opID]
	proxy.mu.Unlock()
	if marked {
		t.Error("panicked transition recorded as completed — the stuck Execute CR would be skipped, not retried")
	}
}
