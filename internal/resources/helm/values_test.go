// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

// ─── mergeMaps ────────────────────────────────────────────────────────────────

func TestMergeMaps_srcOverridesDst(t *testing.T) {
	dst := map[string]interface{}{"a": "old", "b": "keep"}
	src := map[string]interface{}{"a": "new", "c": "added"}
	result := mergeMaps(dst, src)

	if result["a"] != "new" {
		t.Errorf("expected a=new, got %v", result["a"])
	}
	if result["b"] != "keep" {
		t.Errorf("expected b=keep, got %v", result["b"])
	}
	if result["c"] != "added" {
		t.Errorf("expected c=added, got %v", result["c"])
	}
}

func TestMergeMaps_deepMerge(t *testing.T) {
	dst := map[string]interface{}{
		"parent": map[string]interface{}{"a": "old", "b": "keep"},
	}
	src := map[string]interface{}{
		"parent": map[string]interface{}{"a": "new", "c": "added"},
	}
	result := mergeMaps(dst, src)

	parent, ok := result["parent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected parent to be a map")
	}
	if parent["a"] != "new" {
		t.Errorf("expected parent.a=new, got %v", parent["a"])
	}
	if parent["b"] != "keep" {
		t.Errorf("expected parent.b=keep (not overwritten), got %v", parent["b"])
	}
	if parent["c"] != "added" {
		t.Errorf("expected parent.c=added, got %v", parent["c"])
	}
}

func TestMergeMaps_emptyLayers(t *testing.T) {
	dst := map[string]interface{}{"a": "keep"}
	result := mergeMaps(dst, map[string]interface{}{})
	if result["a"] != "keep" {
		t.Errorf("expected a=keep, got %v", result["a"])
	}
}

// ─── MergeValues – inline Values ─────────────────────────────────────────────

func TestMergeValues_inlineOnly(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{"key": "value"})
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:  operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		Values: &apiextv1.JSON{Raw: raw},
	}

	c := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestMergeValues_inlineOverridesValuesFrom(t *testing.T) {
	scheme := newTestScheme()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "base-values", Namespace: "test-ns"},
		Data:       map[string]string{"values.yaml": `{"key": "from-configmap", "other": "keep"}`},
	}

	inlineRaw, _ := json.Marshal(map[string]interface{}{"key": "from-inline"})
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		ValuesFrom: []operatorv1alpha1.HelmValuesReference{
			{Kind: "ConfigMap", Name: "base-values"},
		},
		Values: &apiextv1.JSON{Raw: inlineRaw},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Inline wins over ConfigMap.
	if result["key"] != "from-inline" {
		t.Errorf("expected key=from-inline (inline wins), got %v", result["key"])
	}
	// ConfigMap key not overridden by inline should be preserved.
	if result["other"] != "keep" {
		t.Errorf("expected other=keep, got %v", result["other"])
	}
}

// ─── ValuesFrom ───────────────────────────────────────────────────────────────

func TestMergeValues_fromConfigMap(t *testing.T) {
	scheme := newTestScheme()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-values", Namespace: "test-ns"},
		Data:       map[string]string{"values.yaml": `{"replicaCount": 3}`},
	}

	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		ValuesFrom: []operatorv1alpha1.HelmValuesReference{
			{Kind: "ConfigMap", Name: "my-values"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["replicaCount"] != float64(3) {
		t.Errorf("expected replicaCount=3, got %v", result["replicaCount"])
	}
}

func TestMergeValues_fromSecret(t *testing.T) {
	scheme := newTestScheme()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-values", Namespace: "test-ns"},
		Data:       map[string][]byte{"values.yaml": []byte(`{"password": "s3cr3t"}`)},
	}

	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		ValuesFrom: []operatorv1alpha1.HelmValuesReference{
			{Kind: "Secret", Name: "secret-values"},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["password"] != "s3cr3t" {
		t.Errorf("expected password=s3cr3t, got %v", result["password"])
	}
}

func TestMergeValues_secretParseErrorRedactsContent(t *testing.T) {
	const marker = "hunter2SECRET"
	for name, body := range map[string]string{
		"unknown anchor":   "password: *" + marker + "\n",
		"invalid map key":  "? [" + marker + ", b]\n: x\n",
		"non-mapping root": `"` + marker + `"`,
	} {
		t.Run(name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "secret-values", Namespace: "test-ns"},
				Data:       map[string][]byte{"values.yaml": []byte(body)},
			}
			spec := &operatorv1alpha1.HelmCapsuleSpec{
				Chart:      operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
				ValuesFrom: []operatorv1alpha1.HelmValuesReference{{Kind: "Secret", Name: "secret-values"}},
			}
			c := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(secret).Build()

			_, err := MergeValues(context.Background(), c, "test-ns", spec)
			if err == nil {
				t.Fatal("expected parse error, got nil")
			}
			if strings.Contains(errx.Format(err), marker) {
				t.Fatalf("error leaks Secret content: %s", errx.Format(err))
			}
			if reason, _ := errx.ReasonOf(err); reason != ReasonValuesBuild {
				t.Errorf("reason = %q, want %q", reason, ReasonValuesBuild)
			}
		})
	}
}

