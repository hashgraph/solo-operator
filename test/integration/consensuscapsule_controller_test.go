// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

var _ = Describe("ConsensusCapsule Controller", func() {
	Context("When creating a ConsensusCapsule", func() {
		const (
			capsuleName      = "test-node"
			capsuleNamespace = "consensus-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.Log4j2ConfigSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
				},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.NodeSettingsSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: "checkSignedStateFromDisk,              1",
				},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: "hedera.config.version=0\nnetty.mode=TEST",
				},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create all dependent resources", func() {
			By("Creating a new ConsensusCapsule")
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      capsuleName,
					Namespace: capsuleNamespace,
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   0,
					AccountId:                "0.0.3",
					Weight:                   1,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{Enabled: true, SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "ghcr.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "latest"}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			// Verify ConsensusCapsule is created
			capsuleLookupKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			createdCapsule := &operatorv1alpha1.ConsensusCapsule{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, capsuleLookupKey, createdCapsule)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the ServiceAccount is created")
			saLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-sa", capsuleName), Namespace: capsuleNamespace}
			createdSA := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saLookupKey, createdSA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the RoleBinding is created")
			rbLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-rb", capsuleName), Namespace: capsuleNamespace}
			createdRB := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rbLookupKey, createdRB)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Service is created")
			svcLookupKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the StatefulSet is created")
			stsLookupKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			createdSts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, stsLookupKey, createdSts)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSts.Spec.Template.Spec.Containers[0].Image).To(Equal("gcr.io/hedera-registry/consensus-node:0.65.1"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})

	Context("When creating a ConsensusCapsule with PVCs", func() {
		const (
			capsuleName      = "node-with-pvcs"
			capsuleNamespace = "consensus-pvc-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.Log4j2ConfigSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
				},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.NodeSettingsSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: "checkSignedStateFromDisk,              1",
				},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
					Orbit:   capsuleNamespace,
					Scope:   "node0",
					Content: "hedera.config.version=0\nnetty.mode=TEST",
				},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create PVCs", func() {
			By("Creating a ConsensusCapsule with PVC config")
			storageClass := "standard"
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      capsuleName,
					Namespace: capsuleNamespace,
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   1,
					AccountId:                "0.0.4",
					Weight:                   1,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{Enabled: true, SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "ghcr.io/hashgraph", ImageName: "solo-operator-uc", ImageTag: "latest"}},
						},
					},
					PersistentVolumeClaims: &operatorv1alpha1.ConsensusPVCConfig{
						Logs: &corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: &storageClass,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("10Gi"),
								},
							},
						},
						Stats: &corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: &storageClass,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			By("Verifying the logs PVC is created")
			logsPVCKey := types.NamespacedName{Name: fmt.Sprintf("%s-logs", capsuleName), Namespace: capsuleNamespace}
			logsPVC := &corev1.PersistentVolumeClaim{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, logsPVCKey, logsPVC)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the stats PVC is created")
			statsPVCKey := types.NamespacedName{Name: fmt.Sprintf("%s-stats", capsuleName), Namespace: capsuleNamespace}
			statsPVC := &corev1.PersistentVolumeClaim{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, statsPVCKey, statsPVC)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})

	Context("When creating a ConsensusCapsule with UC sidecar enabled", func() {
		const (
			capsuleName      = "node-uc-enabled"
			capsuleNamespace = "consensus-uc-enabled-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Orbit: capsuleNamespace, Scope: "node0", Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "hedera.config.version=0\nnetty.mode=TEST"},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should include the uc container in the StatefulSet", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   2,
					AccountId:                "0.0.5",
					Weight:                   1,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{
								Enabled: true,
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "uc",
									ImageTag:   "1.0.0",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			stsKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, stsKey, sts) == nil
			}, timeout, interval).Should(BeTrue())

			var names []string
			for _, c := range sts.Spec.Template.Spec.Containers {
				names = append(names, c.Name)
			}
			Expect(names).To(ContainElement("uc"))

			var ucImage string
			for _, c := range sts.Spec.Template.Spec.Containers {
				if c.Name == "uc" {
					ucImage = c.Image
				}
			}
			Expect(ucImage).To(Equal("gcr.io/hedera-registry/uc:1.0.0"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})

	Context("When creating a ConsensusCapsule with UC sidecar disabled", func() {
		const (
			capsuleName      = "node-uc-disabled"
			capsuleNamespace = "consensus-uc-disabled-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Orbit: capsuleNamespace, Scope: "node0", Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "hedera.config.version=0\nnetty.mode=TEST"},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should set UCSidecarReady=False when UC is disabled", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   3,
					AccountId:                "0.0.6",
					Weight:                   1,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{
								Enabled: false,
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "uc",
									ImageTag:   "1.0.0",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			key := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			createdCapsule := &operatorv1alpha1.ConsensusCapsule{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, createdCapsule)
				if err != nil {
					return false
				}
				for _, condition := range createdCapsule.Status.Conditions {
					if condition.Type == "UCSidecarReady" && condition.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})

	Context("When creating a ConsensusCapsule with StartPolicy and DesiredRunState", func() {
		const (
			capsuleName      = "node-desired-state"
			capsuleNamespace = "consensus-desired-state-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Orbit: capsuleNamespace, Scope: "node0", Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "hedera.config.version=0\nnetty.mode=TEST"},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should configure AUTO_START_CONSENSUS_SERVICE=false for StartPolicy=Manual and converge Phase to Stopped", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   4,
					AccountId:                "0.0.7",
					Weight:                   1,
					StartPolicy:              operatorv1alpha1.StartPolicyManual,
					DesiredRunState:          operatorv1alpha1.RunStateStopped,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{
								Enabled: true,
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "uc",
									ImageTag:   "1.0.0",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			By("Verifying StatefulSet has AUTO_START_CONSENSUS_SERVICE=false")
			stsKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, stsKey, sts) == nil
			}, timeout, interval).Should(BeTrue())

			var nodeContainer *corev1.Container
			for i := range sts.Spec.Template.Spec.Containers {
				if sts.Spec.Template.Spec.Containers[i].Name == "consensus-node" {
					nodeContainer = &sts.Spec.Template.Spec.Containers[i]
					break
				}
			}
			Expect(nodeContainer).NotTo(BeNil())
			var autoStartEnv string
			for _, env := range nodeContainer.Env {
				if env.Name == "AUTO_START_CONSENSUS_SERVICE" {
					autoStartEnv = env.Value
				}
			}
			Expect(autoStartEnv).To(Equal("false"))

			By("Simulating a Ready Pod with matching STS revision")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-0", capsuleName),
					Namespace: capsuleNamespace,
					Labels:    consensus.PodLabels(capsule),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node", Image: "gcr.io/hedera-registry/consensus-node:0.65.1"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			pod.Status = corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).Should(Succeed())

			By("Verifying Capsule Phase rolls up to Stopped")
			capsuleKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.ConsensusCapsule{}
				if err := k8sClient.Get(ctx, capsuleKey, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseStopped))

			By("Patching DesiredRunState to Running")
			Eventually(func() error {
				got := &operatorv1alpha1.ConsensusCapsule{}
				if err := k8sClient.Get(ctx, capsuleKey, got); err != nil {
					return err
				}
				got.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning
				return k8sClient.Update(ctx, got)
			}, timeout, interval).Should(Succeed())

			By("Verifying Capsule Phase transitions to Running")
			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.ConsensusCapsule{}
				if err := k8sClient.Get(ctx, capsuleKey, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseRunning))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})
	It("Should reject actuation (precondition) if pod is not Ready", func() {
		capsuleName := "precondition-test"
		capsuleNamespace := "consensus-precondition"

		By("Creating namespace and dependent resources")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
		_ = k8sClient.Create(ctx, ns)

		orbit := &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
			Spec: operatorv1alpha1.OrbitSpec{
				Consensus: operatorv1alpha1.OrbitConsensus{
					Genesis: operatorv1alpha1.OrbitGenesis{AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"}},
				},
			},
		}
		_ = k8sClient.Create(ctx, orbit)

		capsule := &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				Orbit:                    capsuleNamespace,
				NodeId:                   5,
				AccountId:                "0.0.8",
				Weight:                   1,
				StartPolicy:              operatorv1alpha1.StartPolicyManual,
				DesiredRunState:          operatorv1alpha1.RunStateRunning,
				Log4j2ConfigRef:          "dummy",
				SettingsConfigRef:        "dummy",
				ApplicationPropertiesRef: "dummy",
				PodProperties: operatorv1alpha1.ConsensusPodProperties{
					Containers: operatorv1alpha1.ConsensusContainers{
						ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
							SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
								Repository: "gcr.io/test", ImageName: "node", ImageTag: "1.0",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

		By("Simulating a non-Ready Pod")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-0", capsuleName),
				Namespace: capsuleNamespace,
				Labels:    consensus.PodLabels(capsule),
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "consensus-node", Image: "gcr.io/test/node:1.0"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
		}
		Expect(k8sClient.Status().Update(ctx, pod)).Should(Succeed())

		By("Verifying no actuation occurs (s6 is not started because condition fails)")
		Consistently(func() bool {
			s6Runner.mu.Lock()
			defer s6Runner.mu.Unlock()
			return s6Runner.status[capsuleName]
		}, "3s", "500ms").Should(BeFalse(), "s6 runner should not be triggered when pod is not ready")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
	})
	Context("When s6 process crashes manually with DesiredRunState empty", func() {
		const (
			capsuleName      = "node-crash-test"
			capsuleNamespace = "consensus-crash-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleNamespace},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x00"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			log4j2 := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Orbit: capsuleNamespace, Scope: "node0", Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: capsuleNamespace, Scope: "node0", Content: "hedera.config.version=0\nnetty.mode=TEST"},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should map platform status appropriately and set RunState to Stopped without restart", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    capsuleNamespace,
					NodeId:                   5,
					AccountId:                "0.0.8",
					Weight:                   1,
					StartPolicy:              operatorv1alpha1.StartPolicyAuto,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{
								Enabled: true,
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "uc",
									ImageTag:   "1.0.0",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, capsule)).Should(Succeed())

			By("Setting initial mock S6 runner state to True")
			s6Runner.mu.Lock()
			s6Runner.status[capsuleName] = true
			s6Runner.mu.Unlock()

			By("Simulating a Ready Pod with Active platform status")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-0", capsuleName),
					Namespace: capsuleNamespace,
					Labels:    consensus.PodLabels(capsule),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node", Image: "gcr.io/hedera-registry/consensus-node:0.65.1"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			pod.Status = corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
					{Type: operatorv1alpha1.PodConditionPlatformStatus, Status: corev1.ConditionTrue, Reason: operatorv1alpha1.PlatformStatusActive},
				},
			}
			Expect(k8sClient.Status().Update(ctx, pod)).Should(Succeed())

			capsuleKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}

			By("Verifying Capsule Phase is Running and RunState is Running")
			Eventually(func() bool {
				got := &operatorv1alpha1.ConsensusCapsule{}
				if err := k8sClient.Get(ctx, capsuleKey, got); err != nil {
					return false
				}
				return got.Status.Phase == operatorv1alpha1.PhaseRunning && got.Status.RunState == operatorv1alpha1.RunStateRunning
			}, timeout, interval).Should(BeTrue())

			By("Simulating s6 process crash and UC updating pod condition")
			s6Runner.mu.Lock()
			s6Runner.status[capsuleName] = false
			s6Runner.mu.Unlock()

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, pod)).Should(Succeed())

			for i, cond := range pod.Status.Conditions {
				if cond.Type == operatorv1alpha1.PodConditionPlatformStatus {
					pod.Status.Conditions[i].Reason = operatorv1alpha1.PlatformStatusUnknown
					pod.Status.Conditions[i].Status = corev1.ConditionUnknown
					break
				}
			}
			Expect(k8sClient.Status().Update(ctx, pod)).Should(Succeed())

			// Trigger a reconcile manually on the Capsule since pod condition patch by itself in the envtest is flaking mapping
			got := &operatorv1alpha1.ConsensusCapsule{}
			Expect(k8sClient.Get(ctx, capsuleKey, got)).Should(Succeed())
			if got.Annotations == nil {
				got.Annotations = make(map[string]string)
			}
			got.Annotations["test-trigger"] = "true"
			Expect(k8sClient.Update(ctx, got)).Should(Succeed())

			By("Verifying RunState transitions to Stopped")

			Eventually(func() string {
				got := &operatorv1alpha1.ConsensusCapsule{}
				if err := k8sClient.Get(ctx, capsuleKey, got); err != nil {
					return ""
				}
				return string(got.Status.RunState)
			}, timeout, interval).Should(Equal(string(operatorv1alpha1.RunStateStopped)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})
})
