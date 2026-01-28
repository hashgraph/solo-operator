// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("NetworkOperation Controller", func() {
	Context("When creating a NetworkOperation for genesis", func() {
		const (
			netOpName      = "test-genesis"
			netOpNamespace = "netop-genesis-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created and have initial status", func() {
			By("Creating a NetworkOperation for genesis")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkGenesis))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When creating a NetworkOperation for check", func() {
		const (
			netOpName      = "test-check"
			netOpNamespace = "netop-check-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created for network check mode", func() {
			By("Creating a NetworkOperation for check")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkCheck,
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkCheck))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When creating a NetworkOperation for upgrade", func() {
		const (
			netOpName      = "test-upgrade"
			netOpNamespace = "netop-upgrade-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created with upgrade workflow step", func() {
			By("Creating a NetworkOperation for upgrade")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Mode: operatorv1alpha1.NetworkOperationModeNetworkUpgrade,
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						WorkflowStep: operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate,
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkUpgrade))
			Expect(createdNetOp.Spec.NetworkUpgrade.WorkflowStep).To(Equal(operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When multiple NetworkOperations exist in same namespace", func() {
		const (
			netOpNamespace = "netop-multi-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be handled by the sequencer", func() {
			By("Creating first NetworkOperation")
			netOp1 := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "netop-1",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			}
			Expect(k8sClient.Create(ctx, netOp1)).Should(Succeed())

			By("Creating second NetworkOperation")
			netOp2 := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "netop-2",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkCheck,
				},
			}
			Expect(k8sClient.Create(ctx, netOp2)).Should(Succeed())

			// Verify both are created
			Eventually(func() bool {
				op1 := &operatorv1alpha1.NetworkOperation{}
				op2 := &operatorv1alpha1.NetworkOperation{}
				err1 := k8sClient.Get(ctx, types.NamespacedName{Name: "netop-1", Namespace: netOpNamespace}, op1)
				err2 := k8sClient.Get(ctx, types.NamespacedName{Name: "netop-2", Namespace: netOpNamespace}, op2)
				return err1 == nil && err2 == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, netOp2)).Should(Succeed())
		})
	})

	Context("When creating node operations", func() {
		const (
			netOpNamespace = "netop-node-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should handle node add operation", func() {
			By("Creating a NodeAdd NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-add-op",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNodeAdd,
					NodeOperation: &operatorv1alpha1.NodeOperationSpec{
						NodeAdd: &operatorv1alpha1.NodeAddSpec{
							NodeName:  "node5",
							NodeID:    5,
							AccountID: "0.0.8",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-add-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeAdd))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})

		It("Should handle node delete operation", func() {
			By("Creating a NodeDelete NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-delete-op",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNodeDelete,
					NodeOperation: &operatorv1alpha1.NodeOperationSpec{
						NodeDelete: &operatorv1alpha1.NodeDeleteSpec{
							NodeName: "node3",
							NodeID:   3,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-delete-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeDelete))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})

		It("Should handle node update operation", func() {
			By("Creating a NodeUpdate NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-update-op",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNodeUpdate,
					NodeOperation: &operatorv1alpha1.NodeOperationSpec{
						NodeUpdate: &operatorv1alpha1.NodeUpdateSpec{
							NodeName: "node2",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-update-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeUpdate))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})
})
