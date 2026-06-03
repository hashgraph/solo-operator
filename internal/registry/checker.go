// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	PlatformAMD64 = "linux/amd64"
	PlatformARM64 = "linux/arm64"
)

// imageSource is the seam that lets tests substitute a fake for the network.
// Inspect returns the image's index digest and the uncompressed layer hashes
// (config rootfs.diff_ids) for each requested "os/arch" platform.
type imageSource interface {
	Inspect(ctx context.Context, image string, platforms []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error)
}

// OCIChecker compares an image's per-platform layer hashes against the expected
// manifest values. It uses uncompressed diff_ids: deterministic builds produce
// identical diff_ids regardless of build environment, compressed digests do not.
type OCIChecker struct {
	Keychain authn.Keychain // defaults to authn.DefaultKeychain
	src      imageSource    // defaults to a remote source
}

func (c OCIChecker) source() imageSource {
	if c.src != nil {
		return c.src
	}
	kc := c.Keychain
	if kc == nil {
		kc = authn.DefaultKeychain
	}
	return remoteSource{keychain: kc}
}

func (c OCIChecker) Check(ctx context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, deterministic operatorv1alpha1.UpgradeDeterministicSpec) RegistryResult {
	res := RegistryResult{Image: spec.Image}

	// non-deterministic components carry per-registry overrides instead of the global block
	expected := deterministic.LayerHashes
	if !deterministic.Supported {
		expected = spec.LayerHashes
	}
	platforms := make([]string, 0, len(expected))
	for p := range expected {
		platforms = append(platforms, p)
	}

	digest, diffIDs, err := c.source().Inspect(ctx, spec.Image, platforms)
	if err != nil {
		res.State = StateUnreachable
		res.Err = err
		return res
	}
	res.Digest = digest

	if len(expected) == 0 {
		res.State = StateValid
		return res
	}

	for platform, want := range expected {
		if !slices.Equal(diffIDs[platform], want) {
			res.State = StateHashMismatch
			res.Err = fmt.Errorf("%s layer hash mismatch: got %v, want %v", platform, diffIDs[platform], want)
			return res
		}
	}

	res.State = StateValid
	res.LayerHashesVerified = true
	return res
}

type remoteSource struct {
	keychain authn.Keychain
}

func (s remoteSource) Inspect(ctx context.Context, image string, platforms []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", nil, fmt.Errorf("parsing image reference: %w", err)
	}
	opts := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(s.keychain)}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return "", nil, fmt.Errorf("resolving digest: %w", err)
	}
	digest := operatorv1alpha1.Sha256Digest(desc.Digest.String())

	// fetch platforms concurrently; any failure fails the whole inspection
	diffIDs := make(map[string][]operatorv1alpha1.Sha256Digest, len(platforms))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for _, platform := range platforms {
		g.Go(func() error {
			hashes, err := s.platformDiffIDs(gctx, ref, platform)
			if err != nil {
				return err
			}
			mu.Lock()
			diffIDs[platform] = hashes
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return "", nil, err
	}
	return digest, diffIDs, nil
}

func (s remoteSource) platformDiffIDs(ctx context.Context, ref name.Reference, platform string) ([]operatorv1alpha1.Sha256Digest, error) {
	plat, err := v1.ParsePlatform(platform)
	if err != nil {
		return nil, err
	}
	img, err := remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(s.keychain), remote.WithPlatform(*plat))
	if err != nil {
		return nil, fmt.Errorf("reading %s image: %w", platform, err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading %s config: %w", platform, err)
	}
	hashes := make([]operatorv1alpha1.Sha256Digest, len(cfg.RootFS.DiffIDs))
	for i, h := range cfg.RootFS.DiffIDs {
		hashes[i] = operatorv1alpha1.Sha256Digest(h.String())
	}
	return hashes, nil
}
