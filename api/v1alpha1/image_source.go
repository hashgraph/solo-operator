// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ToImageComponentSpec converts a capsule-facing SoftwareVersionSource into the
// generic ImageComponentSpec consumed by the registry selection engine
// (registry.SelectComponent). Each ImageRepository becomes a candidate registry
// (referenced by tag; selection resolves the immutable digest), and the shared
// ImageVerificationSpec list becomes the deterministic per-platform layer hashes
// every candidate must match — so a capsule deploy selects and verifies images
// through exactly the same path as a network upgrade.
//
// The result is used transiently for selection only (never persisted as a CRD),
// so Version is left empty (it carries no meaning off the upgrade path).
func (s *SoftwareVersionSource) ToImageComponentSpec() ImageComponentSpec {
	registries := make([]ImageRegistrySpec, 0, len(s.ImageRepositories))
	for i := range s.ImageRepositories {
		registries = append(registries, ImageRegistrySpec{Image: s.ImageRepositories[i].Ref()})
	}
	return ImageComponentSpec{
		Deterministic: ImageDeterministicSpec{
			Supported:   true,
			LayerHashes: platformLayerHashes(s.ImageVerificationSpec),
		},
		Registries:        registries,
		SelectionStrategy: s.SelectionStrategy,
	}
}

// Ref renders the repository as a "repository/imageName:imageTag" reference,
// omitting the repository prefix and/or tag when unset. No digest is appended —
// registry selection resolves and pins the digest. Mirrors the tag-only form of
// common.BuildImage without depending on internal packages.
func (r ImageRepository) Ref() string {
	image := r.ImageName
	if r.Repository != "" {
		image = r.Repository + "/" + image
	}
	if r.ImageTag != "" {
		image = image + ":" + r.ImageTag
	}
	return image
}

// platformLayerHashes keys each verification entry by "OS/Architecture"
// (e.g. "linux/amd64") and casts the string hashes to Sha256Digest. Entries
// missing OS or Architecture are skipped — a usable platform key is required.
func platformLayerHashes(specs []ImageVerificationSpec) map[string][]Sha256Digest {
	if len(specs) == 0 {
		return nil
	}
	out := make(map[string][]Sha256Digest, len(specs))
	for _, spec := range specs {
		if spec.OS == "" || spec.Architecture == "" {
			continue
		}
		platform := spec.OS + "/" + spec.Architecture
		hashes := make([]Sha256Digest, len(spec.LayerHashes))
		for i, h := range spec.LayerHashes {
			hashes[i] = Sha256Digest(h)
		}
		out[platform] = hashes
	}
	return out
}
