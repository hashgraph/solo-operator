// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	k8stesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"

	"k8s.io/client-go/dynamic/fake"
)

func TestParsePlatformStatus(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantCode  int32
		wantFound bool
	}{
		{"labelled active", "# HELP platform_PlatformStatus x\n# TYPE platform_PlatformStatus gauge\nplatform_PlatformStatus{node=\"0\"} 2\n", 2, true},
		{"bare starting_up", "platform_PlatformStatus 1\n", 1, true},
		{"float value", "platform_PlatformStatus{n=\"0\"} 11.0\n", 11, true},
		{"catastrophic among others", "some_other_metric 5\nplatform_PlatformStatus{node=\"0\"} 11\nanother 3\n", 11, true},
		{"absent", "# HELP other\nother_metric 1\n", 0, false},
		{"prefix-collision only", "platform_PlatformStatusExtra{n=\"0\"} 9\n", 0, false},
		{"with trailing timestamp", "platform_PlatformStatus{node=\"0\"} 2 1700000000000\n", 2, true},
		{"bare with timestamp", "platform_PlatformStatus 1 1700000000000\n", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, found := parsePlatformStatus(strings.NewReader(tc.body))
			if found != tc.wantFound || code != tc.wantCode {
				t.Errorf("parsePlatformStatus = (%d, %v), want (%d, %v)", code, found, tc.wantCode, tc.wantFound)
			}
		})
	}
}

func TestNameForCode(t *testing.T) {
	cases := map[int32]string{
		0:   operatorv1alpha1.PlatformStatusNoValue,
		1:   operatorv1alpha1.PlatformStatusStartingUp,
		2:   operatorv1alpha1.PlatformStatusActive,
		4:   operatorv1alpha1.PlatformStatusBehind,
		6:   operatorv1alpha1.PlatformStatusFreezeComplete,
		11:  operatorv1alpha1.PlatformStatusCatastrophicFailure,
		3:   operatorv1alpha1.PlatformStatusUnknown, // unmapped
		999: operatorv1alpha1.PlatformStatusUnknown, // out of range
	}
	for code, want := range cases {
		if got := nameForCode(code); got != want {
			t.Errorf("nameForCode(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestConfigCapsuleName(t *testing.T) {
	cases := map[string]string{
		"node0-0":        "node0",
		"node-1-0":       "node-1",
		"my-consensus-2": "my-consensus",
		"":               "",
	}
	for podName, want := range cases {
		c := Config{PodName: podName}
		if got := c.CapsuleName(); got != want {
			t.Errorf("CapsuleName(%q) = %q, want %q", podName, got, want)
		}
	}
}

// patchBody decodes the strategic-merge patch the sensor sends so the test can
// assert on the platform-status condition it carries.
type patchBody struct {
	Status struct {
		Conditions []corev1.PodCondition `json:"conditions"`
	} `json:"status"`
}

// TestPlatformStatusMonitor_TransitionOnlyPatchesOwnPod verifies the sensor patches
// its OWN pod's status (pods/status, strategic merge) only when the scraped platform
// status changes, and that each patch carries the correct condition — type,
// reason = raw name, status, and a message including the numeric code — including
// UNKNOWN when the endpoint is unhealthy. The patch is intercepted rather than
// applied: the fake dynamic client cannot faithfully strategic-merge an unstructured
// pod, so the merge-by-type semantics are covered separately by
// TestPlatformStatusConditionPatch_PreservesKubeletConditions.
func TestPlatformStatusMonitor_TransitionOnlyPatchesOwnPod(t *testing.T) {
	var metricCode atomic.Int64
	metricCode.Store(1) // STARTING_UP
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		c := metricCode.Load()
		if c < 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "# TYPE platform_PlatformStatus gauge\nplatform_PlatformStatus{node=\"0\"} %d\n", c)
	}))
	defer srv.Close()

	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	fakeClient := fake.NewSimpleDynamicClient(scheme)

	var (
		mu      sync.Mutex
		patches []k8stesting.PatchAction
	)
	fakeClient.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		patches = append(patches, action.(k8stesting.PatchAction))
		mu.Unlock()
		// Short-circuit: return an empty pod so the sensor's patch call succeeds
		// without depending on the fake's (unfaithful) unstructured strategic merge.
		return true, &unstructured.Unstructured{Object: map[string]interface{}{}}, nil
	})

	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	cfg.MetricsURL = srv.URL
	m := NewPlatformStatusMonitor(cfg, fakeClient, nil)
	ctx := context.Background()

	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(patches)
	}
	last := func() (corev1.PodCondition, k8stesting.PatchAction) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(patches) == 0 {
			t.Fatal("expected at least one patch")
		}
		pa := patches[len(patches)-1]
		var body patchBody
		if err := json.Unmarshal(pa.GetPatch(), &body); err != nil {
			t.Fatalf("unmarshal patch: %v", err)
		}
		if len(body.Status.Conditions) != 1 {
			t.Fatalf("expected exactly one condition in patch, got %d", len(body.Status.Conditions))
		}
		return body.Status.Conditions[0], pa
	}

	// First scrape → STARTING_UP written to the pod's platform-status condition.
	m.poll(ctx)
	if count() != 1 {
		t.Fatalf("expected 1 patch after first poll, got %d", count())
	}
	cond, pa := last()
	if pa.GetPatchType() != types.StrategicMergePatchType {
		t.Errorf("patch type = %q, want StrategicMergePatchType (conditions merge by type)", pa.GetPatchType())
	}
	if pa.GetSubresource() != "status" {
		t.Errorf("subresource = %q, want status", pa.GetSubresource())
	}
	if pa.GetName() != "node0-0" {
		t.Errorf("patched pod = %q, want node0-0 (its own pod, from UC_POD_NAME)", pa.GetName())
	}
	if cond.Type != operatorv1alpha1.PodConditionPlatformStatus {
		t.Errorf("condition type = %q, want %q", cond.Type, operatorv1alpha1.PodConditionPlatformStatus)
	}
	if cond.Reason != operatorv1alpha1.PlatformStatusStartingUp {
		t.Errorf("reason = %q, want STARTING_UP", cond.Reason)
	}
	if cond.Status != corev1.ConditionTrue {
		t.Errorf("status = %q, want True for a concrete reading", cond.Status)
	}
	if !strings.Contains(cond.Message, "code 1") {
		t.Errorf("message %q should carry the numeric code", cond.Message)
	}

	// Same code → no transition, no patch.
	m.poll(ctx)
	if count() != 1 {
		t.Fatalf("expected no additional patch on unchanged status, got %d", count())
	}

	// Transition to ACTIVE → patch.
	metricCode.Store(2)
	m.poll(ctx)
	if count() != 2 {
		t.Fatalf("expected 2 patches after ACTIVE transition, got %d", count())
	}
	if cond, _ := last(); cond.Reason != operatorv1alpha1.PlatformStatusActive {
		t.Errorf("reason = %q, want ACTIVE", cond.Reason)
	}

	// Endpoint unhealthy → fail-soft transition to UNKNOWN → patch (status Unknown),
	// connectivity error set.
	metricCode.Store(-1)
	m.poll(ctx)
	if count() != 3 {
		t.Fatalf("expected 3 patches after UNKNOWN transition, got %d", count())
	}
	cond, _ = last()
	if cond.Reason != operatorv1alpha1.PlatformStatusUnknown {
		t.Errorf("reason = %q, want UNKNOWN when endpoint unreachable", cond.Reason)
	}
	if cond.Status != corev1.ConditionUnknown {
		t.Errorf("status = %q, want Unknown for an UNKNOWN reading", cond.Status)
	}
	if m.ConnectivityError() == nil {
		t.Error("expected a ConnectivityError after an unreachable scrape")
	}

	// Recover to ACTIVE → patch again and connectivity error cleared.
	metricCode.Store(2)
	m.poll(ctx)
	if count() != 4 {
		t.Fatalf("expected 4 patches after recovery, got %d", count())
	}
	if m.ConnectivityError() != nil {
		t.Error("expected ConnectivityError cleared after a successful scrape")
	}
}

