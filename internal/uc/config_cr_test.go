// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const testOperationID = "upgrade-0.75.0-20260413t120000z"

// newProxyWithFakeClient builds a ProvisionerProxy wired to a fake dynamic
// client pre-loaded with the given objects.
func newProxyWithFakeClient(t *testing.T, upgradePath string, objs ...runtime.Object) *ProvisionerProxy {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	client := fake.NewSimpleDynamicClient(scheme, objs...)
	return NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:            "solo",
		NodeID:               0,
		UpgradePath:          upgradePath,
		Orbit:                "test-orbit",
		ConfigCRPollInterval: 5 * time.Millisecond,
	}, client)
}

// writeConfigFile writes a file under the config subdirectory of root.
func writeConfigFile(t *testing.T, root, filename, content string) {
	t.Helper()
	dir := filepath.Join(root, configSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}

// writeRootFile writes a file at the package root (not data/config/).
func writeRootFile(t *testing.T, root, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
}

// writeBlockNodesFile writes a per-node block-node config fixture under
// block-nodes/config/block-nodes-<nodeID>.json.
func writeBlockNodesFile(t *testing.T, root string, nodeID int, content string) {
	t.Helper()
	dir := filepath.Join(root, blockNodesSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	name := fmt.Sprintf("block-nodes-%d.json", nodeID)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write block-nodes file: %v", err)
	}
}

func TestDeployBlockNodesConfigCR_SelectsNodeVerbatim(t *testing.T) {
	root := t.TempDir()
	writeBlockNodesFile(t, root, 0, `{"nodes":[{"address":"block-node-0","port":8080}],"blockItemBatchSize":256}`)
	want := `{"nodes":[{"address":"block-node-1","port":8080}],"blockItemBatchSize":256}`
	writeBlockNodesFile(t, root, 1, want)
	writeBlockNodesFile(t, root, 2, `{"nodes":[{"address":"block-node-2","port":8080}],"blockItemBatchSize":256}`)

	proxy := newProxyWithFakeClient(t, root)
	proxy.cfg.NodeID = 1
	proxy.cfg.CapsuleName = "node1"

	ref, err := proxy.deployBlockNodesConfigCR(context.Background(), testOperationID)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if ref == nil {
		t.Fatal("expected a ref, got nil")
	}
	wantName := configCRName(testOperationID, blockNodesKindEntry)
	if ref.name != wantName {
		t.Errorf("cr name = %q, want %q", ref.name, wantName)
	}

	fc := proxy.client.(*fake.FakeDynamicClient)
	obj, err := fc.Resource(blockNodesConfigGVR).Namespace("solo").Get(context.Background(), wantName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get CR: %v", err)
	}
	got, _, _ := unstructured.NestedString(obj.Object, "spec", "content")
	if got != want {
		t.Errorf("spec.content = %q, want node 1 file verbatim %q", got, want)
	}
	if orbit, _, _ := unstructured.NestedString(obj.Object, "spec", "orbit"); orbit != "test-orbit" {
		t.Errorf("spec.orbit = %q, want test-orbit", orbit)
	}
}

func TestDeployBlockNodesConfigCR_SymlinkRejected(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	dir := filepath.Join(root, blockNodesSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "block-nodes-0.json")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	proxy := newProxyWithFakeClient(t, root)
	proxy.cfg.CapsuleName = "node0"

	_, err := proxy.deployBlockNodesConfigCR(context.Background(), testOperationID)
	if err == nil {
		t.Fatal("expected error for symlinked block-node config, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
	// HIP-1496: a symlinked config file is fatal, not retryable.
	if errorx.IsTemporary(err) {
		t.Error("symlinked config file error must be terminal (non-Temporary)")
	}
}

func TestDeployBlockNodesConfigCR_AbsentIsNoop(t *testing.T) {
	proxy := newProxyWithFakeClient(t, t.TempDir()) // no block-nodes/ dir
	proxy.cfg.CapsuleName = "node0"

	ref, err := proxy.deployBlockNodesConfigCR(context.Background(), testOperationID)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if ref != nil {
		t.Errorf("expected nil ref when the package has no block-node config, got %+v", ref)
	}
}

func TestDeployBlockNodesConfigCR_PerOperationDistinctCRs(t *testing.T) {
	root := t.TempDir()
	first := `{"nodes":[{"address":"old","port":8080}],"blockItemBatchSize":256}`
	writeBlockNodesFile(t, root, 0, first)
	proxy := newProxyWithFakeClient(t, root)
	proxy.cfg.CapsuleName = "node0"

	ref1, err := proxy.deployBlockNodesConfigCR(context.Background(), testOperationID)
	if err != nil {
		t.Fatalf("first deploy: %v", err)
	}

	// A later upgrade ships new content under a new operationId: like every other
	// config CR, it must create a distinct, immutable CR (audit trail) rather than
	// overwriting the first.
	updated := `{"nodes":[{"address":"new","port":9090}],"blockItemBatchSize":512}`
	writeBlockNodesFile(t, root, 0, updated)
	ref2, err := proxy.deployBlockNodesConfigCR(context.Background(), "upgrade-0.76.0-20260501t120000z")
	if err != nil {
		t.Fatalf("second deploy: %v", err)
	}
	if ref1.name == ref2.name {
		t.Fatalf("expected distinct per-operation CR names, both were %q", ref1.name)
	}

	// Both CRs coexist, each carrying its own operation's content verbatim.
	fc := proxy.client.(*fake.FakeDynamicClient)
	assertContent := func(name, want string) {
		t.Helper()
		obj, err := fc.Resource(blockNodesConfigGVR).Namespace("solo").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get CR %s: %v", name, err)
		}
		if got, _, _ := unstructured.NestedString(obj.Object, "spec", "content"); got != want {
			t.Errorf("spec.content of %s = %q, want %q", name, got, want)
		}
	}
	assertContent(ref1.name, first)
	assertContent(ref2.name, updated)
}

// setValidCondition stamps a Valid condition on an unstructured object and
// updates it via the fake client's status subresource.
func setValidCondition(t *testing.T, client *fake.FakeDynamicClient, resGVR schema.GroupVersionResource, name, namespace, status string) {
	t.Helper()
	obj, err := client.Resource(resGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s %s: %v", resGVR.Resource, name, err)
	}
	cond := map[string]interface{}{
		"type":               "Valid",
		"status":             status,
		"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
		"reason":             "SchemaValid",
		"message":            "",
	}
	_ = unstructured.SetNestedSlice(obj.Object, []interface{}{cond}, "status", "conditions")
	if _, err := client.Resource(resGVR).Namespace(namespace).UpdateStatus(context.Background(), obj, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update status %s %s: %v", resGVR.Resource, name, err)
	}
}

// waitForCR polls the fake client until the named resource exists or the
// deadline (200 ms) elapses, avoiding timing-sensitive sleeps in tests.
func waitForCR(t *testing.T, client *fake.FakeDynamicClient, resGVR schema.GroupVersionResource, name, namespace string) {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err := client.Resource(resGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s/%s to be created", resGVR.Resource, name)
}

// ── readFileNoFollow ───────────────────────────────────────────────────────────

func TestReadFileNoFollow_RejectsSymlinkSwappedInAfterScan(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "application.properties", "[hedera]")
	path := filepath.Join(root, configSubdir, "application.properties")

	target := filepath.Join(t.TempDir(), "secret") // swapped in after the scan's Lstat check
	if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove original: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("swap in symlink: %v", err)
	}

	_, err := readFileNoFollow(path)
	if err == nil {
		t.Fatal("expected error for symlink swapped in after scan, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

// ── scanConfigFiles ────────────────────────────────────────────────────────────

func TestScanConfigFiles_empty(t *testing.T) {
	root := t.TempDir()
	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no files, got %d", len(got))
	}
}

func TestScanConfigFiles_configDir(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "application.properties", "[hedera]\nledger.id=0x00")
	writeConfigFile(t, root, "throttles.json", `{"type":"ThrottleDefinitions"}`)

	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 files, got %d", len(got))
	}
	byKind := make(map[string]scannedFile)
	for _, f := range got {
		byKind[f.entry.kind] = f
	}
	if _, ok := byKind["ApplicationProperties"]; !ok {
		t.Error("ApplicationProperties not found")
	}
	if _, ok := byKind["ThrottlesConfig"]; !ok {
		t.Error("ThrottlesConfig not found")
	}
}

func TestScanConfigFiles_rootFiles(t *testing.T) {
	root := t.TempDir()
	writeRootFile(t, root, "log4j2.xml", "<Configuration/>")
	writeRootFile(t, root, "settings.txt", "threadCount=4\n")

	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 files, got %d", len(got))
	}
	byKind := make(map[string]scannedFile)
	for _, f := range got {
		byKind[f.entry.kind] = f
	}
	if _, ok := byKind["Log4j2Config"]; !ok {
		t.Error("Log4j2Config not found")
	}
	if _, ok := byKind["NodeSettings"]; !ok {
		t.Error("NodeSettings not found")
	}
}

