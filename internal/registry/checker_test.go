// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"regexp"
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var digestRe = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func TestOCIChecker_Check_Reachable(t *testing.T) {
	spec := operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:0.62.0"}

	res := OCIChecker{}.Check(context.Background(), spec, operatorv1alpha1.UpgradeDeterministicSpec{})

	if !res.Reachable || res.Err != nil {
		t.Fatalf("expected reachable with no error, got %+v", res)
	}
	if res.Image != spec.Image {
		t.Errorf("image = %q, want %q", res.Image, spec.Image)
	}
	if !digestRe.MatchString(string(res.Digest)) {
		t.Errorf("digest %q is not a valid sha256 digest", res.Digest)
	}

	// Deterministic: same image yields the same digest.
	again := OCIChecker{}.Check(context.Background(), spec, operatorv1alpha1.UpgradeDeterministicSpec{})
	if again.Digest != res.Digest {
		t.Errorf("digest not deterministic: %q != %q", again.Digest, res.Digest)
	}
}

func TestOCIChecker_Check_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := OCIChecker{}.Check(ctx, operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:0.62.0"}, operatorv1alpha1.UpgradeDeterministicSpec{})

	if res.Reachable || res.Err == nil {
		t.Errorf("expected unreachable with error on cancelled context, got %+v", res)
	}
}
