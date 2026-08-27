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

const validApplicationProperties = `# Solo application properties
hedera.config.version=0
netty.mode=TEST
staking.periodMins=1`

var _ = Describe("ApplicationProperties Controller", func() {

	Context("When creating an ApplicationProperties with valid content", func() {
		const (
			crName    = "test-application-properties-cm"
			namespace = "appprops-test"
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
			By("Creating an ApplicationProperties")
			cr := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: namespace, Scope: "node0", Content: validApplicationProperties},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying the ConfigMap is created")
			cmKey := types.NamespacedName{Name: "node0-application-properties", Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdCM.Data["application.properties"]).To(Equal(validApplicationProperties))

			By("Verifying the status is updated")
			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			updatedCR := &operatorv1alpha1.ApplicationProperties{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, crKey, updatedCR); err != nil {
					return ""
				}
				return updatedCR.Status.ConfigMapName
			}, timeout, interval).Should(Equal("node0-application-properties"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})

	Context("When updating spec.content", func() {
		const (
			crName    = "test-application-properties-cm-update"
			namespace = "appprops-update-test"
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
			By("Creating an ApplicationProperties")
			cr := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: namespace, Scope: "node0", Content: validApplicationProperties},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			cmKey := types.NamespacedName{Name: "node0-application-properties", Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating spec.content")
			updatedProps := `hedera.config.version=1
netty.mode=PROD
staking.periodMins=1440`

			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			fetchedCR := &operatorv1alpha1.ApplicationProperties{}
			Eventually(func() error {
				if err := k8sClient.Get(ctx, crKey, fetchedCR); err != nil {
					return err
				}
				fetchedCR.Spec.Content = updatedProps
				return k8sClient.Update(ctx, fetchedCR)
			}, timeout, interval).Should(Succeed())

			By("Verifying the ConfigMap is updated")
			Eventually(func() string {
				if err := k8sClient.Get(ctx, cmKey, createdCM); err != nil {
					return ""
				}
				return createdCM.Data["application.properties"]
			}, timeout, interval).Should(Equal(updatedProps))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, fetchedCR)).Should(Succeed())
		})
	})

	Context("When creating an ApplicationProperties with invalid content", func() {
		const (
			crName    = "test-application-properties-cm-invalid"
			namespace = "appprops-invalid-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should not create a ConfigMap for content without key=value format", func() {
			By("Creating an ApplicationProperties with invalid content")
			cr := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.ApplicationPropertiesSpec{Orbit: namespace, Scope: "node0", Content: "badline without equals"},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying no ConfigMap is created")
			cmKey := types.NamespacedName{Name: "node0-application-properties", Namespace: namespace}
			Consistently(func() bool {
				cm := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, cmKey, cm) == nil
			}, "3s", interval).Should(BeFalse())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})
})
