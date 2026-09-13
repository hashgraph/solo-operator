// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func makeImages(cnImage string) operatorv1alpha1.UpgradeImagesSpec {
	return operatorv1alpha1.UpgradeImagesSpec{
		ConsensusNode: operatorv1alpha1.ImageComponentSpec{
			Version:    "0.75.0",
			Registries: []operatorv1alpha1.ImageRegistrySpec{{Image: cnImage}, {Image: "reg-b/cn:0.75.0"}},
		},
	}
}

func makeSelected(cnImage string) operatorv1alpha1.SelectedSources {
	return operatorv1alpha1.SelectedSources{
		ConsensusNode: &operatorv1alpha1.SelectedSource{Image: cnImage, Digest: testDigest},
	}
}

func testRng() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func TestRevalidateComponents_AllValid(t *testing.T) {
	checker := &fakeChecker{}
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")

	updated, ok := RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "", nil, nil, nil)

	if !ok {
		t.Fatal("expected allSucceeded=true")
	}
	if updated.ConsensusNode == nil {
		t.Fatal("expected ConsensusNode in updated SelectedSources")
	}
	if updated.ConsensusNode.Image != "reg-a/cn:0.75.0" {
		t.Errorf("expected original image preserved, got %s", updated.ConsensusNode.Image)
	}
}

func TestRevalidateComponents_Failover(t *testing.T) {
	checker := &fakeChecker{fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		if spec.Image == "reg-a/cn:0.75.0" {
			return RegistryResult{Image: spec.Image, State: StateUnreachable}
		}
		return RegistryResult{Image: spec.Image, State: StateValid, Digest: altDigest}
	}}
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")

	var failoverName operatorv1alpha1.ComponentName
	var failoverResult FailoverResult
	updated, ok := RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "",
		nil,
		func(name operatorv1alpha1.ComponentName, res FailoverResult) {
			failoverName = name
			failoverResult = res
		},
		nil,
	)

	if !ok {
		t.Fatal("expected allSucceeded=true after failover")
	}
	if failoverName != operatorv1alpha1.ComponentConsensusNode {
		t.Errorf("expected failover on consensusNode, got %s", failoverName)
	}
	if failoverResult.FailedImage != "reg-a/cn:0.75.0" {
		t.Errorf("expected FailedImage=reg-a/cn:0.75.0, got %s", failoverResult.FailedImage)
	}
	if updated.ConsensusNode == nil || updated.ConsensusNode.Image != "reg-b/cn:0.75.0" {
		t.Errorf("expected failover to reg-b/cn:0.75.0, got %v", updated.ConsensusNode)
	}
}

func TestRevalidateComponents_NoValidRegistry(t *testing.T) {
	checker := &fakeChecker{fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		return RegistryResult{Image: spec.Image, State: StateUnreachable}
	}}
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")

	var invalidName operatorv1alpha1.ComponentName
	var invalidErr error
	updated, ok := RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "",
		nil, nil,
		func(name operatorv1alpha1.ComponentName, err error) {
			invalidName = name
			invalidErr = err
		},
	)

	if ok {
		t.Fatal("expected allSucceeded=false")
	}
	if invalidName != operatorv1alpha1.ComponentConsensusNode {
		t.Errorf("expected onInvalid called with consensusNode, got %s", invalidName)
	}
	if invalidErr == nil {
		t.Error("expected non-nil error in onInvalid")
	}
	if updated.ConsensusNode != nil {
		t.Error("expected nil ConsensusNode in updated sources on failure")
	}
}

func TestRevalidateComponents_SkipComponent(t *testing.T) {
	called := false
	checker := &fakeChecker{fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		called = true
		return RegistryResult{Image: spec.Image, State: StateValid, Digest: testDigest}
	}}
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")
	skip := map[operatorv1alpha1.ComponentName]bool{operatorv1alpha1.ComponentConsensusNode: true}

	_, ok := RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "", skip, nil, nil)

	if !ok {
		t.Fatal("expected allSucceeded=true when only component is skipped")
	}
	if called {
		t.Error("expected checker not called for skipped component")
	}
}

func TestRevalidateComponents_OmittedComponentIsNoOp(t *testing.T) {
	checker := &fakeChecker{}
	// No RecordStreamUploader in images — should be silently skipped, no error.
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")

	_, ok := RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "", nil, nil, nil)

	if !ok {
		t.Fatal("expected allSucceeded=true when optional components are omitted")
	}
}

func TestRevalidateComponents_NilCallbacksDoNotPanic(t *testing.T) {
	checker := &fakeChecker{fn: func(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec) RegistryResult {
		return RegistryResult{Image: spec.Image, State: StateUnreachable}
	}}
	images := makeImages("reg-a/cn:0.75.0")
	selected := makeSelected("reg-a/cn:0.75.0")

	// Should not panic even with nil callbacks and a failing registry.
	_, _ = RevalidateComponents(context.Background(), checker, images, selected,
		time.Second, testRng(), "", nil, nil, nil)
}

// Sentinel error used in onInvalid assertion above; verified non-nil is enough.
var _ = errors.New
