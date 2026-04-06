// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// newTestReleaseSubReconciler builds a ReleaseSubReconciler backed by a MockClient
// and an in-memory fake k8s client (no Helm SDK required).
func newTestReleaseSubReconciler(scheme *runtime.Scheme, mockClient *MockClient) *ReleaseSubReconciler {
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	return &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mockClient,
	}
}

func newHelmCapsule(name, namespace string) *operatorv1alpha1.HelmCapsule {
	return &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart: operatorv1alpha1.HelmChartSpec{
				Repo:    "oci://ghcr.io/hashgraph",
				Chart:   "hedera-mirror",
				Version: "0.129.1",
			},
		},
	}
}

func TestReleaseSubReconciler_installsNewRelease(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	r := newTestReleaseSubReconciler(scheme, mock)

	hc := newHelmCapsule("mirror", "test-ns")

	result, err := r.Reconcile(context.Background(), hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	info, _ := mock.GetRelease("mirror", "test-ns")
	if info == nil {
		t.Fatal("expected release to exist after reconcile")
	}
	if info.Version != 1 {
		t.Errorf("expected revision 1, got %d", info.Version)
	}
	if info.Status != "deployed" {
		t.Errorf("expected status=deployed, got %s", info.Status)
	}

	// Status fields written back to hc.
	if hc.Status.ReleaseVersion != 1 {
		t.Errorf("expected hc.Status.ReleaseVersion=1, got %d", hc.Status.ReleaseVersion)
	}
	expected := "hedera-mirror-0.129.1"
	if hc.Status.InstalledChart != expected {
		t.Errorf("expected InstalledChart=%s, got %s", expected, hc.Status.InstalledChart)
	}
}

func TestReleaseSubReconciler_noOpWhenHashUnchanged(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	r := newTestReleaseSubReconciler(scheme, mock)

	hc := newHelmCapsule("mirror", "test-ns")

	// First reconcile — installs and records hash.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	if hc.Status.ValuesHash == "" {
		t.Fatal("expected ValuesHash to be set after first reconcile")
	}

	// Second reconcile with same spec — hash matches, no helm call.
	callsBefore := mock.releases["test-ns/mirror"].Version
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("no-op reconcile error: %v", err)
	}
	if mock.releases["test-ns/mirror"].Version != callsBefore {
		t.Error("expected no Helm upgrade for identical spec (hash unchanged)")
	}
}

func TestReleaseSubReconciler_upgradesOnChartVersionChange(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	r := newTestReleaseSubReconciler(scheme, mock)

	hc := newHelmCapsule("mirror", "test-ns")

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	firstHash := hc.Status.ValuesHash

	// Bump chart version — hash changes, upgrade must fire.
	hc.Spec.Chart.Version = "0.130.0"
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("upgrade reconcile error: %v", err)
	}

	if hc.Status.ValuesHash == firstHash {
		t.Error("expected ValuesHash to change after chart version bump")
	}
	info, _ := mock.GetRelease("mirror", "test-ns")
	if info.Version != 2 {
		t.Errorf("expected revision 2 after chart version bump, got %d", info.Version)
	}
}

func TestReleaseSubReconciler_customTimeout(t *testing.T) {
	scheme := newTestScheme()

	var capturedTimeout time.Duration
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
		capturedTimeout = opts.Timeout
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	r := newTestReleaseSubReconciler(scheme, mock)
	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.Timeout = &metav1.Duration{Duration: 5 * time.Minute}

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTimeout != 5*time.Minute {
		t.Errorf("expected timeout=5m, got %v", capturedTimeout)
	}
}

func TestReleaseSubReconciler_defaultTimeout(t *testing.T) {
	scheme := newTestScheme()

	var capturedTimeout time.Duration
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
		capturedTimeout = opts.Timeout
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	r := newTestReleaseSubReconciler(scheme, mock)
	hc := newHelmCapsule("mirror", "test-ns") // no Timeout set

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTimeout != defaultTimeout {
		t.Errorf("expected default timeout=%v, got %v", defaultTimeout, capturedTimeout)
	}
}

func TestReleaseSubReconciler_upgradesOnValuesChange(t *testing.T) {
	scheme := newTestScheme()

	// Create a ConfigMap that ValuesFrom references.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "vals", Namespace: "test-ns"},
		Data:       map[string]string{"values.yaml": `{"replicaCount": 1}`},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.ValuesFrom = []operatorv1alpha1.HelmValuesReference{
		{Kind: "ConfigMap", Name: "vals"},
	}

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	firstHash := hc.Status.ValuesHash

	// Mutate the ConfigMap value — hash must change, upgrade must fire.
	cm.Data["values.yaml"] = `{"replicaCount": 3}`
	if err := c.Update(context.Background(), cm); err != nil {
		t.Fatalf("updating configmap: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}

	if hc.Status.ValuesHash == firstHash {
		t.Error("expected ValuesHash to change after ConfigMap values changed")
	}
	info, _ := mock.GetRelease("mirror", "test-ns")
	if info.Version != 2 {
		t.Errorf("expected revision 2 after values change, got %d", info.Version)
	}
}

func TestReleaseSubReconciler_propagatesHelmError(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, _, _ string, _ operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		return nil, errors.New("simulated helm failure")
	}

	r := newTestReleaseSubReconciler(scheme, mock)
	hc := newHelmCapsule("mirror", "test-ns")

	_, err := r.Reconcile(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
}

// ─── MockClient unit tests ────────────────────────────────────────────────────

func TestMockClient_installAndGet(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()
	chart := operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"}

	info, err := mock.InstallOrUpgrade(ctx, "my-release", "my-ns", chart, nil, ReleaseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Version != 1 {
		t.Errorf("expected version=1, got %d", info.Version)
	}

	got, err := mock.GetRelease("my-release", "my-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected release to exist")
	}
}

func TestMockClient_getNotFound(t *testing.T) {
	mock := NewMockClient()
	info, err := mock.GetRelease("nonexistent", "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for non-existent release")
	}
}

func TestMockClient_uninstallRemovesRelease(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()
	chart := operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"}

	_, _ = mock.InstallOrUpgrade(ctx, "r", "ns", chart, nil, ReleaseOptions{})
	_ = mock.Uninstall(ctx, "r", "ns")

	info, _ := mock.GetRelease("r", "ns")
	if info != nil {
		t.Error("expected release to be removed after uninstall")
	}
}
