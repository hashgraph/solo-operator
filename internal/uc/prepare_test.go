// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joomcode/errorx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8stesting "k8s.io/client-go/testing"

	"github.com/automa-saga/errx"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const validComponentsYAML = `schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc"
    registries:
      - image: "ghcr.io/hashgraph/consensus-node:0.75.0"
  recordStreamUploader:
    enabled: true
    version: "0.43.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:def"
    registries:
      - image: "ghcr.io/hashgraph/cheetah:0.43.0"
`

// markerCreatedAt is the fixed marker mtime used so the minted operationId is
// deterministic across runs.
var markerCreatedAt = time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

const wantOperationID = wantConsensusNode + "-upgrade-0.75.0-20260413T120000Z"

// writePackage lays out a marker directory that mirrors the consensus node's
// extracted package: the marker file and a manifests/ subdir in the same root.
// The marker's mtime is pinned so the minted operationId is stable.
func writePackage(t *testing.T, componentsYAML string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, manifestsSubdir), 0o755); err != nil {
		t.Fatalf("mkdir manifests: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, manifestsSubdir, componentsManifestFile), []byte(componentsYAML), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	marker := filepath.Join(root, MarkerExecuteImmediate)
	if err := os.WriteFile(marker, []byte("PREPARE_UPGRADE"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.Chtimes(marker, markerCreatedAt, markerCreatedAt); err != nil {
		t.Fatalf("chtimes marker: %v", err)
	}
	return root
}

func testConfig(markerRoot, crPersist string) Config {
	return Config{
		Mode:          ModeClusterOnly,
		NodeID:        0,
		PodName:       wantConsensusNode + "-0",
		Orbit:         "my-orbit",
		Namespace:     "solo",
		UpgradePath:   markerRoot,
		CRPersistPath: crPersist,
	}
}

func newFakeFactory(t *testing.T, cfg Config) *Factory {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	f := NewFactory(cfg, client)
	if err := f.startInformers(t.Context()); err != nil {
		t.Fatalf("start informers: %v", err)
	}
	return f
}

// waitForCached blocks until count instances of gvr/operationID are cached (async watch delivery).
func waitForCached(t *testing.T, f *Factory, gvr schema.GroupVersionResource, operationID string, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if items, err := f.listCached(gvr, operationID); err == nil && len(items) >= count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %s operationId=%s to appear in informer cache", count, gvr.Resource, operationID)
}

func TestParseComponentsManifestValid(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	m, err := parseComponentsManifest(filepath.Join(root, manifestsSubdir, componentsManifestFile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.SchemaVersion != componentsSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", m.SchemaVersion, componentsSchemaVersion)
	}
	if m.Images.ConsensusNode.Version != "0.75.0" {
		t.Fatalf("consensusNode.version = %q, want 0.75.0", m.Images.ConsensusNode.Version)
	}
	if m.Images.RecordStreamUploader == nil || m.Images.RecordStreamUploader.Version != "0.43.0" {
		t.Fatalf("recordStreamUploader not parsed: %+v", m.Images.RecordStreamUploader)
	}
}

func TestParseComponentsManifestSymlinkRejected(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	path := filepath.Join(root, manifestsSubdir, componentsManifestFile)
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove original: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := parseComponentsManifest(path)
	if err == nil {
		t.Fatal("expected error for symlinked manifest, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

func TestParseComponentsManifestLenientUnknownField(t *testing.T) {
	// An additive unknown field within a known schemaVersion must be ignored, not
	// rejected (HIP-1494 backwards-compatibility).
	yamlWithExtra := validComponentsYAML + "futureTopLevelField: ignored\n"
	root := writePackage(t, yamlWithExtra)
	if _, err := parseComponentsManifest(filepath.Join(root, manifestsSubdir, componentsManifestFile)); err != nil {
		t.Fatalf("lenient parse should ignore unknown field, got: %v", err)
	}
}

func TestParseComponentsManifestInvalid(t *testing.T) {
	cases := map[string]string{
		"unknown schemaVersion": "schemaVersion: 99\nimages:\n  consensusNode:\n    version: \"0.75.0\"\n    registries:\n      - image: x\n",
		"missing schemaVersion": "images:\n  consensusNode:\n    version: \"0.75.0\"\n    registries:\n      - image: x\n",
		"missing cn version":    "schemaVersion: 1\nimages:\n  consensusNode:\n    registries:\n      - image: x\n",
		"malformed yaml":        "schemaVersion: 1\nimages: : :\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := writePackage(t, body)
			_, err := parseComponentsManifest(filepath.Join(root, manifestsSubdir, componentsManifestFile))
			if !errorx.IsOfType(err, errManifestInvalid) {
				t.Fatalf("want errManifestInvalid, got %v", err)
			}
			if reason, ok := errx.ReasonOf(err); !ok || reason != "ManifestInvalid" {
				t.Fatalf("want reason ManifestInvalid, got %q (ok=%v)", reason, ok)
			}
		})
	}
}

// TestMintOperationIDRejectsInvalidName verifies mintOperationID rejects versions
// whose derived operationId/CR name would be invalid in Kubernetes — the single
// mint point enforces both the label-value length limit (matching the CRD's
// MaxLength=63) and DNS-1123 name validity (accounting for the "-<step>" suffix) —
// rather than failing opaquely at CR Create time.
func TestMintOperationIDRejectsInvalidName(t *testing.T) {
	// '_' is legal per sanity.ValidateOperationID but invalid in a metadata.name.
	if _, err := mintOperationID("0.75.0_rc1", markerCreatedAt, 0); !errorx.IsOfType(err, errManifestInvalid) {
		t.Errorf("expected errManifestInvalid, got %v", err)
	}
	// A version long enough to push the operationId past the 63-char label/CRD limit.
	if _, err := mintOperationID(strings.Repeat("9", 45), markerCreatedAt, 0); !errorx.IsOfType(err, errManifestInvalid) {
		t.Errorf("expected errManifestInvalid, got %v", err)
	}
	// A clean semver version still mints the expected id.
	id, err := mintOperationID("0.75.0", markerCreatedAt, 0)
	if err != nil {
		t.Fatalf("clean version: unexpected error %v", err)
	}
	if id != wantOperationID {
		t.Fatalf("minted id = %q, want %q", id, wantOperationID)
	}
}

func TestHandleExecuteImmediateCreatesPrepareCR(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	crDir := t.TempDir()
	cfg := testConfig(root, crDir)
	f := newFakeFactory(t, cfg)

	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("HandleExecuteImmediate: %v", err)
	}

	cr := getPrepare(t, f, wantOperationID)

	if got := cr.GetName(); got != wantConsensusNode+"-upgrade-0.75.0-20260413t120000z-prepare" {
		t.Fatalf("name = %q", got)
	}
	if got := cr.GetNamespace(); got != "solo" {
		t.Fatalf("namespace = %q", got)
	}
	labels := cr.GetLabels()
	if labels[operatorv1alpha1.LabelOperationID] != wantOperationID {
		t.Fatalf("operation-id label = %q", labels[operatorv1alpha1.LabelOperationID])
	}
	if labels[operatorv1alpha1.LabelStep] != operatorv1alpha1.StepPrepare {
		t.Fatalf("step label = %q", labels[operatorv1alpha1.LabelStep])
	}
	if cr.GetAnnotations()[annotationCreatedBy] != createdByUC {
		t.Fatalf("created-by annotation = %q", cr.GetAnnotations()[annotationCreatedBy])
	}

	orbit, _, _ := unstructured.NestedString(cr.Object, "spec", "orbit")
	if orbit != "my-orbit" {
		t.Fatalf("spec.orbit = %q", orbit)
	}
	opID, _, _ := unstructured.NestedString(cr.Object, "spec", "operationId")
	if opID != wantOperationID {
		t.Fatalf("spec.operationId = %q", opID)
	}
	cnVersion, _, _ := unstructured.NestedString(cr.Object, "spec", "images", "consensusNode", "version")
	if cnVersion != "0.75.0" {
		t.Fatalf("spec.images.consensusNode.version = %q", cnVersion)
	}

	// persist-before-send: the file is removed after a successful create.
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up after create: %v", rest)
	}
}

func TestHandleExecuteImmediateManifestInvalidCreatesNoCR(t *testing.T) {
	root := writePackage(t, "schemaVersion: 99\nimages:\n  consensusNode:\n    version: \"0.75.0\"\n    registries:\n      - image: x\n")
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(root, crDir))

	// ManifestInvalid is terminal-but-handled: the handler returns nil (no retry)
	// and creates no CR / leaves nothing persisted.
	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("want nil on ManifestInvalid, got %v", err)
	}
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no CR created, got %d", len(list.Items))
	}
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("expected nothing persisted, got %v", rest)
	}
}

