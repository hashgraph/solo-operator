// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("ConsensusCapsule Image Source Validation", func() {
	ctx := context.Background()
	const testNS = "capsule-img-val-ns"

	BeforeEach(func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
		if err := k8sClient.Create(ctx, ns); err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}
	})

	sv := func(tag string) *operatorv1alpha1.SoftwareVersion {
		return &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io/hedera-registry", ImageName: "consensus-node", ImageTag: tag}
	}
	source := func(tag string) *operatorv1alpha1.SoftwareVersionSource {
		return &operatorv1alpha1.SoftwareVersionSource{
			ImageRepositories: []operatorv1alpha1.ImageRepository{
				{Repository: "gcr.io/hedera-registry", ImageName: "consensus-node", ImageTag: tag},
				{Repository: "docker.io/hashgraph", ImageName: "consensus-node", ImageTag: tag},
			},
			ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{
				{OS: "linux", Architecture: "amd64", LayerHashes: []string{"sha256:aaa"}},
			},
		}
	}

	// capsule assembles an otherwise-valid ConsensusCapsule with the given consensus-node
	// image fields and a valid UC sidecar (softwareVersion only).
	capsule := func(name string, node operatorv1alpha1.ConsensusNodeContainer) *operatorv1alpha1.ConsensusCapsule {
		node.Name = "consensus-node"
		return &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				Orbit:                    testNS,
				NodeId:                   0,
				AccountId:                "0.0.3",
				Weight:                   1,
				Log4j2ConfigRef:          "node0-log4j2",
				SettingsConfigRef:        "node0-settings",
				ApplicationPropertiesRef: "node0-application-properties",
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						ConsensusNode: node,
						UC: &operatorv1alpha1.UcSidecar{
							Enabled:         true,
							SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "ghcr.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "1.0.0"},
						},
					},
				},
			},
		}
	}

	Context("Consensus node — exactly one image source", func() {
		It("Rejects both softwareVersion and softwareVersionSource", func() {
			c := capsule("node-both", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("0.65.1"), SoftwareVersionSource: source("0.65.1")})
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("set exactly one of softwareVersion or softwareVersionSource"))
		})

		It("Rejects neither softwareVersion nor softwareVersionSource", func() {
			c := capsule("node-neither", operatorv1alpha1.ConsensusNodeContainer{})
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("set exactly one of softwareVersion or softwareVersionSource"))
		})

		It("Accepts softwareVersion only", func() {
			Expect(k8sClient.Create(ctx, capsule("node-sv-only", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("0.65.1")}))).Should(Succeed())
		})

		It("Accepts softwareVersionSource only", func() {
			Expect(k8sClient.Create(ctx, capsule("node-src-only", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersionSource: source("0.65.1")}))).Should(Succeed())
		})

		It("Rejects a softwareVersion with an empty imageTag and no digest", func() {
			// An empty tag with no digest resolves to :latest, so the image must be pinned
			// by a non-empty tag or a digest.
			c := capsule("node-sv-empty-tag", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("")})
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("non-empty imageTag or a digest"))
		})

		It("Accepts a softwareVersion with an empty imageTag when a digest is set", func() {
			node := operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("")}
			node.SoftwareVersion.Digest = operatorv1alpha1.Sha256Digest("sha256:" + "a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f708192a3b4c5d6e7f809")
			Expect(k8sClient.Create(ctx, capsule("node-sv-digest-only", node))).Should(Succeed())
		})

		It("Rejects a source candidate with an empty imageTag", func() {
			// An empty tag would resolve to :latest and differ per registry, defeating the
			// "same version everywhere" guarantee, so the CRD requires a non-empty tag.
			c := capsule("node-src-empty-tag", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersionSource: source("")})
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("imageTag"))
		})

		It("Rejects a softwareVersion pinned to the mutable latest tag", func() {
			// The single-registry field is a cheap CEL rule. A source candidate's "latest"
			// is rejected in the reconciler instead (unbounded list, CEL cost budget) — see
			// TestLatestTagInSource.
			c := capsule("node-sv-latest", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("latest")})
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must not be 'latest'"))
		})
	})

	Context("Sidecar — at most one image source", func() {
		It("Rejects a UC sidecar that sets both", func() {
			c := capsule("uc-both", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("0.65.1")})
			c.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
				Enabled:               true,
				SoftwareVersion:       &operatorv1alpha1.SoftwareVersion{Repository: "ghcr.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "1.0.0"},
				SoftwareVersionSource: source("1.0.0"),
			}
			err := k8sClient.Create(ctx, c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("set at most one of softwareVersion or softwareVersionSource"))
		})

		It("Accepts a disabled uploader that sets neither", func() {
			c := capsule("uploader-neither", operatorv1alpha1.ConsensusNodeContainer{SoftwareVersion: sv("0.65.1")})
			c.Spec.PodProperties.Containers.RecordsUploader = &operatorv1alpha1.CheetahUploader{Enabled: false, MaxProcessors: 1}
			Expect(k8sClient.Create(ctx, c)).Should(Succeed())
		})
	})
})
