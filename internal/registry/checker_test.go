// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// fakeSource is a stand-in imageSource for unit tests.
type fakeSource struct {
	digest  operatorv1alpha1.Sha256Digest
	diffIDs map[string][]operatorv1alpha1.Sha256Digest // platform → hashes
	err     error
}

func (f fakeSource) Inspect(context.Context, string, []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error) {
	return f.digest, f.diffIDs, f.err
}

func checkerWith(src imageSource) OCIChecker { return OCIChecker{src: src} }

const (
	amd64Hash = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	arm64Hash = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

func platformHashes(amd64, arm64 operatorv1alpha1.Sha256Digest) map[string][]operatorv1alpha1.Sha256Digest {
	return map[string][]operatorv1alpha1.Sha256Digest{
		PlatformAMD64: {amd64},
		PlatformARM64: {arm64},
	}
}

func bothPlatforms() map[string][]operatorv1alpha1.Sha256Digest {
	return platformHashes(amd64Hash, arm64Hash)
}

func TestOCIChecker_Deterministic_Valid(t *testing.T) {
	src := fakeSource{digest: "sha256:abc", diffIDs: bothPlatforms()}
	det := operatorv1alpha1.UpgradeDeterministicSpec{Supported: true, LayerHashes: bothPlatforms()}

	res := checkerWith(src).Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1"}, det)

	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("expected valid+verified, got %+v", res)
	}
	if res.Digest != "sha256:abc" {
		t.Errorf("digest = %q", res.Digest)
	}
}

func TestOCIChecker_Deterministic_HashMismatch(t *testing.T) {
	served := bothPlatforms()
	served[PlatformARM64] = []operatorv1alpha1.Sha256Digest{"sha256:deadbeef"} // tampered
	src := fakeSource{digest: "sha256:abc", diffIDs: served}
	det := operatorv1alpha1.UpgradeDeterministicSpec{Supported: true, LayerHashes: bothPlatforms()}

	res := checkerWith(src).Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1"}, det)

	if res.State != StateHashMismatch || res.Err == nil {
		t.Fatalf("expected hash mismatch, got %+v", res)
	}
	if res.LayerHashesVerified {
		t.Error("LayerHashesVerified must be false on mismatch")
	}
}

// Story 5.3: non-deterministic components use the per-registry overrides, not
// the (empty) global deterministic block.
func TestOCIChecker_NonDeterministic_UsesPerRegistryHashes(t *testing.T) {
	src := fakeSource{digest: "sha256:abc", diffIDs: bothPlatforms()}
	spec := operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1", LayerHashes: bothPlatforms()}
	det := operatorv1alpha1.UpgradeDeterministicSpec{Supported: false}

	res := checkerWith(src).Check(context.Background(), spec, det)

	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("expected valid+verified from per-registry hashes, got %+v", res)
	}
}

func TestOCIChecker_NonDeterministic_MismatchAgainstOverride(t *testing.T) {
	served := bothPlatforms()
	served[PlatformAMD64] = []operatorv1alpha1.Sha256Digest{"sha256:deadbeef"}
	src := fakeSource{digest: "sha256:abc", diffIDs: served}
	spec := operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1", LayerHashes: bothPlatforms()}

	res := checkerWith(src).Check(context.Background(), spec, operatorv1alpha1.UpgradeDeterministicSpec{Supported: false})

	if res.State != StateHashMismatch {
		t.Fatalf("expected hash mismatch against override, got %+v", res)
	}
}

// Story 5.3: non-deterministic components must use ONLY the per-registry
// overrides, even when a global deterministic block is also present.
func TestOCIChecker_NonDeterministic_IgnoresGlobalHashes(t *testing.T) {
	src := fakeSource{digest: "sha256:abc", diffIDs: bothPlatforms()}
	spec := operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1", LayerHashes: bothPlatforms()}
	det := operatorv1alpha1.UpgradeDeterministicSpec{
		Supported:   false,
		LayerHashes: map[string][]operatorv1alpha1.Sha256Digest{PlatformAMD64: {"sha256:deadbeef"}}, // would fail if wrongly used
	}

	res := checkerWith(src).Check(context.Background(), spec, det)

	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("non-deterministic must verify against per-registry hashes, not the global block; got %+v", res)
	}
}

func TestOCIChecker_NoExpectedHashes_Error(t *testing.T) {
	src := fakeSource{digest: "sha256:abc"}
	res := checkerWith(src).Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1"},
		operatorv1alpha1.UpgradeDeterministicSpec{Supported: true})

	if res.State != StateNoHashes || res.Err == nil {
		t.Fatalf("expected StateNoHashes with error when no hashes configured, got %+v", res)
	}
	if res.LayerHashesVerified {
		t.Error("LayerHashesVerified must be false when no hashes configured")
	}
}

func TestOCIChecker_Unreachable(t *testing.T) {
	src := fakeSource{err: fmt.Errorf("connection refused")}
	res := checkerWith(src).Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: "gcr.io/a/cn:1"},
		operatorv1alpha1.UpgradeDeterministicSpec{Supported: true, LayerHashes: bothPlatforms()})

	if res.State != StateUnreachable || res.Err == nil {
		t.Fatalf("expected unreachable, got %+v", res)
	}
}

func TestOCIChecker_BadReference(t *testing.T) {
	res := OCIChecker{}.Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: "::not a ref::"},
		operatorv1alpha1.UpgradeDeterministicSpec{})

	if res.State != StateUnreachable {
		t.Fatalf("expected unreachable for bad reference, got %+v", res)
	}
}

// Exercises the real remoteSource against an in-memory registry: push an image,
// then confirm its diff_ids are extracted and verified end-to-end.
func TestRemoteSource_DiffIDsAgainstInMemoryRegistry(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()
	host := mustHost(t, srv.URL)

	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = "linux", "amd64"
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("set platform: %v", err)
	}

	ref, err := name.ParseReference(host + "/cn:1")
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("push: %v", err)
	}

	want := make([]operatorv1alpha1.Sha256Digest, len(cfg.RootFS.DiffIDs))
	for i, h := range cfg.RootFS.DiffIDs {
		want[i] = operatorv1alpha1.Sha256Digest(h.String())
	}

	checker := OCIChecker{AllowedHosts: []string{mustHostname(t, host)}} // 127.0.0.1 is denied by default
	det := operatorv1alpha1.UpgradeDeterministicSpec{
		Supported:   true,
		LayerHashes: map[string][]operatorv1alpha1.Sha256Digest{PlatformAMD64: want},
	}
	res := checker.Check(context.Background(), operatorv1alpha1.UpgradeRegistrySpec{Image: ref.Name()}, det)

	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("expected valid+verified from in-memory registry, got %+v (err: %v)", res, res.Err)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return u.Host
}

func mustHostname(t *testing.T, hostPort string) string {
	t.Helper()
	h, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split host:port %q: %v", hostPort, err)
	}
	return h
}
