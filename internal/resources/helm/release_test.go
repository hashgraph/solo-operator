// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// newTestReleaseSubReconciler builds a ReleaseSubReconciler backed by a MockClient
// and an in-memory fake k8s client (no Helm SDK required).
// Pass any HelmCapsule objects that will be reconciled so the fake client can serve
// Get and Status().Update() calls inside the reconciler.
func newTestReleaseSubReconciler(scheme *runtime.Scheme, mockClient *MockClient, objs ...client.Object) *ReleaseSubReconciler {
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).
		Build()
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
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

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
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

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
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

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

	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.Timeout = &metav1.Duration{Duration: 5 * time.Minute}
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTimeout != 5*time.Minute {
		t.Errorf("expected timeout=5m, got %v", capturedTimeout)
	}
}

func TestReleaseSubReconciler_DefaultTimeout(t *testing.T) {
	scheme := newTestScheme()

	var capturedTimeout time.Duration
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
		capturedTimeout = opts.Timeout
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("mirror", "test-ns") // no Timeout set
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTimeout != DefaultTimeout {
		t.Errorf("expected default timeout=%v, got %v", DefaultTimeout, capturedTimeout)
	}
}

func TestReleaseSubReconciler_upgradesOnValuesChange(t *testing.T) {
	scheme := newTestScheme()

	// Create a ConfigMap that ValuesFrom references.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "vals", Namespace: "test-ns"},
		Data:       map[string]string{"values.yaml": `{"replicaCount": 1}`},
	}
	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.ValuesFrom = []operatorv1alpha1.HelmValuesReference{
		{Kind: "ConfigMap", Name: "vals"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, hc).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()

	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
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

	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	_, err := r.Reconcile(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "mirror") {
		t.Errorf("expected error to contain release name, got: %v", err)
	}
}

func TestReleaseSubReconciler_conflictWhenSameReleaseName(t *testing.T) {
	scheme := newTestScheme()

	// Two HelmCapsules in the same namespace both resolve to "shared-release".
	hc1 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-a", Namespace: "test-ns", UID: "uid-a"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "shared-release",
		},
	}
	hc2 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-b", Namespace: "test-ns", UID: "uid-b"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "shared-release",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc1, hc2).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()
	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	// Both capsules see each other as conflicting — neither can proceed.
	for _, hc := range []*operatorv1alpha1.HelmCapsule{hc1, hc2} {
		_, err := r.Reconcile(context.Background(), hc)
		if err == nil {
			t.Fatalf("capsule %s: expected conflict error, got nil", hc.Name)
		}
		if reason, _ := errx.ReasonOf(err); reason != ReasonReleaseNameConflict {
			t.Errorf("capsule %s: expected reason %s, got: %v", hc.Name, ReasonReleaseNameConflict, err)
		}
	}
}

func TestReleaseSubReconciler_conflictWhenNameDefaultsToObjectName(t *testing.T) {
	scheme := newTestScheme()

	// capsule-a uses explicit releaseName="mirror"; capsule-b defaults to its own object name "mirror".
	hc1 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-a", Namespace: "test-ns", UID: "uid-a"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "mirror",
		},
	}
	hc2 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "mirror", Namespace: "test-ns", UID: "uid-b"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			// ReleaseName omitted — defaults to "mirror"
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc1, hc2).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()
	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	for _, hc := range []*operatorv1alpha1.HelmCapsule{hc1, hc2} {
		_, err := r.Reconcile(context.Background(), hc)
		if err == nil {
			t.Fatalf("capsule %s: expected conflict error, got nil", hc.Name)
		}
		if reason, _ := errx.ReasonOf(err); reason != ReasonReleaseNameConflict {
			t.Errorf("capsule %s: expected reason %s, got: %v", hc.Name, ReasonReleaseNameConflict, err)
		}
	}
}

func TestReleaseSubReconciler_noConflictAcrossNamespaces(t *testing.T) {
	scheme := newTestScheme()

	// Two HelmCapsules with the same release name but in different namespaces — no conflict.
	hc1 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-a", Namespace: "ns-1", UID: "uid-a"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "mirror",
		},
	}
	hc2 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-b", Namespace: "ns-2", UID: "uid-b"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "mirror",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc1, hc2).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()
	mock := NewMockClient()

	for _, hc := range []*operatorv1alpha1.HelmCapsule{hc1, hc2} {
		r := &ReleaseSubReconciler{
			BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
			helmClient:        mock,
		}
		if _, err := r.Reconcile(context.Background(), hc); err != nil {
			t.Errorf("capsule %s/%s: unexpected error: %v", hc.Namespace, hc.Name, err)
		}
	}
}

