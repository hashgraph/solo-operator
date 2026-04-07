// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("HelmCapsule Controller", func() {

	// ─── Basic install / lifecycle ───────────────────────────────────────────

	Context("When creating a generic HelmCapsule", func() {
		const (
			hcName = "test-mirror"
			hcNS   = "helmcapsule-basic"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should add a finalizer and reach Running phase", func() {
			By("Creating a HelmCapsule")
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}

			By("Checking finalizer is registered")
			Eventually(func() bool {
				got := &operatorv1alpha1.HelmCapsule{}
				if err := k8sClient.Get(ctx, key, got); err != nil {
					return false
				}
				for _, f := range got.Finalizers {
					if f == "operator.solo.hedera.com/helmcapsule-finalizer" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Checking status phase becomes Running")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				if err := k8sClient.Get(ctx, key, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Checking release version is recorded in status")
			Eventually(func() int {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.ReleaseVersion
			}, timeout, interval).Should(BeNumerically(">", 0))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())

			By("Verifying HelmCapsule is eventually deleted (finalizer removed)")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, &operatorv1alpha1.HelmCapsule{})
				return err != nil // NotFound == deleted
			}, timeout, interval).Should(BeTrue())
		})
	})

	// ─── ValuesFrom — ConfigMap ───────────────────────────────────────────────

	Context("When a HelmCapsule references a ConfigMap via ValuesFrom", func() {
		const (
			hcName = "test-from-configmap"
			hcNS   = "helmcapsule-valuesref"
			cmName = "custom-values"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: hcNS},
				Data:       map[string]string{"values.yaml": `{"replicaCount": 2}`},
			}
			err = k8sClient.Create(ctx, cm)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully with ValuesFrom ConfigMap", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
					ValuesFrom: []operatorv1alpha1.HelmValuesReference{
						{Kind: "ConfigMap", Name: cmName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Mirror component — auto-injection ───────────────────────────────────

	Context("When creating a mirror HelmCapsule with an Orbit and ConsensusCapsules", func() {
		const (
			orbitName = "mirror-test-orbit"
			hcNS      = "helmcapsule-mirror"
			hcName    = "mirror"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "03"},
						ApplicationConfiguration: map[string]string{},
						PermissionConfiguration:  map[string]string{},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			for i := 1; i <= 2; i++ {
				cap := &operatorv1alpha1.ConsensusCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("node%d", i),
						Namespace: hcNS,
					},
					Spec: operatorv1alpha1.ConsensusCapsuleSpec{
						Orbit:     orbitName,
						NodeId:    int64(i),
						AccountId: fmt.Sprintf("0.0.%d", i+2),
						Weight:    1,
						PodProperties: operatorv1alpha1.ConsensusPodProperties{
							Containers: operatorv1alpha1.ConsensusContainers{
								ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
									SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
										Repository: "docker.io/hashgraph",
										ImageName:  "hedera-services",
										ImageTag:   "0.58.0",
									},
								},
							},
						},
					},
				}
				err = k8sClient.Create(ctx, cap)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
			}
		})

		It("Should reconcile successfully with auto-injected mirror values", func() {
			comp := operatorv1alpha1.HelmComponentMirror
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
					Component: &comp,
					OrbitRef:  &corev1.LocalObjectReference{Name: orbitName},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── ValuesFrom — Secret ─────────────────────────────────────────────────

	Context("When a HelmCapsule references a Secret via ValuesFrom", func() {
		const (
			hcName     = "test-from-secret"
			hcNS       = "helmcapsule-secret-values"
			secretName = "chart-values-secret"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: hcNS},
				StringData: map[string]string{"values.yaml": `{"replicaCount": 1}`},
			}
			err = k8sClient.Create(ctx, secret)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully with ValuesFrom Secret", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
					ValuesFrom: []operatorv1alpha1.HelmValuesReference{
						{Kind: "Secret", Name: secretName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── ValuesFrom — optional missing ConfigMap ──────────────────────────────

	Context("When a HelmCapsule references an optional missing ConfigMap via ValuesFrom", func() {
		const (
			hcName = "test-optional-missing"
			hcNS   = "helmcapsule-optional"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully when the optional ConfigMap does not exist", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
					ValuesFrom: []operatorv1alpha1.HelmValuesReference{
						{Kind: "ConfigMap", Name: "does-not-exist", Optional: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Chart upgrade on version bump ───────────────────────────────────────

	Context("When a HelmCapsule chart version is updated", func() {
		const (
			hcName = "test-upgrade"
			hcNS   = "helmcapsule-upgrade"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should trigger an upgrade and increment ReleaseVersion", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}

			By("Waiting for initial install to succeed")
			Eventually(func() int {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.ReleaseVersion
			}, timeout, interval).Should(Equal(1))

			By("Bumping the chart version")
			got := &operatorv1alpha1.HelmCapsule{}
			Expect(k8sClient.Get(ctx, key, got)).Should(Succeed())
			got.Spec.Chart.Version = "0.130.0"
			Expect(k8sClient.Update(ctx, got)).Should(Succeed())

			By("Waiting for upgrade to be reflected in ReleaseVersion")
			Eventually(func() int {
				updated := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, updated)
				return updated.Status.ReleaseVersion
			}, timeout, interval).Should(Equal(2))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Relay component — auto-injection ────────────────────────────────────

	Context("When creating a relay HelmCapsule with an Orbit", func() {
		const (
			orbitName = "relay-test-orbit"
			hcNS      = "helmcapsule-relay"
			hcName    = "relay"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "04"},
						ApplicationConfiguration: map[string]string{},
						PermissionConfiguration:  map[string]string{},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			cap := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "node1", Namespace: hcNS},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:     orbitName,
					NodeId:    1,
					AccountId: "0.0.3",
					Weight:    1,
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "docker.io/hashgraph",
									ImageName:  "hedera-services",
									ImageTag:   "0.58.0",
								},
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, cap)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully with auto-injected relay values", func() {
			comp := operatorv1alpha1.HelmComponentRelay
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-json-rpc-relay",
						Version: "0.65.0",
					},
					Component: &comp,
					OrbitRef:  &corev1.LocalObjectReference{Name: orbitName},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Explorer component — auto-injection ─────────────────────────────────

	Context("When creating an explorer HelmCapsule with an Orbit", func() {
		const (
			orbitName = "explorer-test-orbit"
			hcNS      = "helmcapsule-explorer"
			hcName    = "explorer"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "05"},
						ApplicationConfiguration: map[string]string{},
						PermissionConfiguration:  map[string]string{},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully with auto-injected explorer values", func() {
			comp := operatorv1alpha1.HelmComponentExplorer
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hiero-explorer-chart",
						Version: "1.0.0",
					},
					Component: &comp,
					OrbitRef:  &corev1.LocalObjectReference{Name: orbitName},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Block-node component — auto-injection ────────────────────────────────

	Context("When creating a block-node HelmCapsule with an Orbit and ConsensusCapsules", func() {
		const (
			orbitName = "block-node-test-orbit"
			hcNS      = "helmcapsule-block-node"
			hcName    = "block-node"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "06"},
						ApplicationConfiguration: map[string]string{},
						PermissionConfiguration:  map[string]string{},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			cap := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "node1", Namespace: hcNS},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:     orbitName,
					NodeId:    1,
					AccountId: "0.0.3",
					Weight:    1,
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "docker.io/hashgraph",
									ImageName:  "hedera-services",
									ImageTag:   "0.58.0",
								},
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, cap)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should reconcile successfully with auto-injected block-node values", func() {
			comp := operatorv1alpha1.HelmComponentBlockNode
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "block-node-server",
						Version: "0.1.0",
					},
					Component: &comp,
					OrbitRef:  &corev1.LocalObjectReference{Name: orbitName},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})

	// ─── Observed generation ─────────────────────────────────────────────────

	Context("When a HelmCapsule is updated", func() {
		const (
			hcName = "test-observed-gen"
			hcNS   = "helmcapsule-gen"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: hcNS}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update ObservedGeneration in status", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: hcNS},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, hc)).Should(Succeed())

			key := types.NamespacedName{Name: hcName, Namespace: hcNS}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Eventually(func() int64 {
				got := &operatorv1alpha1.HelmCapsule{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.ObservedGeneration
			}, timeout, interval).Should(BeNumerically(">", 0))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, hc)).Should(Succeed())
		})
	})
})
