// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// Pinned public image with known, stable diff_ids; registry.k8s.io isn't rate-limited.
const (
	publicImage    = "registry.k8s.io/pause:3.10"
	pauseAMD64Diff = "sha256:d8bdedd33a4e22f5cb6ca641a7ada5f9e6a92ad001589480818a136184c7ce34"
	pauseARM64Diff = "sha256:1975aab740f75a0e29b0dc8da5e898fcd3d70a5792becd9a847b4c7515cf154f"
)

func pauseHashes() map[string][]operatorv1alpha1.Sha256Digest {
	return platformHashes(pauseAMD64Diff, pauseARM64Diff)
}

func skipIfUnreachable(t *testing.T, res RegistryResult) {
	t.Helper()
	if res.State == StateUnreachable {
		t.Skipf("registry unreachable: %v", res.Err)
	}
}

// Verifies a real public image's layer hashes; skips when offline.
func TestOCIChecker_PublicImage_Verifies(t *testing.T) {
	det := operatorv1alpha1.ImageDeterministicSpec{Supported: true, LayerHashes: pauseHashes()}

	res := OCIChecker{}.Check(context.Background(), operatorv1alpha1.ImageRegistrySpec{Image: publicImage}, det)

	skipIfUnreachable(t, res)
	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("expected valid+verified for pinned public image, got %+v", res)
	}
}

func TestOCIChecker_PublicImage_DetectsTamper(t *testing.T) {
	bad := pauseHashes()
	bad[PlatformAMD64] = []operatorv1alpha1.Sha256Digest{"sha256:0000000000000000000000000000000000000000000000000000000000000000"}
	det := operatorv1alpha1.ImageDeterministicSpec{Supported: true, LayerHashes: bad}

	res := OCIChecker{}.Check(context.Background(), operatorv1alpha1.ImageRegistrySpec{Image: publicImage}, det)

	skipIfUnreachable(t, res)
	if res.State != StateHashMismatch {
		t.Fatalf("expected hash mismatch for tampered hashes, got %+v", res)
	}
}
