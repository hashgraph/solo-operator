// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
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

	AllowedHosts []string // SSRF allow-list; empty denies only private/loopback hosts
}

func (c OCIChecker) sourceWith(kc authn.Keychain) imageSource {
	if c.src != nil {
		return c.src
	}
	// guardedTransport enforces the SSRF policy at connect time (see dialguard.go),
	// authoritatively for DNS-resolved hosts and registry redirects alike.
	return remoteSource{keychain: kc, transport: guardedTransport(c.AllowedHosts)}
}

func (c OCIChecker) source() imageSource {
	kc := c.Keychain
	if kc == nil {
		kc = authn.DefaultKeychain
	}
	return c.sourceWith(kc)
}

// anonymousSource inspects with no credential. It is the retry path when a configured
// pull secret is rejected (401/403) but the image may be public and pullable anonymously.
func (c OCIChecker) anonymousSource() imageSource {
	return c.sourceWith(anonymousKeychain{})
}

// anonymousKeychain resolves every registry to anonymous access.
type anonymousKeychain struct{}

func (anonymousKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

// authError marks a registry authentication/authorization failure (HTTP 401/403) so the
// checker can trigger an anonymous retry. It is needed because Inspect decorates errors
// with errorx's opaque wrapping, which hides the underlying transport.Error from
// errors.As — so the classification is done at the raw level (isRawAuthError) and carried
// out on this marker. It preserves the decorated error for message/reason/hints.
type authError struct{ error }

func (e authError) Unwrap() error { return e.error }

// markAuth tags decorated with the auth marker when the raw error is a 401/403, so the
// checker retries anonymously; otherwise it returns decorated unchanged.
func markAuth(raw, decorated error) error {
	if isRawAuthError(raw) {
		return authError{decorated}
	}
	return decorated
}

// isAuthError reports whether err carries the auth marker (a rejected credential).
func isAuthError(err error) bool {
	var ae authError
	return errors.As(err, &ae)
}

// unwrapAuth strips the auth marker, returning the underlying decorated error so callers
// surface the usual reason/hints.
func unwrapAuth(err error) error {
	var ae authError
	if errors.As(err, &ae) {
		return ae.error
	}
	return err
}

// isRawAuthError classifies a raw go-containerregistry error as a 401/403.
func isRawAuthError(err error) bool {
	var terr *transport.Error
	return errors.As(err, &terr) && (terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusForbidden)
}

func (c OCIChecker) Check(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec, deterministic operatorv1alpha1.ImageDeterministicSpec) RegistryResult {
	res := RegistryResult{Image: spec.Image}

	if err := checkHostAllowed(spec.Image, c.AllowedHosts); err != nil {
		res.State = StateDenied
		res.Err = err
		return res
	}

	// non-deterministic components carry per-registry overrides instead of the global block
	expected := deterministic.LayerHashes
	if !deterministic.Supported {
		expected = spec.LayerHashes
	}

	// Supported=false with no hashes is a deliberate opt-out; still reject a malformed reference.
	if !deterministic.Supported && len(expected) == 0 {
		if _, err := name.ParseReference(spec.Image); err != nil {
			res.State = StateUnreachable
			res.Err = err
			return res
		}
		res.State = StateValid
		return res
	}

	platforms := make([]string, 0, len(expected))
	for p := range expected {
		platforms = append(platforms, p)
	}

	digest, diffIDs, err := c.source().Inspect(ctx, spec.Image, platforms)
	if err != nil && isAuthError(err) {
		// A configured pull secret was rejected (401/403). The image may be public and
		// pullable anonymously — the kubelet falls back to anonymous too — so retry
		// without the credential before declaring the registry unreachable. Warn so a
		// stale/misscoped credential is still visible even when the retry succeeds.
		common.Warn(ctx, ReasonCredentialRejected).Str("image", spec.Image).AnErr("error", err).
			Msg("configured pull secret was rejected (401/403); retrying anonymously")
		digest, diffIDs, err = c.anonymousSource().Inspect(ctx, spec.Image, platforms)
	}
	if err != nil {
		res.State = StateUnreachable
		res.Err = unwrapAuth(err)
		return res
	}
	res.Digest = digest

	if len(expected) == 0 {
		res.State = StateNoHashes
		res.Err = errx.WithHints(
			errx.WithReason(errIntegrity.New("image %s has no layer hashes configured: cannot verify image integrity", spec.Image), ReasonNoLayerHashes),
			"Add layerHashes for this image to the deployment manifest",
		)
		return res
	}

	for platform, want := range expected {
		if !slices.Equal(diffIDs[platform], want) {
			res.State = StateHashMismatch
			res.Err = errx.WithHints(
				errx.WithReason(errIntegrity.New("%s layer hash mismatch: got %v, want %v", platform, diffIDs[platform], want), ReasonLayerHashMismatch),
				"Confirm the published image matches the expected layerHashes, or update the manifest",
			)
			return res
		}
	}

	res.State = StateValid
	res.LayerHashesVerified = true
	return res
}

type remoteSource struct {
	keychain  authn.Keychain
	transport http.RoundTripper
}

// opts assembles the remote options shared by every registry call, including the
// connect-time SSRF guard transport when one is configured.
func (s remoteSource) opts(ctx context.Context) []remote.Option {
	o := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(s.keychain)}
	if s.transport != nil {
		o = append(o, remote.WithTransport(s.transport))
	}
	return o
}

func (s remoteSource) Inspect(ctx context.Context, image string, platforms []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", nil, errx.WithHints(
			errx.WithReason(errImageRef.Wrap(err, "parsing image reference %q", image), ReasonImageRefInvalid),
			"Confirm the image reference is a valid registry/repository:tag",
		)
	}
	opts := s.opts(ctx)

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return "", nil, markAuth(err, errx.WithHints(
			errx.WithReason(errRegistryAPI.Wrap(err, "resolving digest"), ReasonDigestResolve),
			"Confirm the registry is reachable and the image exists",
			"Check pull credentials for the registry",
		))
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
	img, err := remote.Image(ref, append(s.opts(ctx), remote.WithPlatform(*plat))...)
	if err != nil {
		return nil, markAuth(err, errx.WithHints(
			errx.WithReason(errRegistryAPI.Wrap(err, "reading %s image", platform), ReasonImageRead),
			"Confirm the registry is reachable and the image provides the requested platform",
		))
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, markAuth(err, errx.WithHints(
			errx.WithReason(errRegistryAPI.Wrap(err, "reading %s config", platform), ReasonConfigRead),
			"Confirm the registry is reachable and the image config is readable",
		))
	}
	hashes := make([]operatorv1alpha1.Sha256Digest, len(cfg.RootFS.DiffIDs))
	for i, h := range cfg.RootFS.DiffIDs {
		hashes[i] = operatorv1alpha1.Sha256Digest(h.String())
	}
	return hashes, nil
}
