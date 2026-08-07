// SPDX-License-Identifier: Apache-2.0

package log4j2config

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

const validXML = `<?xml version="1.0" encoding="UTF-8"?>
<Configuration status="WARN" monitorInterval="600">
  <Appenders>
    <Console name="Console" target="SYSTEM_OUT">
      <PatternLayout pattern="%d{yyyy-MM-dd HH:mm:ss.SSS} %-5p %-4L %c{1} - %m{nolookups}%n"/>
    </Console>
  </Appenders>
  <Loggers>
    <Root level="WARN">
      <AppenderRef ref="Console"/>
    </Root>
  </Loggers>
</Configuration>`

func TestCommonLabels(t *testing.T) {
	obj := &operatorv1alpha1.Log4j2Config{
		ObjectMeta: metav1.ObjectMeta{Name: "test-log4j2", Namespace: "default"},
	}
	labels := CommonLabels(obj)

	if labels[common.LabelManagedBy] != common.ManagedByValue {
		t.Errorf("managed-by: got %q, want %q", labels[common.LabelManagedBy], common.ManagedByValue)
	}
	if labels[common.LabelPartOf] != PartOfLog4j2Config {
		t.Errorf("part-of: got %q, want %q", labels[common.LabelPartOf], PartOfLog4j2Config)
	}
	if labels["app"] != "test-log4j2" {
		t.Errorf("app: got %q, want %q", labels["app"], "test-log4j2")
	}
}

func TestConfigMapName(t *testing.T) {
	obj := &operatorv1alpha1.Log4j2Config{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
	}
	if got := ConfigMapName(obj); got != "node0" {
		t.Errorf("ConfigMapName: got %q, want %q", got, "node0")
	}
}

func TestValidateContent_ValidNoNamespace(t *testing.T) {
	if err := ValidateContent(validXML); err != nil {
		t.Errorf("expected no error for valid no-namespace XML, got: %v", err)
	}
}

func TestValidateContent_ValidWithLog4jNamespace(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Configuration xmlns="https://logging.apache.org/xml/ns" status="WARN">
  <Appenders><Console name="Console" target="SYSTEM_OUT"/></Appenders>
  <Loggers><Root level="WARN"><AppenderRef ref="Console"/></Root></Loggers>
</Configuration>`
	if err := ValidateContent(xml); err != nil {
		t.Errorf("expected no error for valid namespaced XML, got: %v", err)
	}
}

func TestValidateContent_MalformedXML(t *testing.T) {
	if err := ValidateContent(`<Configuration status="WARN"><Unclosed`); err == nil {
		t.Error("expected error for malformed XML, got nil")
	}
}

func TestValidateContent_WrongRootElement(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><NotLog4j/>`
	if err := ValidateContent(xml); err == nil {
		t.Error("expected error for wrong root element, got nil")
	}
}

func TestValidateContent_WrongNamespace(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Configuration xmlns="http://some-other-namespace.com" status="WARN"/>`
	if err := ValidateContent(xml); err == nil {
		t.Error("expected error for wrong namespace, got nil")
	}
}

func TestValidateContent_EmptyContent(t *testing.T) {
	if err := ValidateContent(""); err == nil {
		t.Error("expected error for empty content, got nil")
	}
}

func TestConfigMapSubReconciler_CreatesConfigMap(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.Log4j2Config{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: validXML},
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
	if cm.Data[ConfigFileName] != validXML {
		t.Errorf("ConfigMap data mismatch: got %q, want %q", cm.Data[ConfigFileName], validXML)
	}
}

func TestConfigMapSubReconciler_UpdatesConfigMap(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.Log4j2Config{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: validXML},
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Data:       map[string]string{ConfigFileName: "<Configuration status=\"INFO\"/>"},
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
	if cm.Data[ConfigFileName] != validXML {
		t.Errorf("ConfigMap not updated: got %q, want %q", cm.Data[ConfigFileName], validXML)
	}
}

func TestConfigMapSubReconciler_InvalidXML(t *testing.T) {
	scheme := newScheme(t)
	obj := &operatorv1alpha1.Log4j2Config{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "default"},
		Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: "<NotLog4j/>"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()
	r := NewConfigMapSubReconciler(c, scheme, nil)

	_, err := r.Reconcile(context.Background(), obj)
	if err != nil {
		t.Errorf("expected nil (stop retrying) for invalid XML, got: %v", err)
	}

	cm := &corev1.ConfigMap{}
	if getErr := c.Get(context.Background(), types.NamespacedName{Name: "node0", Namespace: "default"}, cm); getErr == nil {
		t.Error("ConfigMap should not have been created for invalid XML")
	}
}