func TestReleaseSubReconciler_terminatingCapsuleDoesNotBlockPeer(t *testing.T) {
	scheme := newTestScheme()
	now := metav1.Now()

	// hc1 is being deleted (has DeletionTimestamp).
	hc1 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name: "capsule-a", Namespace: "test-ns", UID: "uid-a",
			DeletionTimestamp: &now, Finalizers: []string{"test"},
		},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "shared-release",
		},
	}
	hc2 := &operatorv1alpha1.HelmCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "capsule-b", Namespace: "test-ns", UID: "uid-b"},
		Spec: operatorv1alpha1.HelmCapsuleSpec{
			Chart:       operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
			ReleaseName: "shared-release",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc1, hc2).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()
	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	// hc2 should not be blocked by the terminating hc1.
	if _, err := r.Reconcile(context.Background(), hc2); err != nil {
		t.Errorf("expected no conflict against terminating capsule, got: %v", err)
	}
}

func TestReleaseSubReconciler_listFailurePropagatesError(t *testing.T) {
	scheme := newTestScheme()

	// Build a client with no HelmCapsule type registered so List returns an error.
	c := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	mock := NewMockClient()
	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	hc := newHelmCapsule("mirror", "test-ns")
	_, err := r.Reconcile(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error when list fails")
	}
	// A transient list failure must NOT be misreported as a release name conflict.
	if reason, _ := errx.ReasonOf(err); reason == ReasonReleaseNameConflict {
		t.Errorf("transient list error must not be reported as a release name conflict, got: %v", err)
	}
}

func TestReleaseSubReconciler_usesCustomReleaseName(t *testing.T) {
	scheme := newTestScheme()

	var capturedReleaseName string
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		capturedReleaseName = releaseName
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("my-capsule", "test-ns")
	hc.Spec.ReleaseName = "custom-release"
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReleaseName != "custom-release" {
		t.Errorf("expected releaseName=custom-release, got %s", capturedReleaseName)
	}
}

func TestReleaseSubReconciler_defaultsReleaseNameToObjectName(t *testing.T) {
	scheme := newTestScheme()

	var capturedReleaseName string
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		capturedReleaseName = releaseName
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("my-capsule", "test-ns") // no ReleaseName set
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReleaseName != "my-capsule" {
		t.Errorf("expected releaseName=my-capsule, got %s", capturedReleaseName)
	}
}

// ─── ReleaseStatus surfacing ──────────────────────────────────────────────────

func TestReleaseSubReconciler_setsReleaseStatusAfterInstall(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc.Status.ReleaseStatus != "deployed" {
		t.Errorf("expected ReleaseStatus=deployed, got %q", hc.Status.ReleaseStatus)
	}
}

// ─── Idle-path requeue for periodic drift detection ───────────────────────────

func TestReleaseSubReconciler_idlePathRequeuesForPeriodicCheck(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	// First reconcile installs and records hash.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	// Second reconcile: hash matches, release deployed — should return RequeueAfter for drift detection.
	result, err := r.Reconcile(context.Background(), hc)
	if err != nil {
		t.Fatalf("idle reconcile error: %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Error("expected RequeueAfter > 0 on idle path for periodic drift detection")
	}
}

// ─── Non-deployed release recovery ───────────────────────────────────────────

func TestReleaseSubReconciler_nonDeployedStatusDoesNotSaveHash(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "pending-install", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc.Status.ValuesHash != "" {
		t.Error("ValuesHash must not be saved when Helm status is not 'deployed'")
	}
}

func TestReleaseSubReconciler_nonDeployedStatusRequeues(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "pending-install", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	result, err := r.Reconcile(context.Background(), hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Error("expected RequeueAfter > 0 when Helm release is not yet deployed")
	}
}

func TestReleaseSubReconciler_retriesWhenHashMatchesButNotDeployed(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	// First reconcile: install and save hash.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	installCalls := mock.releases["test-ns/mirror"].Version

	// Simulate the release becoming failed externally (same hash, but Helm status = failed).
	mock.releases["test-ns/mirror"].Status = "failed"

	// Second reconcile: hash still matches, but release is failed — must retry the upgrade.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("retry reconcile error: %v", err)
	}
	if mock.releases["test-ns/mirror"].Version == installCalls {
		t.Error("expected Helm upgrade to fire when release is failed, even if hash matches")
	}
}