func TestHandleExecuteImmediateIsIdempotent(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	for i := 0; i < 3; i++ {
		if err := f.HandleExecuteImmediate(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("re-dispatch created %d CRs, want exactly 1", len(list.Items))
	}
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR not cleaned up: %v", rest)
	}
}

func TestHandleExecuteImmediateLeavesPersistedOnCreateError(t *testing.T) {
	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	// Inject a non-AlreadyExists Create failure: the persisted CR must remain on
	// disk for the restart-recovery flow (#998) to re-send.
	fakeClient := f.client.(*dynamicfake.FakeDynamicClient)
	fakeClient.PrependReactor("create", "networkupgradeprepares", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	})

	err := f.HandleExecuteImmediate(context.Background())
	if err == nil {
		t.Fatal("expected create error to propagate")
	}
	rest, _ := f.store.List()
	if len(rest) != 1 {
		t.Fatalf("expected 1 persisted CR left for recovery, got %d", len(rest))
	}
	if rest[0].OperationID != wantOperationID || rest[0].Step != operatorv1alpha1.StepPrepare {
		t.Fatalf("persisted CR = %+v", rest[0])
	}
}

func TestHandleExecuteImmediateAdmissionRejectionBacksOffThenRetries(t *testing.T) {
	// A ValidatingAdmissionPolicy Deny surfaces as an Invalid (422) Create error
	// (#1133). The handler returns nil (no retryable error), creates no CR, and
	// cleans up the persisted copy — then backs off: ticks within the cooldown must
	// NOT re-attempt the create (no per-tick API/log spam), but once the cooldown
	// lapses it retries, so a fixed VAP self-heals without a restart.
	gk := schema.GroupKind{Group: operatorv1alpha1.GroupVersion.Group, Kind: kindNetworkUpgradePrepare}
	injErr := k8serrors.NewInvalid(gk, "x", field.ErrorList{
		field.Invalid(field.NewPath("metadata", "labels"), "", "step label missing or inconsistent"),
	})

	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	var creates int
	fakeClient := f.client.(*dynamicfake.FakeDynamicClient)
	fakeClient.PrependReactor("create", "networkupgradeprepares", func(k8stesting.Action) (bool, runtime.Object, error) {
		creates++
		return true, nil, injErr
	})

	// First tick: rejection handled — nil error, nothing persisted, no CR created.
	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("admission rejection must return nil, got %v", err)
	}
	if rest, _ := f.store.List(); len(rest) != 0 {
		t.Fatalf("persisted CR must be cleaned up after admission rejection, got %d", len(rest))
	}
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("no CR should be created on admission rejection, got %d", len(list.Items))
	}

	// Ticks within the cooldown window must short-circuit: no further Create calls.
	for i := 0; i < 3; i++ {
		if err := f.HandleExecuteImmediate(context.Background()); err != nil {
			t.Fatalf("cooldown tick %d must return nil, got %v", i, err)
		}
	}
	if creates != 1 {
		t.Fatalf("Create must be attempted exactly once during the cooldown, got %d", creates)
	}

	// Simulate the cooldown lapsing (e.g. after a mis-configured VAP is fixed): the
	// UC must retry rather than stay stuck until a restart.
	f.mu.Lock()
	f.rejected[rejectKey{wantOperationID, operatorv1alpha1.StepPrepare}] = time.Now().Add(-2 * admissionRejectRetryCooldown)
	f.mu.Unlock()

	if err := f.HandleExecuteImmediate(context.Background()); err != nil {
		t.Fatalf("post-cooldown retry must return nil, got %v", err)
	}
	if creates != 2 {
		t.Fatalf("Create must be retried after the cooldown lapses, got %d attempts", creates)
	}
}

