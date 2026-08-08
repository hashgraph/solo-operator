// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"testing"
	"time"

	"github.com/automa-saga/daemonkit"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/client-go/dynamic/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func newFakeClientWithSSARReactor(allowed bool) *fake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			obj := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
			result := obj.DeepCopy()
			_ = unstructured.SetNestedField(result.Object, allowed, "status", "allowed")
			return true, result, nil
		})
	return client
}

func TestRBACProbe_AllAllowed(t *testing.T) {
	client := newFakeClientWithSSARReactor(true)
	probe := &RBACProbe{
		Client:    client,
		Namespace: "solo",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := probe.Probe(ctx); err != nil {
		t.Fatalf("expected probe to pass, got: %v", err)
	}
}

func TestRBACProbe_SomeDenied(t *testing.T) {
	client := newFakeClientWithSSARReactor(false)
	probe := &RBACProbe{
		Client:       client,
		Namespace:    "solo",
		PollInterval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := probe.Probe(ctx)
	if err == nil {
		t.Fatal("expected probe to fail when permissions are denied")
	}
}

func TestRBACProbe_DeniedResources(t *testing.T) {
	client := newFakeClientWithSSARReactor(false)
	probe := &RBACProbe{
		Client:    client,
		Namespace: "solo",
	}

	denied := probe.DeniedResources(context.Background())
	expectedCount := len(configCRGVRs) * len(configCRVerbs)
	if len(denied) != expectedCount {
		t.Fatalf("expected %d denied entries, got %d: %v", expectedCount, len(denied), denied)
	}
}

func TestRBACProbe_GVRsMatchRoleGrant(t *testing.T) {
	roleResources := map[string]struct{}{
		"log4j2configs": {}, "nodesettings": {}, "applicationproperties": {},
		"applicationoverrideproperties": {}, "bootstrapproperties": {},
		"nodepropertiesconfigs": {}, "throttlesconfigs": {}, "feeschedules": {},
		"simplefeesschedules": {}, "apipermissionproperties": {}, "blocknodesconfigs": {},
	}

	for _, gvr := range configCRGVRs {
		if _, ok := roleResources[gvr.Resource]; !ok {
			t.Errorf("configCRGVRs contains %q which is not in the role.go grant", gvr.Resource)
		}
		if gvr.Group != operatorv1alpha1.GroupVersion.Group {
			t.Errorf("GVR %q has wrong group %q", gvr.Resource, gvr.Group)
		}
	}

	if len(configCRGVRs) != len(roleResources) {
		t.Errorf("configCRGVRs has %d entries but role.go has %d", len(configCRGVRs), len(roleResources))
	}
}

func TestRequiredProbe_ClusterOnlyMode(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewSimpleDynamicClient(scheme)

	mon := NewUpgradeOutcomeMonitorWithClient(UpgradeOutcomeMonitorConfig{
		Namespace:               "solo",
		ProvisionerProxyEnabled: true,
		CapsuleName:             "node0",
	}, client)

	probe := mon.RequiredProbe()
	if probe == nil {
		t.Fatal("expected RequiredProbe to return a probe in cluster-only mode")
	}

	tagged, ok := probe.(*daemonkit.TaggedProbe)
	if !ok {
		t.Fatalf("expected *daemonkit.TaggedProbe, got %T", probe)
	}
	if tagged.Reason != ReasonConfigCRAccessDenied.String() {
		t.Errorf("unexpected reason: %s", tagged.Reason)
	}
}

func TestRequiredProbe_MainnetMode(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewSimpleDynamicClient(scheme)

	mon := NewUpgradeOutcomeMonitorWithClient(UpgradeOutcomeMonitorConfig{
		Namespace: "solo",
	}, client)

	probe := mon.RequiredProbe()
	if probe != nil {
		t.Fatal("expected RequiredProbe to return nil in mainnet mode")
	}
}

func TestConfigCRGVRs_AllUseOperatorGroup(t *testing.T) {
	for _, gvr := range configCRGVRs {
		expected := schema.GroupVersionResource{
			Group:    operatorv1alpha1.GroupVersion.Group,
			Version:  operatorv1alpha1.GroupVersion.Version,
			Resource: gvr.Resource,
		}
		if gvr != expected {
			t.Errorf("GVR mismatch for %s: got %v, want %v", gvr.Resource, gvr, expected)
		}
	}
}
