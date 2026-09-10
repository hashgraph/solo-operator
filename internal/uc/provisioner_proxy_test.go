// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// condFrom extracts the status.conditions entry of condType from a JSON status
// merge patch, or returns nil.
func condFrom(t *testing.T, patch []byte, condType string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(patch, &m); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	conds, _, _ := unstructured.NestedSlice(m, "status", "conditions")
	for _, raw := range conds {
		c, _ := raw.(map[string]interface{})
		if t, _, _ := unstructured.NestedString(c, "type"); t == condType {
			return c
		}
	}
	return nil
}

// TestBuildProvisionerHandoffPatch verifies the success handoff mirrors the
// daemon's reportOutcome: ConfigCRsApplied=True + DaemonResult=True and phase
// advanced to PendingNodeUpgrade, while pre-existing conditions are preserved.
func TestBuildProvisionerHandoffPatch(t *testing.T) {
	cr := newTestExecuteCR("op-1", crCreatedAt)
	_ = unstructured.SetNestedSlice(cr.Object, []interface{}{
		map[string]interface{}{"type": "ManifestValidated", "status": "True", "reason": "SchemaValid", "lastTransitionTime": "2026-05-21T18:25:31Z"},
	}, "status", "conditions")

	patch, err := buildProvisionerHandoffPatch(cr)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(patch, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if phase, _, _ := unstructured.NestedString(m, "status", "phase"); phase != string(operatorv1alpha1.ExecutePhasePendingNodeUpgrade) {
		t.Errorf("phase = %q, want PendingNodeUpgrade", phase)
	}
	for _, condType := range []string{operatorv1alpha1.CondConfigCRsApplied, operatorv1alpha1.CondDaemonResult} {
		c := condFrom(t, patch, condType)
		if c == nil {
			t.Fatalf("condition %s missing", condType)
		}
		if s, _, _ := unstructured.NestedString(c, "status"); s != string(metav1.ConditionTrue) {
			t.Errorf("%s status = %q, want True", condType, s)
		}
		if r, _, _ := unstructured.NestedString(c, "reason"); r != operatorv1alpha1.ReasonDaemonSucceeded.String() {
			t.Errorf("%s reason = %q, want Succeeded", condType, r)
		}
	}
	if condFrom(t, patch, "ManifestValidated") == nil {
		t.Error("pre-existing ManifestValidated condition was clobbered")
	}
}

// TestHandoffDeadlineExceeded verifies the deadline is measured from the CR's
// status.startTime, and that a missing/unparseable anchor is treated as NOT
// exceeded (stays retryable rather than forcing a premature terminal).
func TestHandoffDeadlineExceeded(t *testing.T) {
	proxy := NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:       "solo",
		HandoffDeadline: time.Hour,
	}, nil)

	tests := []struct {
		name      string
		startTime string
		want      bool
	}{
		{"no anchor", "", false},
		{"unparseable anchor", "not-a-time", false},
		{"within budget", time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339), false},
		{"past budget", time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cr := newTestExecuteCR("op-1", crCreatedAt)
			if tc.startTime != "" {
				_ = unstructured.SetNestedField(cr.Object, tc.startTime, "status", "startTime")
			}
			if got := proxy.handoffDeadlineExceeded(cr); got != tc.want {
				t.Errorf("handoffDeadlineExceeded = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildProvisionerFailurePatch verifies the failure handoff writes
// DaemonResult=False carrying the error's reason and leaves status.phase untouched.
func TestBuildProvisionerFailurePatch(t *testing.T) {
	cr := newTestExecuteCR("op-1", crCreatedAt)

	patch, err := buildProvisionerFailurePatch(cr, operatorv1alpha1.ReasonDaemonFileHashMismatch.String(), "hash mismatch")
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(patch, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, found, _ := unstructured.NestedString(m, "status", "phase"); found {
		t.Error("failure patch must not set status.phase")
	}
	c := condFrom(t, patch, operatorv1alpha1.CondDaemonResult)
	if c == nil {
		t.Fatal("DaemonResult condition missing")
	}
	if s, _, _ := unstructured.NestedString(c, "status"); s != string(metav1.ConditionFalse) {
		t.Errorf("DaemonResult status = %q, want False", s)
	}
	if r, _, _ := unstructured.NestedString(c, "reason"); r != operatorv1alpha1.ReasonDaemonFileHashMismatch.String() {
		t.Errorf("DaemonResult reason = %q, want FileHashMismatch", r)
	}
}

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
	cr := newTestExecuteCR(opID, crCreatedAt)
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