// TestPlatformStatusMonitor_RepatchesOnCodeChangeSameName verifies that when two
// distinct numeric codes map to the same name — only possible for UNKNOWN (e.g. two
// different unmapped codes) — the sensor re-patches so the condition's message reflects
// the new code, rather than leaving it stale by keying transition detection on the name
// alone. An identical re-scrape (same name AND code) is still a no-op.
func TestPlatformStatusMonitor_RepatchesOnCodeChangeSameName(t *testing.T) {
	var metricCode atomic.Int64
	metricCode.Store(3) // unmapped → UNKNOWN, code 3
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "platform_PlatformStatus{node=\"0\"} %d\n", metricCode.Load())
	}))
	defer srv.Close()

	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	fakeClient := fake.NewSimpleDynamicClient(scheme)

	var (
		mu      sync.Mutex
		patches []k8stesting.PatchAction
	)
	fakeClient.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		patches = append(patches, action.(k8stesting.PatchAction))
		mu.Unlock()
		return true, &unstructured.Unstructured{Object: map[string]interface{}{}}, nil
	})

	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	cfg.MetricsURL = srv.URL
	m := NewPlatformStatusMonitor(cfg, fakeClient, nil)
	ctx := context.Background()

	count := func() int { mu.Lock(); defer mu.Unlock(); return len(patches) }
	condAt := func(i int) corev1.PodCondition {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		var body patchBody
		if err := json.Unmarshal(patches[i].GetPatch(), &body); err != nil {
			t.Fatalf("unmarshal patch %d: %v", i, err)
		}
		if len(body.Status.Conditions) != 1 {
			t.Fatalf("patch %d: expected one condition, got %d", i, len(body.Status.Conditions))
		}
		return body.Status.Conditions[0]
	}

	// First UNKNOWN reading (code 3).
	m.poll(ctx)
	if count() != 1 {
		t.Fatalf("expected 1 patch after first poll, got %d", count())
	}
	if c := condAt(0); c.Reason != operatorv1alpha1.PlatformStatusUnknown || !strings.Contains(c.Message, "code 3") {
		t.Fatalf("first patch = reason %q message %q, want UNKNOWN with code 3", c.Reason, c.Message)
	}

	// Same name (UNKNOWN) but a different code → must re-patch with the new code in the
	// message (the staleness the review flagged).
	metricCode.Store(999)
	m.poll(ctx)
	if count() != 2 {
		t.Fatalf("expected a re-patch on a code change under the same name, got %d patches", count())
	}
	if c := condAt(1); c.Reason != operatorv1alpha1.PlatformStatusUnknown || !strings.Contains(c.Message, "code 999") {
		t.Errorf("second patch = reason %q message %q, want UNKNOWN with code 999", c.Reason, c.Message)
	}

	// Identical re-scrape (same name AND code) → no new patch.
	m.poll(ctx)
	if count() != 2 {
		t.Errorf("expected no patch on an identical re-scrape, got %d", count())
	}
}

