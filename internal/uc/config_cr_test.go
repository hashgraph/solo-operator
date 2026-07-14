// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		Namespace:   "solo",
		NodeID:      0,
		UpgradePath: upgradePath,
		Orbit:       "test-orbit",
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
	// Recognised file alongside an unrecognised one.
	writeConfigFile(t, root, "application.properties", "[hedera]")
	writeConfigFile(t, root, "unknown-file.txt", "ignore me")

	got, err := scanConfigFiles(root)
	if err != nil {
		t.Fatalf("scanConfigFiles returned error for unrecognised file: %v", err)
	}
	// Only the recognised file is returned; the unrecognised one is warned + skipped.
	if len(got) != 1 {
		t.Fatalf("want 1 file, got %d: %v", len(got), got)
	}
	if got[0].entry.kind != "ApplicationProperties" {
		t.Errorf("want ApplicationProperties, got %s", got[0].entry.kind)
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

	wantName := testOperationID + "-log4j2config"
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
		Namespace:   "solo",
		UpgradePath: root,
		Orbit:       "test-orbit",
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["log4j2.xml"])

	// Deploy in a goroutine so we can set conditions concurrently.
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.deployConfigCRs(context.Background(), testOperationID)
	}()

	// Wait briefly for the CR to be created, then stamp Valid=True.
	time.Sleep(20 * time.Millisecond)
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
		Namespace:   "solo",
		UpgradePath: root,
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["settings.txt"])

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.deployConfigCRs(context.Background(), testOperationID)
	}()

	time.Sleep(20 * time.Millisecond)
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
	executeCR := buildExecuteCR(testOperationID, time.Now())
	_ = unstructured.SetNestedField(executeCR.Object, string(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon),
		"status", "phase")
	executeCR.SetName("test-execute")
	executeCR.SetNamespace("solo")

	fakeClient := fake.NewSimpleDynamicClient(scheme, executeCR)
	p := NewProvisionerProxyWithClient(ProvisionerProxyConfig{
		Namespace:   "solo",
		UpgradePath: root,
		Orbit:       "test-orbit",
	}, fakeClient)

	crName := configCRName(testOperationID, configFileKinds["log4j2.xml"])

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.handleExecute(context.Background(), executeCR)
	}()

	// Stamp Valid=True so the wait completes.
	time.Sleep(20 * time.Millisecond)
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
