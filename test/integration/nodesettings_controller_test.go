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

const validSettingsTxt = `# Hedera platform settings
checkSignedStateFromDisk,              1
state.recoveryFile,                    data/saved/RecoveryFile.dat`

var _ = Describe("NodeSettings Controller", func() {

	Context("When creating a NodeSettings with valid content", func() {
		const (
			crName    = "test-settings"
			namespace = "nodesettings-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create the ConfigMap with the correct content", func() {
			By("Creating a NodeSettings CR")
			cr := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Content: validSettingsTxt},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying the ConfigMap is created")
			cmKey := types.NamespacedName{Name: crName + "-settings-cm", Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdCM.Data["settings.txt"]).To(Equal(validSettingsTxt))

			By("Verifying the status is updated")
			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			updatedCR := &operatorv1alpha1.NodeSettings{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, crKey, updatedCR); err != nil {
					return ""
				}
				return updatedCR.Status.ConfigMapName
			}, timeout, interval).Should(Equal(crName + "-settings-cm"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})

	Context("When updating spec.content", func() {
		const (
			crName    = "test-settings-update"
			namespace = "nodesettings-update-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update the ConfigMap when content changes", func() {
			By("Creating a NodeSettings CR")
			cr := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Content: validSettingsTxt},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			cmKey := types.NamespacedName{Name: crName + "-settings-cm", Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating spec.content")
			updatedContent := "checkSignedStateFromDisk,              0\nuseTLS,                                1"

			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			Eventually(func() error {
				fetchedCR := &operatorv1alpha1.NodeSettings{}
				if err := k8sClient.Get(ctx, crKey, fetchedCR); err != nil {
					return err
				}
				fetchedCR.Spec.Content = updatedContent
				return k8sClient.Update(ctx, fetchedCR)
			}, timeout, interval).Should(Succeed())

			By("Verifying the ConfigMap is updated")
			Eventually(func() string {
				if err := k8sClient.Get(ctx, cmKey, createdCM); err != nil {
					return ""
				}
				return createdCM.Data["settings.txt"]
			}, timeout, interval).Should(Equal(updatedContent))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
			})).Should(Succeed())
		})
	})

	Context("When creating a NodeSettings with invalid content", func() {
		const (
			crName    = "test-settings-invalid"
			namespace = "nodesettings-invalid-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should not create a ConfigMap for malformed content", func() {
			By("Creating a NodeSettings with a line missing a comma")
			cr := &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.NodeSettingsSpec{Content: "badline without comma"},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying no ConfigMap is created")
			cmKey := types.NamespacedName{Name: crName + "-settings-cm", Namespace: namespace}
			Consistently(func() bool {
				cm := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, cmKey, cm) == nil
			}, "3s", interval).Should(BeFalse())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})
})
