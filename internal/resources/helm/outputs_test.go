// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"maps"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func blockNodeCapsule(name, namespace string) *operatorv1alpha1.HelmCapsule {
	hc := newHelmCapsule(name, namespace)
	hc.Spec.Chart.Chart = "block-node-server"
	comp := operatorv1alpha1.HelmComponentBlockNode
	hc.Spec.Component = &comp
	return hc
}

func TestComponentOutputs_blockNode(t *testing.T) {
	tests := []struct {
		name    string
		release string
		values  map[string]interface{}
		want    map[string]string
	}{
		{
			name:    "defaults",
			release: "block-node",
			values:  map[string]interface{}{},
			want: map[string]string{
				"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
		{
			name:    "release containing chart name is used as-is",
			release: "bn-block-node-server",
			values:  map[string]interface{}{},
			want: map[string]string{
				"address":       "bn-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
		{
			name:    "nameOverride replaces chart name",
			release: "release1",
			values:  map[string]interface{}{"nameOverride": "bn"},
			want: map[string]string{
				"address":       "release1-bn.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
		{
			name:    "fullnameOverride wins",
			release: "block-node",
			values:  map[string]interface{}{"fullnameOverride": "my-bn", "nameOverride": "ignored"},
			want: map[string]string{
				"address":       "my-bn.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
		{
			name:    "service.port is the shared fallback",
			release: "block-node",
			values:  map[string]interface{}{"service": map[string]interface{}{"port": float64(8080)}},
			want: map[string]string{
				"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "8080",
				"servicePort":   "8080",
			},
		},
		{
			name:    "plugin ports override service.port, numbers or strings",
			release: "block-node",
			values: map[string]interface{}{
				"service":   map[string]interface{}{"port": float64(8080)},
				"blockNode": map[string]interface{}{"ports": map[string]interface{}{"publisher": float64(40984), "serverStatus": "40982"}},
			},
			want: map[string]string{
				"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "40984",
				"servicePort":   "40982",
			},
		},
		{
			name:    "fractional ports fall back",
			release: "block-node",
			values: map[string]interface{}{
				"blockNode": map[string]interface{}{"ports": map[string]interface{}{"publisher": 40840.9}},
			},
			want: map[string]string{
				"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
		{
			name:    "null and zero plugin ports fall back",
			release: "block-node",
			values: map[string]interface{}{
				"blockNode": map[string]interface{}{"ports": map[string]interface{}{"publisher": nil, "serverStatus": float64(0)}},
			},
			want: map[string]string{
				"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
				"streamingPort": "40840",
				"servicePort":   "40840",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := componentOutputs(blockNodeCapsule(tt.release, "test-ns"), tt.release, tt.values)
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponentOutputs_longNameIsTruncatedLikeHelm(t *testing.T) {
	release := strings.Repeat("a", 60)
	got := componentOutputs(blockNodeCapsule(release, "ns"), release, map[string]interface{}{})
	host := strings.SplitN(got["address"], ".", 2)[0]
	if len(host) != 63 {
		t.Errorf("expected 63-char Service name, got %d (%s)", len(host), host)
	}
}

func TestComponentOutputs_nilWithoutOutputs(t *testing.T) {
	if got := componentOutputs(newHelmCapsule("mirror", "ns"), "mirror", nil); got != nil {
		t.Errorf("expected nil for capsule without component, got %v", got)
	}
	comp := operatorv1alpha1.HelmComponentExplorer
	hc := newHelmCapsule("explorer", "ns")
	hc.Spec.Component = &comp
	if got := componentOutputs(hc, "explorer", nil); got != nil {
		t.Errorf("expected nil for explorer, got %v", got)
	}
}

func TestReleaseSubReconciler_publishesOutputs(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := blockNodeCapsule("block-node", "test-ns")
	hc.Spec.OrbitRef = &corev1.LocalObjectReference{Name: "test-orbit"}
	r := newTestReleaseSubReconciler(scheme, mock, hc, makeOrbit("test-orbit", ""))

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"address":       "block-node-block-node-server.test-ns.svc.cluster.local",
		"streamingPort": "40840",
		"servicePort":   "40840",
	}
	if !maps.Equal(hc.Status.Outputs, want) {
		t.Errorf("got outputs %v, want %v", hc.Status.Outputs, want)
	}
}

func TestReleaseSubReconciler_backfillsOutputsWhenUpToDate(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := blockNodeCapsule("block-node", "test-ns")
	hc.Spec.OrbitRef = &corev1.LocalObjectReference{Name: "test-orbit"}
	r := newTestReleaseSubReconciler(scheme, mock, hc, makeOrbit("test-orbit", ""))

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	hc.Status.Outputs = nil
	if err := r.Client.Status().Update(context.Background(), hc); err != nil {
		t.Fatalf("clearing outputs: %v", err)
	}
	version := mock.releases["test-ns/block-node"].Version

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}
	if mock.releases["test-ns/block-node"].Version != version {
		t.Error("expected no Helm upgrade when only outputs were missing")
	}
	if hc.Status.Outputs["address"] == "" {
		t.Error("expected outputs to be backfilled on an up-to-date release")
	}
}

func TestReleaseSubReconciler_noOutputsUntilDeployed(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "pending-install", ChartString: chart.Chart + "-" + chart.Version}, nil
	}
	hc := blockNodeCapsule("block-node", "test-ns")
	hc.Spec.OrbitRef = &corev1.LocalObjectReference{Name: "test-orbit"}
	r := newTestReleaseSubReconciler(scheme, mock, hc, makeOrbit("test-orbit", ""))

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hc.Status.Outputs) != 0 {
		t.Errorf("expected no outputs before deployed, got %v", hc.Status.Outputs)
	}
}

func TestReleaseSubReconciler_clearsOutputsWhenComponentRemoved(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := blockNodeCapsule("block-node", "test-ns")
	hc.Spec.OrbitRef = &corev1.LocalObjectReference{Name: "test-orbit"}
	r := newTestReleaseSubReconciler(scheme, mock, hc, makeOrbit("test-orbit", ""))

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	if len(hc.Status.Outputs) == 0 {
		t.Fatal("expected outputs after first reconcile")
	}

	hc.Spec.Component = nil
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}
	if len(hc.Status.Outputs) != 0 {
		t.Errorf("expected outputs cleared, got %v", hc.Status.Outputs)
	}
}

func TestReleaseSubReconciler_noOutputsForOtherComponents(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hc.Status.Outputs) != 0 {
		t.Errorf("expected no outputs, got %v", hc.Status.Outputs)
	}
}
