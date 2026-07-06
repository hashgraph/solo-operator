// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// sampleCR builds a NetworkUpgradePrepare-shaped unstructured CR with the
// operation-id/step labels and a nested spec, exercising the round-trip of the
// kinds the handlers actually persist.
func sampleCR(operationID, step string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": operatorv1alpha1.GroupVersion.String(),
		"kind":       "NetworkUpgradePrepare",
		"metadata": map[string]interface{}{
			"name":      operationID + "-" + step,
			"namespace": "solo",
			"labels": map[string]interface{}{
				operatorv1alpha1.LabelOperationID: operationID,
				operatorv1alpha1.LabelStep:        step,
			},
			"annotations": map[string]interface{}{
				"operator.solo.hedera.com/created-by": "UC",
			},
		},
		"spec": map[string]interface{}{
			"operationId": operationID,
			"orbit":       "my-orbit",
			"images": map[string]interface{}{
				"consensusNode": "ghcr.io/hashgraph/consensus:v0.76.0",
			},
		},
	}}
}

func TestCRStorePersistThenDelete(t *testing.T) {
	dir := t.TempDir()
	s := NewCRStore(dir)
	const opID, step = "upgrade-v0.76.0-20260415T143000Z", operatorv1alpha1.StepPrepare

	path, err := s.Persist(opID, step, sampleCR(opID, step))
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// The file must appear before the (would-be) Create, named deterministically.
	if got := filepath.Base(path); got != opID+".prepare.yaml" {
		t.Fatalf("filename = %q, want %q", got, opID+".prepare.yaml")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persisted file not present: %v", err)
	}

	// Delete (the success path) removes it.
	if err := s.Delete(opID, step); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still present after Delete: err=%v", err)
	}
}

func TestCRStoreDeleteMissingIsNotError(t *testing.T) {
	s := NewCRStore(t.TempDir())
	if err := s.Delete("upgrade-20260415T143000Z", operatorv1alpha1.StepFreeze); err != nil {
		t.Fatalf("Delete of missing file should be nil, got %v", err)
	}
}

func TestCRStoreRoundTripsLosslessly(t *testing.T) {
	dir := t.TempDir()
	s := NewCRStore(dir)
	const opID, step = "upgrade-v0.76.0-20260415T143000Z", operatorv1alpha1.StepExecute
	original := sampleCR(opID, step)

	if _, err := s.Persist(opID, step, original); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(got))
	}
	pc := got[0]
	if pc.OperationID != opID || pc.Step != step {
		t.Fatalf("recovered (operationId=%q step=%q), want (%q,%q)", pc.OperationID, pc.Step, opID, step)
	}
	// marshal -> write -> read -> unmarshal must reproduce the object exactly.
	if !reflect.DeepEqual(pc.Object.Object, original.Object) {
		t.Fatalf("round-trip mismatch:\n got: %#v\nwant: %#v", pc.Object.Object, original.Object)
	}
}

// TestCRStoreCrashLeftoverIsRecoverable simulates a crash in the window between
// persist and a successful Create: the file is left on disk and must be detected
// and decoded on the next startup scan (the re-send itself is #998).
func TestCRStoreCrashLeftoverIsRecoverable(t *testing.T) {
	dir := t.TempDir()
	s := NewCRStore(dir)

	// Two distinct operations persisted, neither deleted (crash before Create ack).
	want := map[string]string{
		"upgrade-v0.76.0-20260415T143000Z": operatorv1alpha1.StepPrepare,
		"upgrade-v0.77.0-20260501T090000Z": operatorv1alpha1.StepFreeze,
	}
	for opID, step := range want {
		if _, err := s.Persist(opID, step, sampleCR(opID, step)); err != nil {
			t.Fatalf("Persist(%s): %v", opID, err)
		}
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("recovered %d CRs, want 2", len(got))
	}
	for _, pc := range got {
		if want[pc.OperationID] != pc.Step {
			t.Fatalf("recovered step %q for %q, want %q", pc.Step, pc.OperationID, want[pc.OperationID])
		}
		if pc.Object == nil || pc.Object.Object == nil {
			t.Fatalf("recovered CR %q has no decoded object", pc.OperationID)
		}
	}
}

func TestCRStorePersistIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewCRStore(dir)
	const opID, step = "upgrade-20260415T143000Z", operatorv1alpha1.StepPrepare

	if _, err := s.Persist(opID, step, sampleCR(opID, step)); err != nil {
		t.Fatalf("first Persist: %v", err)
	}
	// Re-detection of the same marker must overwrite, not error or duplicate.
	if _, err := s.Persist(opID, step, sampleCR(opID, step)); err != nil {
		t.Fatalf("second Persist: %v", err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d entries, want 1 (idempotent overwrite)", len(got))
	}
}

func TestCRStoreListIgnoresForeignAndPartialFiles(t *testing.T) {
	dir := t.TempDir()
	s := NewCRStore(dir)
	const opID, step = "upgrade-20260415T143000Z", operatorv1alpha1.StepPrepare
	if _, err := s.Persist(opID, step, sampleCR(opID, step)); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Drop files List must skip: a leftover temp file, a foreign file, and a
	// name with an unknown step.
	for _, name := range []string{
		opID + ".prepare.yaml.tmp",
		"README.md",
		opID + ".bogusstep.yaml",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("noise"), 0o640); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d entries, want only the valid CR", len(got))
	}
}

func TestCRStoreListMissingDirIsEmpty(t *testing.T) {
	s := NewCRStore(filepath.Join(t.TempDir(), "does-not-exist"))
	got, err := s.List()
	if err != nil {
		t.Fatalf("List of missing dir should be nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List of missing dir returned %d entries, want 0", len(got))
	}
}

func TestCRStoreRejectsBadInput(t *testing.T) {
	s := NewCRStore(t.TempDir())

	if _, err := s.Persist("../escape", operatorv1alpha1.StepPrepare, sampleCR("x", "prepare")); err == nil {
		t.Fatal("Persist with path-traversal operationId should fail")
	}
	if _, err := s.Persist("upgrade-20260415T143000Z", "not-a-step", sampleCR("x", "y")); err == nil {
		t.Fatal("Persist with unknown step should fail")
	}
	if _, err := s.Persist("upgrade-20260415T143000Z", operatorv1alpha1.StepPrepare, nil); err == nil {
		t.Fatal("Persist of nil CR should fail")
	}
}
