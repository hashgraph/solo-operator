// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// selectSlowDelay holds a "slow" image's CR non-terminal without racing an instant checker.
const selectSlowDelay = 4 * time.Second

// selectiveImageChecker is the suite-level ImageChecker: "unreachable" images report
// StateUnreachable, "slow" images report StateValid after selectSlowDelay, others StateValid immediately.
type selectiveImageChecker struct{}

func (selectiveImageChecker) Check(ctx context.Context, spec operatorv1alpha1.ImageRegistrySpec, _ operatorv1alpha1.ImageDeterministicSpec) registry.RegistryResult {
	if strings.Contains(spec.Image, "unreachable") {
		return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable}
	}
	if strings.Contains(spec.Image, "slow") {
		select {
		case <-time.After(selectSlowDelay):
		case <-ctx.Done():
			return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable, Err: ctx.Err()}
		}
	}
	return registry.RegistryResult{
		Image:               spec.Image,
		State:               registry.StateValid,
		Digest:              "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LayerHashesVerified: true,
	}
}

// stubImageChecker kept as alias so the suite_test.go reference compiles.
type stubImageChecker = selectiveImageChecker

var _ = Describe("NetworkUpgradePrepare Controller", func() {
	ctx := context.Background()

	const ns = "nup-integration"

	BeforeEach(func() {
		createNamespaceIgnoreExisting(ctx, ns)

		orbit := &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
			Spec: operatorv1alpha1.OrbitSpec{
				Consensus: &operatorv1alpha1.OrbitConsensus{
					Genesis: operatorv1alpha1.OrbitGenesis{
						AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x01"},
					},
				},
			},
		}
		if err := k8sClient.Create(ctx, orbit); err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}
	})

	Context("all registries reachable", func() {
		const (
			nupName = "prepare-happy"
			opID    = "nup-op-happy"
		)

		key := types.NamespacedName{Name: nupName, Namespace: ns}

		enabled := true
		disabled := false
		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:            ns,
				OperationID:      opID,
				ConsensusCapsule: "node0",
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.ImageComponentSpec{
						Version: "0.75.0",
						Enabled: &enabled,
						Registries: []operatorv1alpha1.ImageRegistrySpec{
							{Image: "gcr.io/a/cn:0.75.0"},
							{Image: "ghcr.io/b/cn:0.75.0"},
						},
					},
					RecordStreamUploader: &operatorv1alpha1.ImageComponentSpec{
						Version: "0.43.0",
						Enabled: &disabled,
						Registries: []operatorv1alpha1.ImageRegistrySpec{
							{Image: "gcr.io/a/rsu:0.43.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			deleteAndPurge(ctx, nup.DeepCopy())
		})

		It("reconciles to Succeeded with all conditions, events, and selectedSources", func() {
			By("reaching Succeeded phase")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			got := &operatorv1alpha1.NetworkUpgradePrepare{}
			Expect(k8sClient.Get(ctx, key, got)).To(Succeed())

			By("setting StartTime")
			Expect(got.Status.StartTime).NotTo(BeNil())

			By("populating selectedSources for consensusNode only (disabled RSU excluded)")
			Expect(got.Status.SelectedSources).NotTo(BeNil())
			Expect(got.Status.SelectedSources.ConsensusNode).NotTo(BeNil())
			Expect(got.Status.SelectedSources.ConsensusNode.Digest).NotTo(BeEmpty())
			Expect(got.Status.SelectedSources.RecordStreamUploader).To(BeNil())

			By("setting ManifestValidated=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondManifestValidated)).To(BeTrue())

			By("setting ImageValidation=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondImageValidation)).To(BeTrue())

			By("setting RegistrySelected=True")
			Expect(conditionTrue(got.Status.Conditions, operatorv1alpha1.CondRegistrySelected)).To(BeTrue())

			By("emitting RegistryAuditComplete for consensusNode (2/2 reachable)")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryAuditComplete.String(),
				"consensusNode: 2/2 registries reachable")).To(BeTrue())

			By("not emitting any event for the disabled recordStreamUploader")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryAuditComplete.String(),
				"recordStreamUploader")).To(BeFalse())

			By("emitting RegistrySelected for consensusNode")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistrySelected.String(), "consensusNode")).To(BeTrue())
		})

		It("stays Succeeded on re-reconcile (idempotent)", func() {
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Consistently(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout/3, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))
		})
	})

	Context("partial failure — one registry unreachable, one valid", func() {
		const (
			nupName = "prepare-partial"
			opID    = "nup-op-partial"
		)

		key := types.NamespacedName{Name: nupName, Namespace: ns}

		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:            ns,
				OperationID:      opID,
				ConsensusCapsule: "node0",
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.ImageComponentSpec{
						Version: "0.75.0",
						Registries: []operatorv1alpha1.ImageRegistrySpec{
							{Image: "gcr.io/unreachable/cn:0.75.0"},
							{Image: "ghcr.io/valid/cn:0.75.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			deleteAndPurge(ctx, nup.DeepCopy())
		})

		It("succeeds, emits RegistryUnreachable, and selects the valid registry", func() {
			By("reaching Succeeded (valid fallback registry selected)")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("emitting RegistryUnreachable warning (1/2 unreachable)")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryUnreachable.String(),
				"consensusNode: 1/2 registries unreachable")).To(BeTrue())

			By("emitting RegistryAuditComplete showing 1/2 reachable")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryAuditComplete.String(),
				"consensusNode: 1/2 registries reachable")).To(BeTrue())

			By("selecting the valid registry in selectedSources")
			got := &operatorv1alpha1.NetworkUpgradePrepare{}
			Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
			Expect(got.Status.SelectedSources).NotTo(BeNil())
			Expect(got.Status.SelectedSources.ConsensusNode.Image).To(Equal("ghcr.io/valid/cn:0.75.0"))
		})
	})

	Context("all registries unreachable", func() {
		const (
			nupName = "prepare-allfail"
			opID    = "nup-op-allfail"
		)

		key := types.NamespacedName{Name: nupName, Namespace: ns}

		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nupName,
				Namespace: ns,
				Labels:    map[string]string{operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepPrepare},
			},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:            ns,
				OperationID:      opID,
				ConsensusCapsule: "node0",
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.ImageComponentSpec{
						Version: "0.75.0",
						Registries: []operatorv1alpha1.ImageRegistrySpec{
							{Image: "gcr.io/unreachable/cn:0.75.0"},
							{Image: "ghcr.io/unreachable/cn:0.75.0"},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, nup.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			deleteAndPurge(ctx, nup.DeepCopy())
		})

		It("stays Running, sets RegistrySelected=False, emits RegistryInvalid, and skips selectedSources", func() {
			By("reaching Running phase (registry failures are retryable)")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkUpgradePrepare{}
				_ = k8sClient.Get(ctx, key, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseRunning))

			got := &operatorv1alpha1.NetworkUpgradePrepare{}
			Expect(k8sClient.Get(ctx, key, got)).To(Succeed())

			By("setting RegistrySelected=False condition")
			Expect(conditionFalse(got.Status.Conditions, operatorv1alpha1.CondRegistrySelected)).To(BeTrue())

			By("emitting RegistryInvalid warning")
			Expect(hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryInvalid.String(), "consensusNode")).To(BeTrue())

			By("not populating selectedSources")
			Expect(got.Status.SelectedSources).To(BeNil())
		})
	})
})

func conditionTrue(conds []operatorv1alpha1.Condition, condType string) bool {
	for _, c := range conds {
		if c.Type == condType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

func conditionFalse(conds []operatorv1alpha1.Condition, condType string) bool {
	for _, c := range conds {
		if c.Type == condType {
			return c.Status == metav1.ConditionFalse
		}
	}
	return false
}

func hasEvent(ctx context.Context, ns, objName, reason, msgSubstr string) bool {
	var events corev1.EventList
	if err := k8sClient.List(ctx, &events, client.InNamespace(ns)); err != nil {
		return false
	}
	for _, e := range events.Items {
		if e.InvolvedObject.Name == objName && e.Reason == reason && strings.Contains(e.Message, msgSubstr) {
			return true
		}
	}
	return false
}