// ─── Chart credentials ────────────────────────────────────────────────────────

func TestReleaseSubReconciler_loadsCredentialsFromSecret(t *testing.T) {
	scheme := newTestScheme()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "chart-creds", Namespace: "test-ns"},
		Data: map[string][]byte{
			"username": []byte("alice"),
			"password": []byte("s3cr3t"),
		},
	}
	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.Chart.CredentialsRef = &corev1.LocalObjectReference{Name: "chart-creds"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, hc).WithStatusSubresource(&operatorv1alpha1.HelmCapsule{}).Build()

	var capturedCreds *ChartCredentials
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, opts ReleaseOptions) (*ReleaseInfo, error) {
		capturedCreds = opts.Credentials
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	r := &ReleaseSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HelmCapsule](c, scheme, "helm-release"),
		helmClient:        mock,
	}

	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCreds == nil {
		t.Fatal("expected credentials to be loaded and passed to InstallOrUpgrade")
	}
	if capturedCreds.Username != "alice" || capturedCreds.Password != "s3cr3t" {
		t.Errorf("unexpected credentials: %+v", capturedCreds)
	}
}

func TestReleaseSubReconciler_missingCredentialsSecretErrors(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	hc.Spec.Chart.CredentialsRef = &corev1.LocalObjectReference{Name: "nonexistent"}
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	_, err := r.Reconcile(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error for missing credentials secret")
	}
}

// ─── GetRelease error handling ────────────────────────────────────────────────

// When GetRelease errors inside the hash-match block, the error must be surfaced
// rather than silently falling through to InstallOrUpgrade.
func TestReleaseSubReconciler_getReleasesErrorIsReturned(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	// First reconcile: install succeeds and hash is recorded.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}
	firstVersion := mock.releases["test-ns/mirror"].Version

	// Inject an error into GetRelease.
	mock.GetReleaseFunc = func(_, _ string) (*ReleaseInfo, error) {
		return nil, errors.New("api server timeout")
	}

	// Second reconcile: hash matches, GetRelease errors — must return the error, not upgrade.
	_, err := r.Reconcile(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error when GetRelease fails during hash-match check")
	}
	if mock.releases["test-ns/mirror"].Version != firstVersion {
		t.Error("InstallOrUpgrade must not be called when GetRelease returns an error")
	}
}

// ─── Failed release recovery ──────────────────────────────────────────────────

// When InstallOrUpgrade fails, ValuesHash must not be persisted so the next
// reconcile retries the operation rather than treating the spec as up-to-date.
func TestReleaseSubReconciler_helmErrorDoesNotSaveHash(t *testing.T) {
	scheme := newTestScheme()
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, _, _ string, _ operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		return nil, errors.New("helm install failed: timeout waiting for pods")
	}

	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	if _, err := r.Reconcile(context.Background(), hc); err == nil {
		t.Fatal("expected error when InstallOrUpgrade fails")
	}

	if hc.Status.ValuesHash != "" {
		t.Error("ValuesHash must not be saved when InstallOrUpgrade returns an error")
	}
}

// After a failed reconcile (hash not saved), the next reconcile must retry Helm.
func TestReleaseSubReconciler_retriesAfterHelmError(t *testing.T) {
	scheme := newTestScheme()

	callCount := 0
	mock := NewMockClient()
	mock.InstallOrUpgradeFunc = func(_ context.Context, releaseName, namespace string, chart operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ ReleaseOptions) (*ReleaseInfo, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("transient failure")
		}
		return &ReleaseInfo{Name: releaseName, Namespace: namespace, Version: 1, Status: "deployed", ChartString: chart.Chart + "-" + chart.Version}, nil
	}

	hc := newHelmCapsule("mirror", "test-ns")
	r := newTestReleaseSubReconciler(scheme, mock, hc)

	// First reconcile fails.
	if _, err := r.Reconcile(context.Background(), hc); err == nil {
		t.Fatal("expected error on first reconcile")
	}

	// Second reconcile: hash still empty → must retry.
	if _, err := r.Reconcile(context.Background(), hc); err != nil {
		t.Fatalf("second reconcile (retry) error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected exactly 2 InstallOrUpgrade calls, got %d", callCount)
	}
	if hc.Status.ValuesHash == "" {
		t.Error("expected ValuesHash to be saved after successful retry")
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
	_ = mock.Uninstall(ctx, "r", "ns", UninstallOptions{})

	info, _ := mock.GetRelease("r", "ns")
	if info != nil {
		t.Error("expected release to be removed after uninstall")
	}
}