func TestHandleExecuteImmediateForbiddenIsRetryable(t *testing.T) {
	// A Forbidden (403) Create error must NOT be treated as terminal: it is also
	// how the API server reports an RBAC/authorization gap, which is transient
	// (fixable by granting the UC ServiceAccount create permission). The handler
	// must surface a retryable error and LEAVE the persisted CR on disk so
	// restart-recovery can re-send it once permissions are fixed — never silently
	// drop it (#1133 review).
	gr := networkUpgradePrepareGVR.GroupResource()
	injErr := k8serrors.NewForbidden(gr, "x", errors.New("forbidden: UC ServiceAccount cannot create resource"))

	root := writePackage(t, validComponentsYAML)
	f := newFakeFactory(t, testConfig(root, t.TempDir()))

	fakeClient := f.client.(*dynamicfake.FakeDynamicClient)
	fakeClient.PrependReactor("create", "networkupgradeprepares", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, injErr
	})

	if err := f.HandleExecuteImmediate(context.Background()); err == nil {
		t.Fatal("Forbidden must be retryable (non-nil error), got nil")
	}
	if rest, _ := f.store.List(); len(rest) != 1 {
		t.Fatalf("persisted CR must be retained after Forbidden for recovery, got %d", len(rest))
	}
}

func getPrepare(t *testing.T, f *Factory, operationID string) *unstructured.Unstructured {
	t.Helper()
	cr, err := f.client.Resource(networkUpgradePrepareGVR).Namespace("solo").
		Get(context.Background(), prepareCRName(operationID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get prepare CR: %v", err)
	}
	return cr
}