func TestMergeValues_optionalMissingConfigMap(t *testing.T) {
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		ValuesFrom: []operatorv1alpha1.HelmValuesReference{
			{Kind: "ConfigMap", Name: "missing", Optional: true},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("optional missing ConfigMap should not error, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestMergeValues_requiredMissingConfigMapErrors(t *testing.T) {
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart: operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "test", Version: "1.0.0"},
		ValuesFrom: []operatorv1alpha1.HelmValuesReference{
			{Kind: "ConfigMap", Name: "missing", Optional: false},
		},
	}

	c := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
	_, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err == nil {
		t.Fatal("expected error for missing required ConfigMap")
	}
}

// ─── Per-component injection ──────────────────────────────────────────────────

// makeOrbit sets both LedgerId and ChainId to id for test convenience. The relay
// test reads ChainId (CHAIN_ID) and the explorer test reads LedgerId (ledgerID);
// setting both keeps existing single-arg callers unchanged.
func makeOrbit(name, id string) *operatorv1alpha1.Orbit {
	return &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{
					AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: id, ChainId: id},
				},
			},
		},
	}
}

func makeCapsule(name, namespace, orbit, hostname string, port int32) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:     orbit,
			NodeId:    0,
			AccountId: "0.0.3",
			Weight:    1,
		},
		Status: operatorv1alpha1.ConsensusCapsuleStatus{
			AddressBookEntry: &operatorv1alpha1.AddressBookEntry{
				InternalHostname: hostname,
				InternalPort:     port,
			},
		},
	}
}

func TestMirrorValues_endpointsInjected(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("test-orbit", "03")
	cap1 := makeCapsule("node1", "test-ns", "test-orbit", "node1.test-ns.svc", 50211)
	cap2 := makeCapsule("node2", "test-ns", "test-orbit", "node2.test-ns.svc", 50211)
	addressbookSecret := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "addressbook-bin", Namespace: "test-ns"},
		BinaryData: map[string][]byte{"addressbook.bin": []byte("fake-addressbook")},
	}

	comp := operatorv1alpha1.HelmComponentMirror
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "hedera-mirror", Version: "0.129.1"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(orbit, cap1, cap2, addressbookSecret).
		WithStatusSubresource(cap1, cap2).
		Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hedera, _ := result["hedera"].(map[string]interface{})
	mirror, _ := hedera["mirror"].(map[string]interface{})
	importer, _ := mirror["importer"].(map[string]interface{})

	endpoints, _ := importer["consensusNodeEndpoints"].([]string)
	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d: %v", len(endpoints), endpoints)
	}
	if importer["network"] != "test-orbit" {
		t.Errorf("expected network=test-orbit, got %v", importer["network"])
	}

	// addressBook is injected as a top-level importer sub-chart value.
	importerSubchart, _ := result["importer"].(map[string]interface{})
	if importerSubchart["addressBook"] == nil {
		t.Error("expected addressBook to be set at importer.addressBook")
	}
}

func TestMirrorValues_noAddressBook(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("test-orbit", "")
	cap1 := makeCapsule("node1", "test-ns", "test-orbit", "node1.svc", 50211)

	comp := operatorv1alpha1.HelmComponentMirror
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "hedera-mirror", Version: "0.129.1"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(orbit, cap1).
		WithStatusSubresource(cap1).
		Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("expected no error when addressbook-bin absent, got: %v", err)
	}

	if result["importer"] != nil {
		importerSubchart, _ := result["importer"].(map[string]interface{})
		if importerSubchart["addressBook"] != nil {
			t.Error("expected addressBook to be absent when ConfigMap missing")
		}
	}
}

