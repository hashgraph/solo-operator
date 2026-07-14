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

var _ = Describe("NetworkUpgradeFreezeAbort Controller", func() {
	ctx := context.Background()

	const ns = "nufa-integration"

	BeforeEach(func() {
		createNamespaceIgnoreExisting(ctx, ns)
	})

	Context("no Execute CR exists for the same operationId", func() {
		const (
			nuaName = "abort-no-execute"
			opID    = "nufa-op-no-execute"
		)
		abortKey := types.NamespacedName{Name: nuaName, Namespace: ns}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NetworkUpgradeFreezeAbort{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuaName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepAbort,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeAbortSpec{
					Orbit:       ns,
					OperationID: opID,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			deleteAndPurge(ctx, &operatorv1alpha1.NetworkUpgradeFreezeAbort{
				ObjectMeta: metav1.ObjectMeta{Name: nuaName, Namespace: ns},
			})
		})

		It("reconciles to Succeeded with AbortProcessed=True", func() {
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
				_ = k8sClient.Get(ctx, abortKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
			Expect(k8sClient.Get(ctx, abortKey, got)).To(Succeed())

			By("setting StartTime")
			Expect(got.Status.StartTime).NotTo(BeNil())

			By("setting CompletionTime")
			Expect(got.Status.CompletionTime).NotTo(BeNil())

			By("setting AbortProcessed=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondAbortProcessed)).To(BeTrue())
		})

		It("stays Succeeded on re-reconcile (idempotent)", func() {
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
				_ = k8sClient.Get(ctx, abortKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Consistently(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
				_ = k8sClient.Get(ctx, abortKey, got)
				return got.Status.Phase
			}, timeout/3, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))
		})
	})

	Context("Execute CR already exists for the same operationId", func() {
		const (
			nuaName = "abort-after-freeze"
			nueName = "execute-before-abort"
			opID    = "nufa-op-after-freeze"
		)
		abortKey := types.NamespacedName{Name: nuaName, Namespace: ns}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nueName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:         ns,
					OperationID:   opID,
					ConsensusNode: "node0-abort",
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
			})).To(Succeed())

			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NetworkUpgradeFreezeAbort{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuaName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepAbort,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeFreezeAbortSpec{
					Orbit:       ns,
					OperationID: opID,
				},
			})).To(Succeed())
		})

		AfterEach(func() {
			deleteAndPurge(ctx, &operatorv1alpha1.NetworkUpgradeFreezeAbort{
				ObjectMeta: metav1.ObjectMeta{Name: nuaName, Namespace: ns},
			})
			deleteAndPurge(ctx, &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{Name: nueName, Namespace: ns},
			})
		})

		It("reconciles to Failed with AbortProcessed=False", func() {
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
				_ = k8sClient.Get(ctx, abortKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseFailed))

			got := &operatorv1alpha1.NetworkUpgradeFreezeAbort{}
			Expect(k8sClient.Get(ctx, abortKey, got)).To(Succeed())

			By("setting CompletionTime")
			Expect(got.Status.CompletionTime).NotTo(BeNil())

			By("setting AbortProcessed=False")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondAbortProcessed)).To(BeFalse())
		})
	})
})
