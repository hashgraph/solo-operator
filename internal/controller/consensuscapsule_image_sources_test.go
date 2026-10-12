// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

const testResolvedDigest = operatorv1alpha1.Sha256Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

// fakeImageChecker implements registry.ImageChecker for resolveImageSources tests.
type fakeImageChecker struct {
	calls int
	fn    func(spec operatorv1alpha1.ImageRegistrySpec) registry.RegistryResult
}

func (f *fakeImageChecker) Check(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec, _ operatorv1alpha1.ImageDeterministicSpec) registry.RegistryResult {
	f.calls++
	if f.fn != nil {
		return f.fn(spec)
	}
	return registry.RegistryResult{Image: spec.Image, State: registry.StateValid, Digest: testResolvedDigest, LayerHashesVerified: true}
}

func capsuleWithConsensusSource() *operatorv1alpha1.ConsensusCapsule {
	c := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test", Generation: 1},
	}
	c.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersionSource = &operatorv1alpha1.SoftwareVersionSource{
		ImageRepositories: []operatorv1alpha1.ImageRepository{
			{Repository: "ghcr.io/hashgraph", ImageName: "consensus-node", ImageTag: "0.75.0"},
		},
		ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{
			{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:aaa"}},
		},
	}
	return c
}

func newResolveReconciler(checker registry.ImageChecker, capsule *operatorv1alpha1.ConsensusCapsule) *ConsensusCapsuleReconciler {
	scheme := newControllerTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(capsule).WithStatusSubresource(capsule).Build()
	return &ConsensusCapsuleReconciler{Client: c, Scheme: scheme, Checker: checker}
}

func TestResolveImageSources_ResolvesAndPins(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil || res != nil {
		t.Fatalf("expected continue (nil,nil), got res=%v err=%v", res, err)
	}
	sv := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
	if sv == nil {
		t.Fatal("expected SoftwareVersion to be populated")
	}
	if sv.Digest != testResolvedDigest {
		t.Errorf("digest = %q, want %q", sv.Digest, testResolvedDigest)
	}
	if sv.Repository != "ghcr.io/hashgraph" || sv.ImageName != "consensus-node" || sv.ImageTag != "0.75.0" {
		t.Errorf("unexpected pinned image: %+v", sv)
	}
	entry := findResolvedImage(capsule, "consensusNode")
	if entry == nil || entry.Validity != operatorv1alpha1.ImageValidityValid || entry.Digest != testResolvedDigest {
		t.Errorf("unexpected status entry: %+v", entry)
	}
}

func TestResolveImageSources_CacheHitSkipsChecker(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	src := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersionSource
	hash, err := sourceHash(src)
	if err != nil {
		t.Fatalf("sourceHash: %v", err)
	}
	capsule.Status.ResolvedImages = []operatorv1alpha1.ResolvedImageStatus{{
		Container:  "consensusNode",
		SourceHash: hash,
		Image:      "ghcr.io/hashgraph/consensus-node:0.75.0@" + string(testResolvedDigest),
		Digest:     testResolvedDigest,
		Repository: "ghcr.io/hashgraph",
		Validity:   operatorv1alpha1.ImageValidityValid,
	}}
	checker := &fakeImageChecker{}
	r := newResolveReconciler(checker, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil || res != nil {
		t.Fatalf("expected continue, got res=%v err=%v", res, err)
	}
	if checker.calls != 0 {
		t.Errorf("expected checker not called on cache hit, got %d calls", checker.calls)
	}
	sv := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
	if sv == nil || sv.Digest != testResolvedDigest {
		t.Errorf("expected pinned SoftwareVersion from cache, got %+v", sv)
	}
}

func TestResolveImageSources_SourceChangedReResolves(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	capsule.Status.ResolvedImages = []operatorv1alpha1.ResolvedImageStatus{{
		Container:  "consensusNode",
		SourceHash: "stale-hash",
		Digest:     testResolvedDigest,
		Validity:   operatorv1alpha1.ImageValidityValid,
	}}
	checker := &fakeImageChecker{}
	r := newResolveReconciler(checker, capsule)

	if _, err := r.resolveImageSources(context.Background(), capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checker.calls == 0 {
		t.Error("expected checker to be called when source hash changed")
	}
}

func TestResolveImageSources_MissingHashesIsTerminal(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersionSource.ImageVerificationSpec = nil
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a stop result for a source without layer hashes")
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("phase = %q, want Failed", capsule.Status.Phase)
	}
}

func TestResolveImageSources_MismatchedCandidateTagsIsTerminal(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	// Add a second candidate registry carrying a different version tag. A deterministic
	// source must publish one version to every registry, so this is a terminal config error.
	src := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersionSource
	src.ImageRepositories = append(src.ImageRepositories, operatorv1alpha1.ImageRepository{
		Repository: "docker.io/hashgraph", ImageName: "consensus-node", ImageTag: "0.76.0",
	})
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a stop result for a source with differing candidate tags")
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("phase = %q, want Failed", capsule.Status.Phase)
	}
}

