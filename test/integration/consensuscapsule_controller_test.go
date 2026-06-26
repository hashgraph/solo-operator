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
)

var _ = Describe("ConsensusCapsule Controller", func() {
	Context("When creating a ConsensusCapsule", func() {
		const (
			capsuleName      = "node0"
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
				ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "network-log4j2", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.Log4j2ConfigSpec{
					Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
				},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "network-settings", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.NodeSettingsSpec{
					Content: "checkSignedStateFromDisk,              1",
				},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "network-appprops", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
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
					Orbit:                    "test-orbit",
					NodeId:                   0,
					AccountId:                "0.0.3",
					Weight:                   1,
					Log4j2ConfigRef:          "network-log4j2",
					SettingsConfigRef:        "network-settings",
					ApplicationPropertiesRef: "network-appprops",
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

			By("Verifying the genesis-properties ConfigMap is created")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-genesis-properties-cm", capsuleName), Namespace: capsuleNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data).NotTo(HaveKey("api-permission.properties"))
			Expect(createdCM.Data).NotTo(HaveKey("application.properties"))
			Expect(createdCM.Data).NotTo(HaveKey("settings.txt"))
			Expect(createdCM.Data).NotTo(HaveKey("log4j2.xml"))

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
				ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "network-log4j2", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.Log4j2ConfigSpec{
					Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
				},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "network-settings", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.NodeSettingsSpec{
					Content: "checkSignedStateFromDisk,              1",
				},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "network-appprops", Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
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
					Orbit:                    "test-orbit",
					NodeId:                   1,
					AccountId:                "0.0.4",
					Weight:                   1,
					Log4j2ConfigRef:          "network-log4j2",
					SettingsConfigRef:        "network-settings",
					ApplicationPropertiesRef: "network-appprops",
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
				ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "network-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "network-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "network-appprops", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Content: "hedera.config.version=0\nnetty.mode=TEST"},
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
					Orbit:                    "test-orbit",
					NodeId:                   2,
					AccountId:                "0.0.5",
					Weight:                   1,
					Log4j2ConfigRef:          "network-log4j2",
					SettingsConfigRef:        "network-settings",
					ApplicationPropertiesRef: "network-appprops",
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
				ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "network-log4j2", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`},
			}
			err = k8sClient.Create(ctx, log4j2)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			nodeSettings := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "network-settings", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Content: "checkSignedStateFromDisk,              1"},
			}
			err = k8sClient.Create(ctx, nodeSettings)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "network-appprops", Namespace: capsuleNamespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Content: "hedera.config.version=0\nnetty.mode=TEST"},
			}
			err = k8sClient.Create(ctx, appProps)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should not include the uc container in the StatefulSet", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: capsuleNamespace},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    "test-orbit",
					NodeId:                   3,
					AccountId:                "0.0.6",
					Weight:                   1,
					Log4j2ConfigRef:          "network-log4j2",
					SettingsConfigRef:        "network-settings",
					ApplicationPropertiesRef: "network-appprops",
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

			stsKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, stsKey, sts) == nil
			}, timeout, interval).Should(BeTrue())

			for _, c := range sts.Spec.Template.Spec.Containers {
				Expect(c.Name).NotTo(Equal("uc"))
			}

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, capsule)).Should(Succeed())
		})
	})
})
