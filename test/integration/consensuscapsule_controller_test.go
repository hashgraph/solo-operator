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
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: capsuleNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				// Namespace might already exist
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			// Create the Role that the RoleBinding references
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "consensus-capsule-role",
					Namespace: capsuleNamespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			err = k8sClient.Create(ctx, role)
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
					Orbit:     "test-orbit",
					NodeId:    0,
					AccountId: "0.0.3",
					Weight:    1,
					Log4j2XML: "<Configuration></Configuration>",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								Image: "hashgraph/hedera-node:v0.50.0",
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

			By("Verifying the ConfigMap is created")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-cm", capsuleName), Namespace: capsuleNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data["log4j2.xml"]).To(Equal("<Configuration></Configuration>"))

			By("Verifying the Service is created")
			svcLookupKey := types.NamespacedName{Name: capsuleName, Namespace: capsuleNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Deployment is created")
			deployLookupKey := types.NamespacedName{Name: "consensus-node", Namespace: capsuleNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdDeploy.Spec.Template.Spec.Containers[0].Image).To(Equal("hashgraph/hedera-node:v0.50.0"))

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
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: capsuleNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			// Create the Role
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "consensus-capsule-role",
					Namespace: capsuleNamespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			err = k8sClient.Create(ctx, role)
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
					Orbit:     "test-orbit",
					NodeId:    1,
					AccountId: "0.0.4",
					Weight:    1,
					Log4j2XML: "<Configuration></Configuration>",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								Image: "hashgraph/hedera-node:v0.50.0",
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
})