func TestScanConfigFiles_unrecognised(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "application.properties", "[hedera]")
	writeConfigFile(t, root, "unknown-file.txt", "ignore me")

	_, err := scanConfigFiles(root)
	if err == nil {
		t.Fatal("expected error for unrecognised config file, got nil")
	}
}

func TestScanConfigFiles_ignoredFileSkipped(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "application.properties", "[hedera]")
	// genesis-network.json is a HIP data/config/ file but operator-managed via
	// NetworkGenesis — it must be skipped, not treated as unrecognised, and must
	// not produce a scannedFile.
	writeConfigFile(t, root, "genesis-network.json", `{"nodes":[]}`)

	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 file (genesis-network.json skipped), got %d", len(got))
	}
	if got[0].entry.kind != "ApplicationProperties" {
		t.Errorf("kind: got %q, want ApplicationProperties", got[0].entry.kind)
	}
}

func TestScanConfigFiles_symlinkRejected(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "application.properties", "[hedera]")

	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	link := filepath.Join(root, configSubdir, "throttles.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := scanConfigFiles(root)
	if err == nil {
		t.Fatal("expected error for symlinked config file, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

func TestScanConfigFiles_rootFileSymlinkRejected(t *testing.T) {
	root := t.TempDir()

	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("top-secret"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	link := filepath.Join(root, "settings.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := scanConfigFiles(root)
	if err == nil {
		t.Fatal("expected error for symlinked root file, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileSymlink {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileSymlink)
	}
}

func TestScanConfigFiles_oversizedRejected(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "throttles.json", strings.Repeat("a", maxConfigFileSize+1))

	_, err := scanConfigFiles(root)
	if err == nil {
		t.Fatal("expected error for oversized config file, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileTooLarge {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileTooLarge)
	}
	// HIP-1496: a bad config file is fatal — the provisioner reports
	// DaemonResult=False rather than retrying, so it must NOT be Temporary.
	if errorx.IsTemporary(err) {
		t.Error("oversized config file error must be terminal (non-Temporary)")
	}
}

func TestScanConfigFiles_unrecognisedIsFatal(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, root, "not-a-known-config.json", "{}")

	_, err := scanConfigFiles(root)
	if err == nil {
		t.Fatal("expected error for unrecognised config filename, got nil")
	}
	if reason, _ := errx.ReasonOf(err); reason != ReasonConfigFileUnrecognised {
		t.Fatalf("reason: got %q, want %q", reason, ReasonConfigFileUnrecognised)
	}
	if errorx.IsTemporary(err) {
		t.Error("unrecognised config filename error must be terminal (non-Temporary)")
	}
}

func TestScanConfigFiles_allRecognised(t *testing.T) {
	root := t.TempDir()
	// Write all recognised data/config/ files.
	for name := range configFileKinds {
		if name == "log4j2.xml" || name == "settings.txt" {
			writeRootFile(t, root, name, "content")
		} else {
			writeConfigFile(t, root, name, "content")
		}
	}
	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(configFileKinds) {
		t.Fatalf("want %d files, got %d", len(configFileKinds), len(got))
	}
}

// ── buildConfigCR ──────────────────────────────────────────────────────────────

func TestBuildConfigCR_nameAndLabels(t *testing.T) {
	cfg := ProvisionerProxyConfig{Namespace: "solo", Orbit: "my-orbit"}
	path := filepath.Join(t.TempDir(), "log4j2.xml")
	if err := os.WriteFile(path, []byte("<Configuration/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	sf := scannedFile{absPath: path, entry: configFileKinds["log4j2.xml"]}
	cr, err := buildConfigCR(cfg, testOperationID, sf)
	if err != nil {
		t.Fatalf("buildConfigCR: %v", err)
	}

	wantName := testOperationID + "-log4j2"
	if cr.GetName() != wantName {
		t.Errorf("name = %q, want %q", cr.GetName(), wantName)
	}
	if cr.GetNamespace() != "solo" {
		t.Errorf("namespace = %q, want %q", cr.GetNamespace(), "solo")
	}
	if cr.GetKind() != "Log4j2Config" {
		t.Errorf("kind = %q, want Log4j2Config", cr.GetKind())
	}
	if cr.GetLabels()[operatorv1alpha1.LabelOperationID] != testOperationID {
		t.Errorf("LabelOperationID missing or wrong")
	}
	if cr.GetAnnotations()[annotationCreatedBy] != createdByUC {
		t.Errorf("annotationCreatedBy missing or wrong")
	}
	content, _, _ := unstructured.NestedString(cr.Object, "spec", "content")
	if content != "<Configuration/>" {
		t.Errorf("spec.content = %q, want %q", content, "<Configuration/>")
	}
	orbit, _, _ := unstructured.NestedString(cr.Object, "spec", "orbit")
	if orbit != "my-orbit" {
		t.Errorf("spec.orbit = %q, want my-orbit", orbit)
	}
	scope, _, _ := unstructured.NestedString(cr.Object, "spec", "scope")
	if scope != "node0" {
		t.Errorf("spec.scope = %q, want node0", scope)
	}
}

func TestBuildConfigCR_noOrbit(t *testing.T) {
	cfg := ProvisionerProxyConfig{Namespace: "solo", Orbit: ""}
	path := filepath.Join(t.TempDir(), "settings.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	sf := scannedFile{absPath: path, entry: configFileKinds["settings.txt"]}
	cr, err := buildConfigCR(cfg, testOperationID, sf)
	if err != nil {
		t.Fatalf("buildConfigCR: %v", err)
	}

	_, found, _ := unstructured.NestedString(cr.Object, "spec", "orbit")
	if found {
		t.Error("spec.orbit should be absent when Orbit is empty")
	}
}

// ── deployConfigCRs / handleExecute integration ────────────────────────────────

func TestDeployConfigCRs_noFiles(t *testing.T) {
	root := t.TempDir()
	p := newProxyWithFakeClient(t, root)
	ctx := context.Background()

	if err := p.deployConfigCRs(ctx, testOperationID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeployConfigCRs_happyPath(t *testing.T) {
	root := t.TempDir()
	writeRootFile(t, root, "log4j2.xml", "<Configuration/>")

	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	fakeClient := fake.NewSimpleDynamicClient(scheme)
	p := NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:            "solo",
		UpgradePath:          root,
		Orbit:                "test-orbit",
		ConfigCRPollInterval: 5 * time.Millisecond,
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["log4j2.xml"])

	// Deploy in a goroutine so we can set conditions concurrently.
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.deployConfigCRs(context.Background(), testOperationID)
	}()

	// Poll until the CR exists, then stamp Valid=True.
	waitForCR(t, fakeClient, log4j2ConfigGVR, crName, "solo")
	setValidCondition(t, fakeClient, log4j2ConfigGVR, crName, "solo", string(metav1.ConditionTrue))

	if err := <-errCh; err != nil {
		t.Fatalf("deployConfigCRs failed: %v", err)
	}
}

func TestDeployConfigCRs_validFalse(t *testing.T) {
	root := t.TempDir()
	writeRootFile(t, root, "settings.txt", "bad content")

	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}
	fakeClient := fake.NewSimpleDynamicClient(scheme)
	p := NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:            "solo",
		UpgradePath:          root,
		ConfigCRPollInterval: 5 * time.Millisecond,
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["settings.txt"])

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.deployConfigCRs(context.Background(), testOperationID)
	}()

	waitForCR(t, fakeClient, nodeSettingsGVR, crName, "solo")
	setValidCondition(t, fakeClient, nodeSettingsGVR, crName, "solo", string(metav1.ConditionFalse))

	err := <-errCh
	if err == nil {
		t.Fatal("expected error for Valid=False, got nil")
	}
}

func TestDeployConfigCRs_timeout(t *testing.T) {
	root := t.TempDir()
	writeRootFile(t, root, "log4j2.xml", "<Configuration/>")

	p := newProxyWithFakeClient(t, root)

	// Very short timeout — the condition will never be set so it times out.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := p.deployConfigCRs(ctx, testOperationID); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHandleExecute_configCRsThenPhaseAdvance(t *testing.T) {
	root := t.TempDir()
	writeRootFile(t, root, "log4j2.xml", "<Configuration/>")

	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}

	// Pre-create the Execute CR in the fake client so Patch can find it.
	executeCR := newTestExecuteCR(testOperationID, time.Now())
	_ = unstructured.SetNestedField(executeCR.Object, string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon),
		"status", "phase")
	executeCR.SetName("test-execute")
	executeCR.SetNamespace("solo")

	fakeClient := fake.NewSimpleDynamicClient(scheme, executeCR)
	p := NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:            "solo",
		UpgradePath:          root,
		Orbit:                "test-orbit",
		ConfigCRPollInterval: 5 * time.Millisecond,
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["log4j2.xml"])

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.handleExecute(context.Background(), executeCR)
	}()

	// Poll until the CR exists, then stamp Valid=True so the wait completes.
	waitForCR(t, fakeClient, log4j2ConfigGVR, crName, "solo")
	setValidCondition(t, fakeClient, log4j2ConfigGVR, crName, "solo", string(metav1.ConditionTrue))

	if err := <-errCh; err != nil {
		t.Fatalf("handleExecute failed: %v", err)
	}

	// Verify the Execute CR phase was advanced.
	got, err := fakeClient.Resource(networkUpgradeExecuteGVR).Namespace("solo").
		Get(context.Background(), "test-execute", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get execute CR: %v", err)
	}
	phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
	if phase != string(operatorv1alpha1.ExecutePhasePendingNodeUpgrade) {
		t.Errorf("status.phase = %q, want PendingNodeUpgrade", phase)
	}
}
