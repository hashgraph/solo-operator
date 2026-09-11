// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// wrapLikeInspect wraps err exactly the way remoteSource.Inspect does — errorx-decorated
// and tagged with the auth marker for 401/403 — so the test proves isAuthError classifies
// the production error shape (errorx's opaque wrapping otherwise hides the transport error).
func wrapLikeInspect(err error) error {
	return markAuth(err, errx.WithHints(
		errx.WithReason(errRegistryAPI.Wrap(err, "resolving digest"), ReasonDigestResolve),
		"Confirm the registry is reachable and the image exists",
	))
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401 marked", wrapLikeInspect(&transport.Error{StatusCode: http.StatusUnauthorized}), true},
		{"403 marked", wrapLikeInspect(&transport.Error{StatusCode: http.StatusForbidden}), true},
		{"500 is not auth", wrapLikeInspect(&transport.Error{StatusCode: http.StatusInternalServerError}), false},
		{"raw 401 without marker", &transport.Error{StatusCode: http.StatusUnauthorized}, false},
		{"plain error", errors.New("dial tcp: connection refused"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthError(tt.err); got != tt.want {
				t.Errorf("isAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// authRetrySource returns a 401 on the first Inspect and succeeds on the second, so the
// anonymous retry (which reuses this same test src) turns an auth failure into a result.
type authRetrySource struct {
	calls   int
	digest  operatorv1alpha1.Sha256Digest
	diffIDs map[string][]operatorv1alpha1.Sha256Digest
}

func (s *authRetrySource) Inspect(context.Context, string, []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error) {
	s.calls++
	if s.calls == 1 {
		return "", nil, wrapLikeInspect(&transport.Error{StatusCode: http.StatusUnauthorized})
	}
	return s.digest, s.diffIDs, nil
}

func TestCheck_RetriesAnonymouslyOnAuthError(t *testing.T) {
	src := &authRetrySource{
		digest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		diffIDs: platformHashes(amd64Hash, arm64Hash),
	}
	checker := checkerWith(src)

	res := checker.Check(context.Background(),
		operatorv1alpha1.ImageRegistrySpec{Image: "ghcr.io/x/cn:1.0"},
		operatorv1alpha1.ImageDeterministicSpec{Supported: true, LayerHashes: platformHashes(amd64Hash, arm64Hash)},
	)

	if src.calls != 2 {
		t.Fatalf("expected 2 Inspect calls (auth failure then anonymous retry), got %d", src.calls)
	}
	if res.State != StateValid || !res.LayerHashesVerified {
		t.Fatalf("expected StateValid after anonymous retry, got %s (err=%v)", res.State, res.Err)
	}
}

func TestCheck_NonAuthErrorDoesNotRetry(t *testing.T) {
	// A non-auth failure (500) must not trigger the anonymous retry.
	src := &countingErrSource{err: wrapLikeInspect(&transport.Error{StatusCode: http.StatusInternalServerError})}
	checker := checkerWith(src)

	res := checker.Check(context.Background(),
		operatorv1alpha1.ImageRegistrySpec{Image: "ghcr.io/x/cn:1.0"},
		operatorv1alpha1.ImageDeterministicSpec{Supported: true, LayerHashes: platformHashes(amd64Hash, arm64Hash)},
	)
	if src.calls != 1 {
		t.Fatalf("expected exactly 1 Inspect call for a non-auth error, got %d", src.calls)
	}
	if res.State != StateUnreachable {
		t.Fatalf("expected StateUnreachable, got %s", res.State)
	}
}

type countingErrSource struct {
	calls int
	err   error
}

func (s *countingErrSource) Inspect(context.Context, string, []string) (operatorv1alpha1.Sha256Digest, map[string][]operatorv1alpha1.Sha256Digest, error) {
	s.calls++
	return "", nil, s.err
}
