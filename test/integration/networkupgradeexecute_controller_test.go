// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	consensusresources "github.com/hashgraph/solo-operator/internal/resources/consensus"
)

var _ = Describe("NetworkUpgradeExecute Controller", func() {
	ctx := context.Background()

	const ns = "nue-integration"

	BeforeEach(func() {
		createNamespaceIgnoreExisting(ctx, ns)

		log4j2 := &operatorv1alpha1.Log4j2Config{
			ObjectMeta: metav1.ObjectMeta{Name: "network-log4j2", Namespace: ns},
			Spec: operatorv1alpha1.Log4j2ConfigSpec{
				Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
			},
		}
		if err := k8sClient.Create(ctx, log4j2); err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}

		nodeSettings := &operatorv1alpha1.NodeSettings{
			ObjectMeta: metav1.ObjectMeta{Name: "network-settings", Namespace: ns},
			Spec:       operatorv1alpha1.NodeSettingsSpec{Content: "checkSignedStateFromDisk,              1"},
		}
		if err := k8sClient.Create(ctx, nodeSettings); err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}

		appProps := &operatorv1alpha1.ApplicationProperties{
			ObjectMeta: metav1.ObjectMeta{Name: "network-appprops", Namespace: ns},
			Spec: operatorv1alpha1.ApplicationPropertiesSpec{
				Content: "hedera.config.version=0\nnetty.mode=TEST",
			},
		}
		if err := k8sClient.Create(ctx, appProps); err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}
	})

	buildPrereqs := func(opID, nupName, nufName string, images operatorv1alpha1.UpgradeImagesSpec) operatorv1alpha1.SelectedSources {
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
				Images:      images,
			},
		}
		Expect(k8sClient.Create(ctx, nup)).To(Succeed())

		prepareKey := types.NamespacedName{Name: nupName, Namespace: ns}
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

		freezeKey := types.NamespacedName{Name: nufName, Namespace: ns}
		By("waiting for Freeze to succeed")
		Eventually(func() operatorv1alpha1.Phase {
			got := &operatorv1alpha1.NetworkUpgradeFreeze{}
			_ = k8sClient.Get(ctx, freezeKey, got)
			return got.Status.Phase
		}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

		return *prepare.Status.SelectedSources
	}

	buildCapsuleWithSTS := func(name, orbitLabel string, readyReplicas int32) *operatorv1alpha1.ConsensusCapsule {
		capsule := &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels:    map[string]string{common.LabelOrbit: orbitLabel},
			},
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				Orbit:                    orbitLabel,
				NodeId:                   0,
				AccountId:                "0.0.3",
				Weight:                   1,
				Log4j2ConfigRef:          "network-log4j2",
				SettingsConfigRef:        "network-settings",
				ApplicationPropertiesRef: "network-appprops",
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
							SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
								Repository: "gcr.io/old",
								ImageName:  "cn",
								ImageTag:   "0.74.0",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, capsule)).To(Succeed())

		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consensusresources.StatefulSetName(capsule),
				Namespace: ns,
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, sts)).To(Succeed())

		if readyReplicas > 0 {
			sts.Status.Replicas = readyReplicas
			sts.Status.ReadyReplicas = readyReplicas
			Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
		}

		return capsule
	}

	Context("prereqs succeeded, all registries reachable, daemon reports success", func() {
		const (
			nupName = "execute-prepare-happy"
			nufName = "execute-freeze-happy"
			nuxName = "execute-happy"
			opID    = "nue-op-happy"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}

		var selected operatorv1alpha1.SelectedSources

		BeforeEach(func() {
			images := operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
				},
			}
			selected = buildPrereqs(opID, nupName, nufName, images)

			buildCapsuleWithSTS("node0-happy", ns, 1)

			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:           ns,
					OperationID:     opID,
					ConsensusNode:   "node0-happy",
					SelectedSources: selected,
					Images: operatorv1alpha1.UpgradeImagesSpec{
						ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
							Version:    "0.75.0",
							Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: "node0-happy", Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "node0-happy", Namespace: ns}})
		})

		It("initializes to ReadyForProvisionerDaemon with StartTime", func() {
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			Expect(got.Status.StartTime).NotTo(BeNil())
		})

		It("advances to PendingNodeUpgrade after daemon writes DaemonResult=True", func() {
			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("simulating daemon writing DaemonResult=True and CondConfigCRsApplied=True")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions,
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondDaemonResult,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondConfigCRsApplied,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
			)
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			By("expecting PendingNodeUpgrade or beyond")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(BeElementOf(
				operatorv1alpha1.ExecutePhasePendingNodeUpgrade,
				operatorv1alpha1.ExecutePhaseSucceeded,
			))
		})

		It("reaches Succeeded after daemon success and pods become ready", func() {
			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("writing DaemonResult=True and CondConfigCRsApplied=True")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions,
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondDaemonResult,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondConfigCRsApplied,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
			)
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			By("expecting Succeeded phase")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseSucceeded))

			final := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, final)).To(Succeed())

			By("setting CompletionTime")
			Expect(final.Status.CompletionTime).NotTo(BeNil())

			By("setting NodesUpgraded=True")
			Expect(conditionTrue(final.Status.Conditions, operatorv1alpha1.CondNodesUpgraded)).To(BeTrue())

			By("setting ConsensusCapsulePatched=True")
			Expect(conditionTrue(final.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)).To(BeTrue())

			By("emitting UpgradeSucceeded event")
			Expect(hasEvent(ctx, ns, nuxName, operatorv1alpha1.EventUpgradeSucceeded, opID)).To(BeTrue())
		})
	})

	Context("daemon reports failure", func() {
		const (
			nupName = "execute-prepare-daemon-fail"
			nufName = "execute-freeze-daemon-fail"
			nuxName = "execute-daemon-fail"
			opID    = "nue-op-daemon-fail"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}

		BeforeEach(func() {
			images := operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
				},
			}
			selected := buildPrereqs(opID, nupName, nufName, images)

			buildCapsuleWithSTS("node0-daemon-fail", ns, 0)

			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:           ns,
					OperationID:     opID,
					ConsensusNode:   "node0-daemon-fail",
					SelectedSources: selected,
					Images: operatorv1alpha1.UpgradeImagesSpec{
						ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
							Version:    "0.75.0",
							Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: "node0-daemon-fail", Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "node0-daemon-fail", Namespace: ns}})
		})

		It("transitions to Failed when daemon writes DaemonResult=False", func() {
			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("writing DaemonResult=False")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions, operatorv1alpha1.Condition{
				Type:               operatorv1alpha1.CondDaemonResult,
				Status:             metav1.ConditionFalse,
				Reason:             operatorv1alpha1.ReasonDaemonFileHashMismatch,
				LastTransitionTime: metav1.Now(),
			})
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseFailed))

			final := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, final)).To(Succeed())
			Expect(final.Status.CompletionTime).NotTo(BeNil())
		})
	})

	Context("prereqs not yet succeeded", func() {
		const (
			nuxName = "execute-no-prereqs"
			opID    = "nue-op-no-prereqs"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}

		BeforeEach(func() {
			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:         ns,
					OperationID:   opID,
					ConsensusNode: "node0-no-prereqs",
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
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
		})

		It("stays in Pending while waiting for prereqs", func() {
			Consistently(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout/4, interval).ShouldNot(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))
		})
	})

	Context("registry revalidation — selected registry unreachable, failover available", func() {
		const (
			nupName = "execute-prepare-failover"
			nufName = "execute-freeze-failover"
			nuxName = "execute-failover"
			opID    = "nue-op-failover"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}

		BeforeEach(func() {
			images := operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version: "0.75.0",
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{
						{Image: "gcr.io/unreachable/cn:0.75.0"},
						{Image: "gcr.io/valid/cn:0.75.0"},
					},
				},
			}
			buildPrereqs(opID, nupName, nufName, images)
			buildCapsuleWithSTS("node0-failover", ns, 0)

			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:         ns,
					OperationID:   opID,
					ConsensusNode: "node0-failover",
					Images:        images,
					SelectedSources: operatorv1alpha1.SelectedSources{
						ConsensusNode: &operatorv1alpha1.SelectedSource{
							Image:  "gcr.io/unreachable/cn:0.75.0",
							Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: "node0-failover", Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "node0-failover", Namespace: ns}})
		})

		It("revalidates, fails over, sets RegistrySelected=True, emits RegistryFailover event", func() {
			By("waiting for DaemonResult=True then RegistrySelected=True after failover")

			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("writing DaemonResult=True and CondConfigCRsApplied=True")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions,
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondDaemonResult,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondConfigCRsApplied,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
			)
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			By("waiting for RegistrySelected=True after failover")
			Eventually(func() bool {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return conditionTrue(got.Status.Conditions, operatorv1alpha1.CondRegistrySelected)
			}, timeout, interval).Should(BeTrue())

			final := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, final)).To(Succeed())

			By("updating selectedSources to the failover registry")
			Expect(final.Status.SelectedSources).NotTo(BeNil())
			Expect(final.Status.SelectedSources.ConsensusNode).NotTo(BeNil())
			Expect(final.Status.SelectedSources.ConsensusNode.Image).To(Equal("gcr.io/valid/cn:0.75.0"))

			By("emitting RegistryFailover warning event")
			Expect(hasEvent(ctx, ns, nuxName, operatorv1alpha1.EventRegistryFailover, "consensusNode")).To(BeTrue())
		})
	})

	Context("capsule patch — images and Offline=false applied atomically", func() {
		const (
			nupName = "execute-prepare-patch"
			nufName = "execute-freeze-patch"
			nuxName = "execute-patch"
			opID    = "nue-op-patch"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}
		capsName := "node0-patch"

		BeforeEach(func() {
			images := operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
				},
			}
			selected := buildPrereqs(opID, nupName, nufName, images)

			buildCapsuleWithSTS(capsName, ns, 1)

			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:           ns,
					OperationID:     opID,
					ConsensusNode:   capsName,
					Images:          images,
					SelectedSources: selected,
				},
			}
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: capsName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: capsName, Namespace: ns}})
		})

		It("patches capsule image and sets Offline=false after daemon succeeds", func() {
			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("writing DaemonResult=True and CondConfigCRsApplied=True")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions,
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondDaemonResult,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondConfigCRsApplied,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
			)
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			By("waiting for ConsensusCapsulePatched condition")
			Eventually(func() bool {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return conditionTrue(got.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)
			}, timeout, interval).Should(BeTrue())

			capsule := &operatorv1alpha1.ConsensusCapsule{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: capsName, Namespace: ns}, capsule)).To(Succeed())

			By("setting Offline=false")
			Expect(capsule.Spec.Offline).To(BeFalse())

			By("updating image tag to 0.75.0")
			sv := capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion
			Expect(sv).NotTo(BeNil())
			Expect(sv.ImageTag).To(Equal("0.75.0"))

			By("emitting CapsulePatched event")
			Expect(hasEvent(ctx, ns, nuxName, operatorv1alpha1.EventCapsulePatched, opID)).To(BeTrue())
		})
	})

	Context("idempotency — stays Succeeded on re-reconcile", func() {
		const (
			nupName = "execute-prepare-idem"
			nufName = "execute-freeze-idem"
			nuxName = "execute-idem"
			opID    = "nue-op-idem"
		)

		nuxKey := types.NamespacedName{Name: nuxName, Namespace: ns}
		capsName := "node0-idem"

		BeforeEach(func() {
			images := operatorv1alpha1.UpgradeImagesSpec{
				ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
					Version:    "0.75.0",
					Registries: []operatorv1alpha1.UpgradeRegistrySpec{{Image: "gcr.io/a/cn:0.75.0"}},
				},
			}
			selected := buildPrereqs(opID, nupName, nufName, images)
			buildCapsuleWithSTS(capsName, ns, 1)

			nux := &operatorv1alpha1.NetworkUpgradeExecute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nuxName,
					Namespace: ns,
					Labels: map[string]string{
						operatorv1alpha1.LabelOperationID: opID,
						operatorv1alpha1.LabelStep:        operatorv1alpha1.StepExecute,
					},
				},
				Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
					Orbit:           ns,
					OperationID:     opID,
					ConsensusNode:   capsName,
					Images:          images,
					SelectedSources: selected,
				},
			}
			Expect(k8sClient.Create(ctx, nux)).To(Succeed())

			By("waiting for ReadyForProvisionerDaemon")
			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseReadyForProvisionerDaemon))

			By("writing DaemonResult=True and CondConfigCRsApplied=True to drive to Succeeded")
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			Expect(k8sClient.Get(ctx, nuxKey, got)).To(Succeed())
			got.Status.Conditions = append(got.Status.Conditions,
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondDaemonResult,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
				operatorv1alpha1.Condition{
					Type:               operatorv1alpha1.CondConfigCRsApplied,
					Status:             metav1.ConditionTrue,
					Reason:             operatorv1alpha1.ReasonDaemonSucceeded,
					LastTransitionTime: metav1.Now(),
				},
			)
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			Eventually(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExecutePhaseSucceeded))
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: nuxName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: nufName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: capsName, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: capsName, Namespace: ns}})
		})

		It("stays Succeeded on re-reconcile", func() {
			Consistently(func() operatorv1alpha1.ExecutePhase {
				got := &operatorv1alpha1.NetworkUpgradeExecute{}
				_ = k8sClient.Get(ctx, nuxKey, got)
				return got.Status.Phase
			}, timeout/3, interval).Should(Equal(operatorv1alpha1.ExecutePhaseSucceeded))
		})
	})
})
