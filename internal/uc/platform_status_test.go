// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"

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

// newCapsuleObj returns an unstructured ConsensusCapsule the fake dynamic client
// can serve to the platform-status sensor's status patch.
func newCapsuleObj(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": operatorv1alpha1.GroupVersion.String(),
		"kind":       "ConsensusCapsule",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}}
}

// TestPlatformStatusMonitor_TransitionOnlyWrites verifies the sensor patches the
// capsule only when the scraped platform status changes, and writes the correct
// raw name + numeric code (including UNKNOWN when the endpoint is unhealthy).
func TestPlatformStatusMonitor_TransitionOnlyWrites(t *testing.T) {
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
	fakeClient := fake.NewSimpleDynamicClient(scheme, newCapsuleObj("node0", "solo"))

	var patchCount int32
	fakeClient.PrependReactor("patch", "consensuscapsules", func(k8stesting.Action) (bool, runtime.Object, error) {
		atomic.AddInt32(&patchCount, 1)
		return false, nil, nil // fall through to the default tracker
	})

	cfg := DefaultConfig()
	cfg.Namespace = "solo"
	cfg.PodName = "node0-0"
	cfg.PodRevision = "rev-abc123"
	cfg.MetricsURL = srv.URL
	m := NewPlatformStatusMonitor(cfg, fakeClient, nil)
	ctx := context.Background()

	readStatus := func() (string, int64, string) {
		got, err := fakeClient.Resource(consensusCapsuleGVR).Namespace("solo").Get(ctx, "node0", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get capsule: %v", err)
		}
		name, _, _ := unstructured.NestedString(got.Object, "status", "platformStatus")
		code, _, _ := unstructured.NestedInt64(got.Object, "status", "platformStatusCode")
		rev, _, _ := unstructured.NestedString(got.Object, "status", "platformStatusRevision")
		return name, code, rev
	}

	// First scrape → STARTING_UP written.
	m.poll(ctx)
	if got := atomic.LoadInt32(&patchCount); got != 1 {
		t.Fatalf("expected 1 patch after first poll, got %d", got)
	}
	if name, code, rev := readStatus(); name != operatorv1alpha1.PlatformStatusStartingUp || code != 1 || rev != "rev-abc123" {
		t.Fatalf("status = (%q,%d,%q), want (STARTING_UP,1,rev-abc123)", name, code, rev)
	}

	// Same code → no transition, no patch.
	m.poll(ctx)
	if got := atomic.LoadInt32(&patchCount); got != 1 {
		t.Fatalf("expected no additional patch on unchanged status, got %d", got)
	}

	// Transition to ACTIVE → patch.
	metricCode.Store(2)
	m.poll(ctx)
	if got := atomic.LoadInt32(&patchCount); got != 2 {
		t.Fatalf("expected 2 patches after ACTIVE transition, got %d", got)
	}
	if name, code, rev := readStatus(); name != operatorv1alpha1.PlatformStatusActive || code != 2 || rev != "rev-abc123" {
		t.Fatalf("status = (%q,%d,%q), want (ACTIVE,2,rev-abc123)", name, code, rev)
	}

	// Endpoint unhealthy → fail-soft transition to UNKNOWN → patch, connectivity error set.
	metricCode.Store(-1)
	m.poll(ctx)
	if got := atomic.LoadInt32(&patchCount); got != 3 {
		t.Fatalf("expected 3 patches after UNKNOWN transition, got %d", got)
	}
	if name, _, _ := readStatus(); name != operatorv1alpha1.PlatformStatusUnknown {
		t.Fatalf("status = %q, want UNKNOWN when endpoint unreachable", name)
	}
	if m.ConnectivityError() == nil {
		t.Error("expected a ConnectivityError after an unreachable scrape")
	}

	// Recover to ACTIVE → patch again and connectivity error cleared.
	metricCode.Store(2)
	m.poll(ctx)
	if got := atomic.LoadInt32(&patchCount); got != 4 {
		t.Fatalf("expected 4 patches after recovery, got %d", got)
	}
	if m.ConnectivityError() != nil {
		t.Error("expected ConnectivityError cleared after a successful scrape")
	}
}
