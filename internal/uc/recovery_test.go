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
	exists, _, err := f.alreadyCreated(context.Background(), networkUpgradePrepareGVR, wantOperationID, operatorv1alpha1.StepPrepare)
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
// RecoverOnStartup (integration: seed + re-send)
// --------------------------------------------------------------------------

func TestRecoverOnStartup_FullFlow(t *testing.T) {
	markerRoot := t.TempDir()
	crDir := t.TempDir()
	f := newFakeFactory(t, testConfig(markerRoot, crDir))

	// A persisted CR from the crash window — not yet in the cluster.
	persistCROnDisk(t, f, wantOperationID, operatorv1alpha1.StepPrepare)

	if err := f.RecoverOnStartup(context.Background()); err != nil {
		t.Fatalf("RecoverOnStartup: %v", err)
	}

	// The crash-window Prepare CR must have been sent to the cluster.
	prepList, _ := f.client.Resource(networkUpgradePrepareGVR).Namespace(f.cfg.Namespace).
		List(context.Background(), metav1.ListOptions{})
	if len(prepList.Items) != 1 {
		t.Fatalf("expected 1 prepare CR after recovery; got %d", len(prepList.Items))
	}

	// Marker dir and its contents must be untouched — rotation is the
	// provisioner-proxy monitor's responsibility.
	if _, err := os.Stat(markerRoot); err != nil {
		t.Fatalf("marker dir should not be touched by RecoverOnStartup: %v", err)
	}
}