// TestPlatformStatusConditionPatch_PreservesKubeletConditions verifies the sensor's
// strategic-merge patch updates ONLY the platform-status condition and leaves the
// kubelet-owned conditions (PodScheduled, Initialized, ContainersReady, Ready)
// intact — the "must not replace .status.conditions" invariant. It applies the real
// patch bytes with the real strategic-merge library against the typed Pod schema,
// which reproduces the API server's merge-by-`type` behaviour (unlike the fake
// dynamic client on an unstructured pod).
func TestPlatformStatusConditionPatch_PreservesKubeletConditions(t *testing.T) {
	original := &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	origBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}

	patch := platformStatusConditionPatch(operatorv1alpha1.PlatformStatusActive, 2, "2026-07-18T00:00:00Z")
	mergedBytes, err := strategicpatch.StrategicMergePatch(origBytes, patch, corev1.Pod{})
	if err != nil {
		t.Fatalf("strategic merge: %v", err)
	}
	var merged corev1.Pod
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	byType := map[corev1.PodConditionType]corev1.PodCondition{}
	for _, c := range merged.Status.Conditions {
		byType[c.Type] = c
	}
	// All four kubelet conditions must survive the patch, still True.
	for _, kubelet := range []corev1.PodConditionType{corev1.PodScheduled, corev1.PodInitialized, corev1.ContainersReady, corev1.PodReady} {
		c, ok := byType[kubelet]
		if !ok {
			t.Errorf("kubelet condition %s was clobbered by the platform-status patch", kubelet)
			continue
		}
		if c.Status != corev1.ConditionTrue {
			t.Errorf("kubelet condition %s status = %q, want True (unchanged)", kubelet, c.Status)
		}
	}
	// The platform-status condition must be merged in by type (5 total, not a replace).
	ps, ok := byType[operatorv1alpha1.PodConditionPlatformStatus]
	if !ok {
		t.Fatal("platform-status condition not present after merge")
	}
	if ps.Reason != operatorv1alpha1.PlatformStatusActive {
		t.Errorf("platform-status reason = %q, want ACTIVE", ps.Reason)
	}
	if len(merged.Status.Conditions) != 5 {
		t.Errorf("expected 5 conditions after merge (4 kubelet + platform-status), got %d", len(merged.Status.Conditions))
	}
}

// panicRoundTripper panics on every request, simulating an unexpected panic in
// the scrape path.
type panicRoundTripper struct{}

func (panicRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	panic("boom in scrape")
}

// TestPlatformStatusMonitor_RunRecoversPanic verifies a panic in the poll loop is
// recovered by Run's top-level defer and surfaced as a returned error — which the
// daemonkit supervisor turns into a back-off restart — rather than unwinding to
// the top of the goroutine and crashing the whole UC process.
func TestPlatformStatusMonitor_RunRecoversPanic(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	fakeClient := fake.NewSimpleDynamicClient(scheme)

	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	cfg.MetricsURL = "http://127.0.0.1:0/metrics"
	m := NewPlatformStatusMonitor(cfg, fakeClient, nil)
	m.http = &http.Client{Transport: panicRoundTripper{}}

	// Run does an immediate scrape before entering its ticker loop; the panicking
	// transport makes that scrape panic. Run must recover and return an error.
	err := m.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run to return a recovered-panic error, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonPlatformStatusMonitorPanic {
		t.Errorf("reason = %q, want %q", reason, ReasonPlatformStatusMonitorPanic)
	}
}
