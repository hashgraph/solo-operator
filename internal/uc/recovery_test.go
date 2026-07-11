// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// seedExecuteCR creates a NetworkUpgradeExecute CR in the fake cluster with
// the given operationId and status.phase, mirroring what would exist after a
// prior UC process ran through the execute phase.
func seedExecuteCR(t *testing.T, f *Factory, operationID string, phase operatorv1alpha1.ExecutePhase) {
	t.Helper()
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": operatorv1alpha1.GroupVersion.String(),
			"kind":       "NetworkUpgradeExecute",
			"metadata": map[string]interface{}{
				"name":      operationID + "-execute",
				"namespace": f.cfg.Namespace,
				"labels": map[string]interface{}{
					operatorv1alpha1.LabelOperationID: operationID,
					operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
				},
			},
			"spec": map[string]interface{}{
				"operationId": operationID,
			},
		},
	}
	if phase != "" {
		if err := unstructured.SetNestedField(cr.Object, string(phase), "status", "phase"); err != nil {
			t.Fatalf("set phase: %v", err)
		}
	}
	if _, err := f.client.Resource(networkUpgradeExecuteGVR).Namespace(f.cfg.Namespace).
		Create(context.Background(), cr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed execute CR: %v", err)
	}
}

// persistCROnDisk writes a fake persisted CR for operationId+step to the store
// and returns the expected disk path.
func persistCROnDisk(t *testing.T, f *Factory, operationID, step string) string {
	t.Helper()
	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": operatorv1alpha1.GroupVersion.String(),
			"kind":       "NetworkUpgradePrepare",
			"metadata": map[string]interface{}{
				"name":      operationID + "-" + step,
				"namespace": f.cfg.Namespace,
				"labels": map[string]interface{}{
					operatorv1alpha1.LabelOperationID: operationID,
					operatorv1alpha1.LabelStep:        step,
				},
			},
			"spec": map[string]interface{}{
				"operationId": operationID,
			},
		},
	}
	path, err := f.store.Persist(operationID, step, cr)
	if err != nil {
		t.Fatalf("persist CR: %v", err)
	}
	return path
}

// writeMarkers creates the given marker files in the marker path dir.
func writeMarkers(t *testing.T, dir string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		p := filepath.Join(dir, m)
		if err := os.WriteFile(p, []byte(m), 0o644); err != nil {
			t.Fatalf("write marker %s: %v", m, err)
		}
	}
}

// --------------------------------------------------------------------------
// recoverPersistedCRs
// --------------------------------------------------------------------------

func TestRecoverPersistedCRs_DiskOnlyResent(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)

	if err := f.recoverPersistedCRs(context.Background()); err != nil {
		t.Fatalf("recoverPersistedCRs: %v", err)
	}

	// CR must now exist in the cluster.
	list, err := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("expected 1 prepare CR in cluster; got %d, err=%v", len(list.Items), err)
	}

	// Disk copy must be gone.
	remaining, _ := f.store.List()
	if len(remaining) != 0 {
		t.Fatalf("expected 0 persisted CRs after recovery; got %d", len(remaining))
	}

	// Cache must reflect the created step.
	exists, err := f.alreadyCreated(context.Background(), networkUpgradePrepareGVR, wantOperationID, operatorv1alpha1.StepPrepare)
	if err != nil || !exists {
		t.Fatalf("expected CR to be in cache after re-send; exists=%v err=%v", exists, err)
	}
}

func TestRecoverPersistedCRs_AlreadyInCluster(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	// Persist to disk AND pre-create in cluster (crash after send, before delete).
	path := persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)
	seedPrepareCR(t, f, wantOperationID, "")

	if err := f.recoverPersistedCRs(context.Background()); err != nil {
		t.Fatalf("recoverPersistedCRs: %v", err)
	}

	// Disk copy must be deleted.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected persisted file to be deleted; stat err=%v", err)
	}

	// Only the original seeded CR should exist (no duplicate).
	list, _ := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if len(list.Items) != 1 {
		t.Fatalf("expected exactly 1 prepare CR; got %d", len(list.Items))
	}
}

func TestRecoverPersistedCRs_NoPersisted(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	// Should be a no-op with no error.
	if err := f.recoverPersistedCRs(context.Background()); err != nil {
		t.Fatalf("recoverPersistedCRs: %v", err)
	}
}

// --------------------------------------------------------------------------
// cleanupCompletedArtifacts
// --------------------------------------------------------------------------

