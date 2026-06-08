// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const testDigest = operatorv1alpha1.Sha256Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
const altDigest = operatorv1alpha1.Sha256Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

func compWithRegistries(images ...string) operatorv1alpha1.UpgradeComponentSpec {
	regs := make([]operatorv1alpha1.UpgradeRegistrySpec, len(images))
	for i, img := range images {
		regs[i] = operatorv1alpha1.UpgradeRegistrySpec{Image: img}
	}
	return operatorv1alpha1.UpgradeComponentSpec{Registries: regs}
}

func currentSource(image string) operatorv1alpha1.SelectedSource {
	return operatorv1alpha1.SelectedSource{Image: image, Digest: testDigest, LayerHashesVerified: true}
}

func TestRevalidateAndFailover_StillValid(t *testing.T) {
	checker := &fakeChecker{}
	comp := compWithRegistries("img-a", "img-b")
	current := currentSource("img-a")

	res, err := RevalidateAndFailover(context.Background(), checker, comp, current, time.Second, newRNG(1), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DidFailover {
		t.Error("expected no failover when original is still valid")
	}
	if res.Selected.Image != "img-a" {
		t.Errorf("expected original image, got %s", res.Selected.Image)
	}
}

func TestRevalidateAndFailover_FailoverToAlternate(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec) RegistryResult {
			if spec.Image == "img-a" {
				return RegistryResult{Image: spec.Image, State: StateUnreachable}
			}
			return RegistryResult{Image: spec.Image, State: StateValid, Digest: altDigest, LayerHashesVerified: true}
		},
	}
	comp := compWithRegistries("img-a", "img-b")
	current := currentSource("img-a")

	res, err := RevalidateAndFailover(context.Background(), checker, comp, current, time.Second, newRNG(1), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.DidFailover {
		t.Error("expected failover when original is unreachable")
	}
	if res.FailedImage != "img-a" {
		t.Errorf("expected FailedImage=img-a, got %s", res.FailedImage)
	}
	if res.Selected.Image != "img-b" {
		t.Errorf("expected replacement img-b, got %s", res.Selected.Image)
	}
	if res.Selected.Digest != altDigest {
		t.Errorf("expected replacement digest to be recorded")
	}
}

func TestRevalidateAndFailover_NoAlternateAvailable(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec) RegistryResult {
			return RegistryResult{Image: spec.Image, State: StateUnreachable}
		},
	}
	comp := compWithRegistries("img-a", "img-b")
	current := currentSource("img-a")

	_, err := RevalidateAndFailover(context.Background(), checker, comp, current, time.Second, newRNG(1), "")
	if err == nil {
		t.Error("expected error when no valid registry exists")
	}
}

func TestRevalidateAndFailover_HashMismatchTreatedAsInvalid(t *testing.T) {
	checker := &fakeChecker{
		fn: func(_ context.Context, spec operatorv1alpha1.UpgradeRegistrySpec) RegistryResult {
			if spec.Image == "img-a" {
				return RegistryResult{Image: spec.Image, State: StateHashMismatch}
			}
			return RegistryResult{Image: spec.Image, State: StateValid, Digest: altDigest}
		},
	}
	comp := compWithRegistries("img-a", "img-b")
	current := currentSource("img-a")

	res, err := RevalidateAndFailover(context.Background(), checker, comp, current, time.Second, newRNG(1), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.DidFailover {
		t.Error("expected failover on hash mismatch")
	}
	if res.Selected.Image != "img-b" {
		t.Errorf("expected img-b as replacement, got %s", res.Selected.Image)
	}
}
