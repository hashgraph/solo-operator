// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("NetworkUpgradeFreeze Controller", func() {
	ctx := context.Background()

	const ns = "nuf-integration"

	BeforeEach(func() {
		createNamespaceIgnoreExisting(ctx, ns)
	})

	Context("prepare succeeded, all registries reachable", func() {
		const (
			nupName = "freeze-prepare-happy"
			nufName = "freeze-happy"
			opID    = "nuf-op-happy"
		)

		prepareKey := types.NamespacedName{Name: nupName, Namespace: ns}
		freezeKey := types.NamespacedName{Name: nufName, Namespace: ns}

		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels: map[string]string{
					operatorv1alpha1.LabelOperationID: opID,
					operatorv1alpha1.LabelStep:        operatorv1alpha1.StepPrepare,
				},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:       ns,
				OperationID: opID,
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
						Version: "0.75.0",
						Registries: []operatorv1alpha1.UpgradeRegistrySpec{
							{Image: "gcr.io/a/cn:0.75.0"},
							{Image: "ghcr.io/b/cn:0.75.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())

			By("waiting for Prepare to succeed")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, prepareKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			prepare := &operatorv1alpha1.NetworkUpgradePrepare{}
			Expect(k8sClient.Get(ctx, prepareKey, prepare)).To(Succeed())

			nuf := &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nufName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepFreeze,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
					Orbit:           ns,
					OperationID:     opID,
					ScheduledTime:   metav1.Now(),
					Images:          prepare.Spec.Images,
					SelectedSources: *prepare.Status.SelectedSources,
				},
			}
			Expect(k8sClient.Create(ctx, nuf)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns},
			})).To(Succeed())
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{
				ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns},
			})).To(Succeed())
		})

		It("reconciles to Succeeded with StartTime, both conditions True", func() {
			By("reaching Succeeded phase")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			got := &operatorv1alpha1.NetworkUpgradeFreeze{}
			Expect(k8sClient.Get(ctx, freezeKey, got)).To(Succeed())

			By("setting StartTime")
			Expect(got.Status.StartTime).NotTo(BeNil())

			By("setting RegistryHealthCheck=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondRegistryHealthCheck)).To(BeTrue())

			By("setting ReadyForFreeze=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)).To(BeTrue())
		})

		It("stays Succeeded on re-reconcile (idempotent)", func() {
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Consistently(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout/3, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))
		})
	})

	Context("prepare succeeded, selected registry unreachable — failover to alternate", func() {
		const (
			nupName = "freeze-prepare-failover"
			nufName = "freeze-failover"
			opID    = "nuf-op-failover"
		)

		prepareKey := types.NamespacedName{Name: nupName, Namespace: ns}
		freezeKey := types.NamespacedName{Name: nufName, Namespace: ns}

		// Prepare selects from two valid registries; we then inject an unreachable
		// selected source into the Freeze spec to force a failover.
		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels: map[string]string{
					operatorv1alpha1.LabelOperationID: opID,
					operatorv1alpha1.LabelStep:        operatorv1alpha1.StepPrepare,
				},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:       ns,
				OperationID: opID,
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
						Version: "0.75.0",
						Registries: []operatorv1alpha1.UpgradeRegistrySpec{
							{Image: "gcr.io/unreachable/cn:0.75.0"},
							{Image: "ghcr.io/valid/cn:0.75.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())

			By("waiting for Prepare to succeed (selects valid registry)")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, prepareKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			prepare := &operatorv1alpha1.NetworkUpgradePrepare{}
			Expect(k8sClient.Get(ctx, prepareKey, prepare)).To(Succeed())

			// Override selectedSources to simulate the registry becoming unreachable
			// between prepare and freeze time. The Freeze spec still lists both
			// registries in images, so failover to the valid one is possible.
			nuf := &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nufName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepFreeze,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
					Orbit:         ns,
					OperationID:   opID,
					ScheduledTime: metav1.Now(),
					Images:        prepare.Spec.Images,
					SelectedSources: operatorv1alpha1.SelectedSources{
						ConsensusNode: &operatorv1alpha1.SelectedSource{
							Image:               "gcr.io/unreachable/cn:0.75.0",
							Digest:              "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
							LayerHashesVerified: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nuf)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns},
			})).To(Succeed())
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{
				ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns},
			})).To(Succeed())
		})

		It("reconciles to Succeeded, emits RegistryFailover, updates selectedSources", func() {
			By("reaching Succeeded phase")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			got := &operatorv1alpha1.NetworkUpgradeFreeze{}
			Expect(k8sClient.Get(ctx, freezeKey, got)).To(Succeed())

			By("setting ReadyForFreeze=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)).To(BeTrue())

			By("updating selectedSources to the failover registry")
			Expect(got.Status.SelectedSources).NotTo(BeNil())
			Expect(got.Status.SelectedSources.ConsensusNode).NotTo(BeNil())
			Expect(got.Status.SelectedSources.ConsensusNode.Image).To(Equal("ghcr.io/valid/cn:0.75.0"))

			By("emitting RegistryFailover warning event")
			Expect(hasEvent(ctx, ns, nufName, operatorv1alpha1.EventRegistryFailover, "consensusNode")).To(BeTrue())
		})
	})

	Context("prepare succeeded, all registries unreachable", func() {
		const (
			nupName = "freeze-prepare-allfail"
			nufName = "freeze-allfail"
			opID    = "nuf-op-allfail"
		)

		prepareKey := types.NamespacedName{Name: nupName, Namespace: ns}
		freezeKey := types.NamespacedName{Name: nufName, Namespace: ns}

		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels: map[string]string{
					operatorv1alpha1.LabelOperationID: opID,
					operatorv1alpha1.LabelStep:        operatorv1alpha1.StepPrepare,
				},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:       ns,
				OperationID: opID,
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
						Version: "0.75.0",
						Registries: []operatorv1alpha1.UpgradeRegistrySpec{
							{Image: "gcr.io/valid/cn:0.75.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())

			By("waiting for Prepare to succeed")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, prepareKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			nuf := &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nufName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepFreeze,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
					Orbit:         ns,
					OperationID:   opID,
					ScheduledTime: metav1.Now(),
					Images: operatorv1alpha1.UpgradeImagesSpec{
						ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
							Version: "0.75.0",
							Registries: []operatorv1alpha1.UpgradeRegistrySpec{
								{Image: "gcr.io/unreachable/cn:0.75.0"},
							},
						},
					},
					SelectedSources: operatorv1alpha1.SelectedSources{
						ConsensusNode: &operatorv1alpha1.SelectedSource{
							Image:               "gcr.io/unreachable/cn:0.75.0",
							Digest:              "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
							LayerHashesVerified: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nuf)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns},
			})).To(Succeed())
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{
				ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns},
			})).To(Succeed())
		})

		It("stays Running, sets both conditions False", func() {
			By("reaching Running phase")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseRunning))

			got := &operatorv1alpha1.NetworkUpgradeFreeze{}
			Expect(k8sClient.Get(ctx, freezeKey, got)).To(Succeed())

			By("setting RegistryHealthCheck=False")
			Expect(conditionFalse(got.Status.Conditions, operatorv1alpha1.CondRegistryHealthCheck)).To(BeTrue())

			By("setting ReadyForFreeze=False")
			Expect(conditionFalse(got.Status.Conditions, operatorv1alpha1.CondReadyForFreeze)).To(BeTrue())
		})
	})

	Context("prepare not yet succeeded", func() {
		const (
			nupName = "freeze-prepare-pending"
			nufName = "freeze-pending"
			opID    = "nuf-op-pending"
		)

		freezeKey := types.NamespacedName{Name: nufName, Namespace: ns}

		BeforeEach(func() {
			// Create Prepare but do NOT wait for it — Freeze is created while Prepare is still Running.
			nup := &operatorv1alpha1.NetworkUpgradePrepare{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nupName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepPrepare,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
					Orbit:       ns,
					OperationID: opID,
					Images: operatorv1alpha1.UpgradeImagesSpec{
						ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
							Version:    "0.75.0",
							Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nup)).To(Succeed())

			nuf := &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nufName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepFreeze,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeSpec{
					Orbit:         ns,
					OperationID:   opID,
					ScheduledTime: metav1.Now(),
					Images: operatorv1alpha1.UpgradeImagesSpec{
						ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
							Version:    "0.75.0",
							Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
						},
					},
					SelectedSources: operatorv1alpha1.SelectedSources{
						ConsensusNode: &operatorv1alpha1.SelectedSource{
							Image:  "gcr.io/a/cn:0.75.0",
							Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nuf)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{
				ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns},
			})).To(Succeed())
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{
				ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns},
			})).To(Succeed())
		})

		It("eventually reaches Succeeded once Prepare completes", func() {
			By("staying non-terminal while Prepare is still running")
			Consistently(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, "3s", interval).ShouldNot(Or(
				Equal(operatorv1alpha1.PhaseSucceeded),
				Equal(operatorv1alpha1.PhaseFailed),
			))

			By("reaching Succeeded once Prepare succeeds")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreeze{}
				_ = k8sClient.Get(ctx, freezeKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))
		})
	})
})