func TestRelayValues_mirrorURLDerived(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("test-orbit", "295")
	cap1 := makeCapsule("node1", "test-ns", "test-orbit", "node1.svc", 50211)

	comp := operatorv1alpha1.HelmComponentRelay
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "hedera-json-rpc-relay", Version: "0.69.2"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "test-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(orbit, cap1).
		WithStatusSubresource(cap1).
		Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config, _ := result["config"].(map[string]interface{})
	if config["MIRROR_NODE_URL"] != "http://mirror.test-ns.svc.cluster.local" {
		t.Errorf("unexpected MIRROR_NODE_URL: %v", config["MIRROR_NODE_URL"])
	}
	if config["CHAIN_ID"] != "295" {
		t.Errorf("expected CHAIN_ID=295, got %v", config["CHAIN_ID"])
	}
}

func TestExplorerValues_configJSON(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("my-orbit", "03")

	comp := operatorv1alpha1.HelmComponentExplorer
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "hiero-explorer-chart", Version: "24.15.0"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "my-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit).Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configStr, ok := result["config"].(string)
	if !ok {
		t.Fatalf("expected config to be a string, got %T", result["config"])
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &entries); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 config entry, got %d", len(entries))
	}
	if entries[0]["name"] != "my-orbit" {
		t.Errorf("expected name=my-orbit, got %v", entries[0]["name"])
	}
	if entries[0]["ledgerID"] != "03" {
		t.Errorf("expected ledgerID=03, got %v", entries[0]["ledgerID"])
	}
}

func TestBlockNodeValues_noInjection(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("test-orbit", "")
	cap1 := makeCapsule("node1", "test-ns", "test-orbit", "node1.svc", 50211)

	comp := operatorv1alpha1.HelmComponentBlockNode
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "block-node-server", Version: "0.41.0"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "test-orbit"},
		Values:    &apiextv1.JSON{Raw: []byte(`{"plugins":{"names":""}}`)},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(orbit, cap1).
		WithStatusSubresource(cap1).
		Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected only the inline values, got %v", result)
	}
	if _, ok := result["blockNode"]; ok {
		t.Errorf("expected no blockNode injection, got %v", result["blockNode"])
	}
}

func TestMergeValues_componentValuesOverriddenByInline(t *testing.T) {
	scheme := newTestScheme()
	orbit := makeOrbit("test-orbit", "03")
	cap1 := makeCapsule("node1", "test-ns", "test-orbit", "node1.svc", 50211)
	addressbookSecret := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "addressbook-bin", Namespace: "test-ns"},
		BinaryData: map[string][]byte{"addressbook.bin": []byte("fake-addressbook")},
	}

	// Inline Values override the auto-injected network value.
	inlineRaw, _ := json.Marshal(map[string]interface{}{
		"hedera": map[string]interface{}{
			"mirror": map[string]interface{}{
				"importer": map[string]interface{}{
					"network": "MAINNET",
				},
			},
		},
	})

	comp := operatorv1alpha1.HelmComponentMirror
	spec := &operatorv1alpha1.HelmCapsuleSpec{
		Chart:     operatorv1alpha1.HelmChartSpec{Repo: "oci://example.com", Chart: "hedera-mirror", Version: "0.129.1"},
		Component: &comp,
		OrbitRef:  &corev1.LocalObjectReference{Name: "test-orbit"},
		Values:    &apiextv1.JSON{Raw: inlineRaw},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(orbit, cap1, addressbookSecret).
		WithStatusSubresource(cap1).
		Build()

	result, err := MergeValues(context.Background(), c, "test-ns", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hedera, _ := result["hedera"].(map[string]interface{})
	mirror, _ := hedera["mirror"].(map[string]interface{})
	importer, _ := mirror["importer"].(map[string]interface{})

	// Inline override wins.
	if importer["network"] != "MAINNET" {
		t.Errorf("expected network=MAINNET (inline override), got %v", importer["network"])
	}
	// Auto-injected endpoints still present (deep merge, not replacement).
	if importer["consensusNodeEndpoints"] == nil {
		t.Error("expected consensusNodeEndpoints to survive deep merge")
	}
}