func TestCleanupCompletedArtifacts_SucceededRemovesAll(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	seedExecuteCR(t, f, wantOperationID, operatorv1alpha1.ExecutePhaseSucceeded)
	persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)
	writeMarkers(t, markerRoot, MarkerExecuteImmediate, MarkerFreezeScheduled)

	if err := f.cleanupCompletedArtifacts(context.Background()); err != nil {
		t.Fatalf("cleanupCompletedArtifacts: %v", err)
	}

	// Marker dir should be gone (Succeeded → full cleanup).
	if _, err := os.Stat(markerRoot); !os.IsNotExist(err) {
		t.Fatalf("expected marker dir to be removed after Succeeded; stat err=%v", err)
	}

	// Persisted CRs should be gone.
	remaining, _ := f.store.List()
	if len(remaining) != 0 {
		t.Fatalf("expected 0 persisted CRs after cleanup; got %d", len(remaining))
	}
}

func TestCleanupCompletedArtifacts_FailedRemovesMarkersNotDir(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	seedExecuteCR(t, f, wantOperationID, operatorv1alpha1.ExecutePhaseFailed)
	persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)
	writeMarkers(t, markerRoot, MarkerExecuteImmediate)

	if err := f.cleanupCompletedArtifacts(context.Background()); err != nil {
		t.Fatalf("cleanupCompletedArtifacts: %v", err)
	}

	// Marker dir must be preserved for diagnostics after Failed.
	if _, err := os.Stat(markerRoot); err != nil {
		t.Fatalf("expected marker dir to be preserved after Failed; stat err=%v", err)
	}

	// Marker file should be gone.
	if _, err := os.Stat(filepath.Join(markerRoot, MarkerExecuteImmediate)); !os.IsNotExist(err) {
		t.Fatalf("expected marker file to be removed; stat err=%v", err)
	}

	// Persisted CRs should be gone.
	remaining, _ := f.store.List()
	if len(remaining) != 0 {
		t.Fatalf("expected 0 persisted CRs after cleanup; got %d", len(remaining))
	}
}

func TestCleanupCompletedArtifacts_InFlightNotCleaned(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	// Execute CR is in a non-terminal phase.
	seedExecuteCR(t, f, wantOperationID, operatorv1alpha1.ExecutePhasePendingNodeUpgrade)
	persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)
	writeMarkers(t, markerRoot, MarkerExecuteImmediate)

	if err := f.cleanupCompletedArtifacts(context.Background()); err != nil {
		t.Fatalf("cleanupCompletedArtifacts: %v", err)
	}

	// Marker dir and files must be preserved.
	if _, err := os.Stat(filepath.Join(markerRoot, MarkerExecuteImmediate)); err != nil {
		t.Fatalf("expected marker file to be preserved for in-flight CR; stat err=%v", err)
	}

	// Persisted CRs must be preserved.
	remaining, _ := f.store.List()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 persisted CR to be preserved; got %d", len(remaining))
	}
}

func TestCleanupCompletedArtifacts_NoExecuteCRs(t *testing.T) {
	f := newFakeFactory(t, testConfig(t.TempDir(), t.TempDir()))
	if err := f.cleanupCompletedArtifacts(context.Background()); err != nil {
		t.Fatalf("cleanupCompletedArtifacts: %v", err)
	}
}

// --------------------------------------------------------------------------
// RecoverOnStartup (integration: seed + re-send + cleanup)
// --------------------------------------------------------------------------

func TestRecoverOnStartup_FullFlow(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	// A disk-only CR (crash window) for a different operationId.
	crashOpID := "upgrade-0.74.0-20260101T000000Z"
	persistCROnDisk(t, f, crashOpID, operatorv1alpha1.StepPrepare)

	// A completed Execute CR that left marker files behind.
	seedExecuteCR(t, f, wantOperationID, operatorv1alpha1.ExecutePhaseSucceeded)
	writeMarkers(t, markerRoot, MarkerExecuteImmediate)

	if err := f.RecoverOnStartup(context.Background()); err != nil {
		t.Fatalf("RecoverOnStartup: %v", err)
	}

	// Crash-window CR must have been sent to the cluster.
	prepList, _ := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if len(prepList.Items) != 1 {
		t.Fatalf("expected 1 prepare CR after recovery; got %d", len(prepList.Items))
	}

	// Marker dir removed (Succeeded Execute CR).
	if _, err := os.Stat(markerRoot); !os.IsNotExist(err) {
		t.Fatalf("expected marker dir removed; stat err=%v", err)
	}
}
