// SPDX-License-Identifier: Apache-2.0

package applicationproperties

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

const validProperties = `# Solo application properties
hedera.config.version=0
netty.mode=TEST
staking.periodMins=1`

func TestCommonLabels(t *testing.T) {
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "test-appprops", Namespace: "default"},
	}
	labels := CommonLabels(obj)

	if labels[common.LabelManagedBy] != common.ManagedByValue {
		t.Errorf("managed-by: got %q, want %q", labels[common.LabelManagedBy], common.ManagedByValue)
	}
	if labels[common.LabelPartOf] != PartOfApplicationProperties {
		t.Errorf("part-of: got %q, want %q", labels[common.LabelPartOf], PartOfApplicationProperties)
	}
	if labels["app"] != "test-appprops" {
		t.Errorf("app: got %q, want %q", labels["app"], "test-appprops")
	}
}

func TestConfigMapName(t *testing.T) {
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
		Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Scope: "node0"},
	}
	if got := ConfigMapName(obj); got != "node0-application-properties" {
		t.Errorf("ConfigMapName: got %q, want %q", got, "node0-application-properties")
	}
}

func TestValidateContent_Valid(t *testing.T) {
	if err := ValidateContent(validProperties); err != nil {
		t.Errorf("expected no error for valid properties, got: %v", err)
	}
}

func TestValidateContent_ValidCommentOnly(t *testing.T) {
	content := "# just a comment\n\n# another comment"
	if err := ValidateContent(content); err != nil {
		t.Errorf("expected no error for comment-only content, got: %v", err)
	}
}

func TestValidateContent_MissingEquals(t *testing.T) {
	if err := ValidateContent("hedera.config.version 0"); err == nil {
		t.Error("expected error for line without equals, got nil")
	}
}

func TestValidateContent_EmptyKey(t *testing.T) {
	if err := ValidateContent("=value"); err == nil {
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
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Scope: "node0", Content: validProperties},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0-application-properties", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if cm.Data[ConfigFileName] != validProperties {
		t.Errorf("ConfigMap data mismatch: got %q, want %q", cm.Data[ConfigFileName], validProperties)
	}
}

func TestConfigMapSubReconciler_UpdatesConfigMap(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Scope: "node0", Content: validProperties},
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: "default"},
		Data:       map[string]string{ConfigFileName: "hedera.config.version=1"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj, existingCM).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0-application-properties", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if cm.Data[ConfigFileName] != validProperties {
		t.Errorf("ConfigMap not updated: got %q, want %q", cm.Data[ConfigFileName], validProperties)
	}
}

// orbitWithLedgerID builds a cluster-scoped Orbit CR with the given raw ledger id.
func orbitWithLedgerID(name, ledgerID string) *operatorv1alpha1.Orbit {
	o := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: name}}
	o.Spec.Network = &operatorv1alpha1.OrbitNetwork{LedgerId: ledgerID}
	return o
}

func validCondition(obj *operatorv1alpha1.ApplicationProperties) *metav1.Condition {
	return apimeta.FindStatusCondition(obj.Status.Conditions, operatorv1alpha1.ApplicationPropertiesConditionTypeValid)
}

// TestConfigMapSubReconciler_LedgerIDEquivalent verifies the validator accepts a
// content ledger.id that matches the Orbit's ledgerId across hex-representation
// differences: content "0x1" vs Orbit "0x01" name the same ledger, so it must
// validate and create the ConfigMap rather than reject on a raw string mismatch.
func TestConfigMapSubReconciler_LedgerIDEquivalent(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec: operatorv1alpha1.ApplicationPropertiesSpec{
			Scope:   "node0",
			Orbit:   "solo-orbit",
			Content: "hedera.config.version=0\nledger.id=0x1",
		},
	}
	orbit := orbitWithLedgerID("solo-orbit", "0x01")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj, orbit).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	if _, err := r.Reconcile(context.Background(), obj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cond := validCondition(obj); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected Valid=True for 0x1 vs 0x01, got %+v", cond)
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0-application-properties", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap should have been created: %v", err)
	}
}

// TestConfigMapSubReconciler_LedgerIDMismatch verifies a genuinely different
// ledger id (0x02 vs Orbit 0x01) is rejected: Valid=False and no ConfigMap.
func TestConfigMapSubReconciler_LedgerIDMismatch(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec: operatorv1alpha1.ApplicationPropertiesSpec{
			Scope:   "node0",
			Orbit:   "solo-orbit",
			Content: "hedera.config.version=0\nledger.id=0x02",
		},
	}
	orbit := orbitWithLedgerID("solo-orbit", "0x01")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj, orbit).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	if _, err := r.Reconcile(context.Background(), obj); err != nil {
		t.Fatalf("expected nil (stop retrying) on mismatch, got: %v", err)
	}
	cond := validCondition(obj)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "LedgerIDMismatch" {
		t.Fatalf("expected Valid=False/LedgerIDMismatch for 0x02 vs 298, got %+v", cond)
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "node0-application-properties", Namespace: "default"}, cm); err == nil {
		t.Error("ConfigMap should not have been created on ledger mismatch")
	}
}

func TestConfigMapSubReconciler_InvalidContent(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.ApplicationProperties{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Scope: "node0", Content: "badline without equals"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Errorf("expected nil (stop retrying) for invalid content, got: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if getErr := c.Get(context.Background(), types.NamespacedName{Name: "node0-application-properties", Namespace: "default"}, cm); getErr == nil {
		t.Error("ConfigMap should not have been created for invalid content")
	}
}
