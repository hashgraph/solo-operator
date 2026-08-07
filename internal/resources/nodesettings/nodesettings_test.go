// SPDX-License-Identifier: Apache-2.0

package nodesettings

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := operatorv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

const validSettings = `# Hedera platform settings
checkSignedStateFromDisk,              1
state.recoveryFile,                    data/saved/RecoveryFile.dat`

func TestCommonLabels(t *testing.T) {
	obj := &operatorv1alpha1.NodeSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "test-settings", Namespace: "default"},
	}
	labels := CommonLabels(obj)

	if labels[common.LabelManagedBy] != common.ManagedByValue {
		t.Errorf("managed-by: got %q, want %q", labels[common.LabelManagedBy], common.ManagedByValue)
	}
	if labels[common.LabelPartOf] != PartOfNodeSettings {
		t.Errorf("part-of: got %q, want %q", labels[common.LabelPartOf], PartOfNodeSettings)
	}
	if labels["app"] != "test-settings" {
		t.Errorf("app: got %q, want %q", labels["app"], "test-settings")
	}
}

func TestConfigMapName(t *testing.T) {
	obj := &operatorv1alpha1.NodeSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
	}
	if got := ConfigMapName(obj); got != "node0" {
		t.Errorf("ConfigMapName: got %q, want %q", got, "node0")
	}
}

func TestConfigMapNameForRef(t *testing.T) {
	if got := ConfigMapNameForRef("network"); got != "network" {
		t.Errorf("ConfigMapNameForRef: got %q, want %q", got, "network")
	}
}

func TestValidateContent_Valid(t *testing.T) {
	if err := ValidateContent(validSettings); err != nil {
		t.Errorf("expected no error for valid settings, got: %v", err)
	}
}

func TestValidateContent_ValidCommentOnly(t *testing.T) {
	content := "# just a comment\n\n# another comment"
	if err := ValidateContent(content); err != nil {
		t.Errorf("expected no error for comment-only content, got: %v", err)
	}
}

func TestValidateContent_MissingComma(t *testing.T) {
	if err := ValidateContent("checkSignedStateFromDisk 1"); err == nil {
		t.Error("expected error for line without comma, got nil")
	}
}

func TestValidateContent_EmptyKey(t *testing.T) {
	if err := ValidateContent(",value"); err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

func TestValidateContent_EmptyContent(t *testing.T) {
	if err := ValidateContent(""); err == nil {
		t.Error("expected error for empty content, got nil")
	}
}

func TestValidateContent_WhitespaceOnly(t *testing.T) {
	if err := ValidateContent("   \n  \n"); err == nil {
		t.Error("expected error for whitespace-only content, got nil")
	}
}

func TestConfigMapSubReconciler_CreatesConfigMap(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.NodeSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.NodeSettingsSpec{Content: validSettings},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if cm.Data[ConfigFileName] != validSettings {
		t.Errorf("ConfigMap data mismatch: got %q, want %q", cm.Data[ConfigFileName], validSettings)
	}
}

func TestConfigMapSubReconciler_UpdatesConfigMap(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.NodeSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.NodeSettingsSpec{Content: validSettings},
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Data:       map[string]string{ConfigFileName: "checkSignedStateFromDisk, 0"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj, existingCM).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if cm.Data[ConfigFileName] != validSettings {
		t.Errorf("ConfigMap not updated: got %q, want %q", cm.Data[ConfigFileName], validSettings)
	}
}

func TestConfigMapSubReconciler_InvalidContent(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.NodeSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.NodeSettingsSpec{Content: "badline without comma"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Errorf("expected nil (stop retrying) for invalid content, got: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if getErr := c.Get(context.Background(), types.NamespacedName{Name: "node0", Namespace: "default"}, cm); getErr == nil {
		t.Error("ConfigMap should not have been created for invalid content")
	}
}