func TestResolveImageSources_SidecarMismatchedCandidateTagsIsTerminal(t *testing.T) {
	// The intra-source tag check runs for every sourced container, not just the
	// consensus node. A UC sidecar whose candidate tags disagree is terminal.
	capsule := capsuleWithConsensusSource() // valid consensus-node source
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
		Enabled: true,
		SoftwareVersionSource: &operatorv1alpha1.SoftwareVersionSource{
			ImageRepositories: []operatorv1alpha1.ImageRepository{
				{Repository: "ghcr.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "1.0.0"},
				{Repository: "docker.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "1.1.0"},
			},
			ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{
				{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:bbb"}},
			},
		},
	}
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a stop result for a sidecar source with differing candidate tags")
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("phase = %q, want Failed", capsule.Status.Phase)
	}
}

func TestResolveImageSources_LatestCandidateTagIsTerminal(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	// Pin every candidate to the mutable "latest" tag. The tags agree (no mismatch), but
	// "latest" is a mutable reference, so it is a terminal config error.
	src := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersionSource
	for i := range src.ImageRepositories {
		src.ImageRepositories[i].ImageTag = "latest"
	}
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected a stop result for a source pinned to latest")
	}
	if capsule.Status.Phase != operatorv1alpha1.PhaseFailed {
		t.Errorf("phase = %q, want Failed", capsule.Status.Phase)
	}
}

func TestLatestTagInSource(t *testing.T) {
	repo := func(tag string) operatorv1alpha1.ImageRepository {
		return operatorv1alpha1.ImageRepository{Repository: "r", ImageName: "n", ImageTag: tag}
	}
	tests := []struct {
		name       string
		src        *operatorv1alpha1.SoftwareVersionSource
		want       string
		wantLatest bool
	}{
		{"nil source", nil, "", false},
		{"no latest", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("0.75.0")}}, "", false},
		{"first is latest", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("latest"), repo("0.75.0")}}, "latest", true},
		{"later is latest", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("latest")}}, "latest", true},
		// Case-insensitive: the mutable tag is rejected regardless of casing.
		{"mixed case latest", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("Latest")}}, "Latest", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, latest := latestTagInSource(tt.src)
			if got != tt.want || latest != tt.wantLatest {
				t.Errorf("latestTagInSource() = (%q, %v), want (%q, %v)", got, latest, tt.want, tt.wantLatest)
			}
		})
	}
}

func TestMismatchedSourceTag(t *testing.T) {
	repo := func(tag string) operatorv1alpha1.ImageRepository {
		return operatorv1alpha1.ImageRepository{Repository: "r", ImageName: "n", ImageTag: tag}
	}
	tests := []struct {
		name         string
		src          *operatorv1alpha1.SoftwareVersionSource
		want         string
		wantMismatch bool
	}{
		{"nil source", nil, "", false},
		{"empty list", &operatorv1alpha1.SoftwareVersionSource{}, "", false},
		{"single candidate", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0")}}, "", false},
		{"all match", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("0.75.0"), repo("0.75.0")}}, "", false},
		{"second differs", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("0.76.0")}}, "0.76.0", true},
		{"last differs", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("0.75.0"), repo("0.77.0")}}, "0.77.0", true},
		// A later candidate with an empty tag differs from a non-empty first tag: the
		// returned string is "" but it must still be flagged as a mismatch.
		{"empty tag differs", &operatorv1alpha1.SoftwareVersionSource{ImageRepositories: []operatorv1alpha1.ImageRepository{repo("0.75.0"), repo("")}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mismatch := mismatchedSourceTag(tt.src)
			if got != tt.want || mismatch != tt.wantMismatch {
				t.Errorf("mismatchedSourceTag() = (%q, %v), want (%q, %v)", got, mismatch, tt.want, tt.wantMismatch)
			}
		})
	}
}

func TestResolveImageSources_NoValidRegistryRequeues(t *testing.T) {
	capsule := capsuleWithConsensusSource()
	checker := &fakeImageChecker{fn: func(spec operatorv1alpha1.ImageRegistrySpec) registry.RegistryResult {
		return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable}
	}}
	r := newResolveReconciler(checker, capsule)

	res, err := r.resolveImageSources(context.Background(), capsule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.RequeueAfter == 0 {
		t.Fatalf("expected a requeue result, got %v", res)
	}
	if capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion != nil {
		t.Error("expected no pin when no valid registry")
	}
	entry := findResolvedImage(capsule, "consensusNode")
	if entry == nil || entry.Validity != operatorv1alpha1.ImageValidityInvalid {
		t.Errorf("expected Invalid status entry, got %+v", entry)
	}
}

func TestResolveImageSources_PrunesRemovedSource(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test", Generation: 1},
	}
	// No source on any container, but a stale status entry exists.
	capsule.Status.ResolvedImages = []operatorv1alpha1.ResolvedImageStatus{{
		Container: "consensusNode", Validity: operatorv1alpha1.ImageValidityValid,
	}}
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	if _, err := r.resolveImageSources(context.Background(), capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capsule.Status.ResolvedImages) != 0 {
		t.Errorf("expected stale status entry pruned, got %+v", capsule.Status.ResolvedImages)
	}
}

func TestResolveImageSources_SourceWinsOverSoftwareVersion(t *testing.T) {
	// Defense-in-depth: the CRD makes softwareVersion/softwareVersionSource mutually
	// exclusive, but if both ever reach the controller the resolved source still wins
	// (the operator deploys the verified, digest-pinned source image).
	capsule := capsuleWithConsensusSource()
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
		Repository: "old", ImageName: "old", ImageTag: "old",
	}
	r := newResolveReconciler(&fakeImageChecker{}, capsule)

	if _, err := r.resolveImageSources(context.Background(), capsule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sv := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
	if sv.Repository == "old" || sv.Digest != testResolvedDigest {
		t.Errorf("expected source to win, got %+v", sv)
	}
}
