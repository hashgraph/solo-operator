// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("EnvoyProxy Controller", func() {
	Context("When creating an EnvoyProxy", func() {
		const (
			envoyName      = "test-envoy"
			envoyNamespace = "envoy-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: envoyNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create all dependent resources", func() {
			By("Creating a new EnvoyProxy")
			replicas := int32(2)
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:     "envoyproxy/envoy:v1.21.6",
					Replicas:  &replicas,
					ProxyPort: 8081,
					AdminPort: 9901,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			// Verify EnvoyProxy is created
			envoyLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdEnvoy := &operatorv1alpha1.EnvoyProxy{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, envoyLookupKey, createdEnvoy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the ConfigMap is created")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoyName), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data["envoy.yaml"]).ToNot(BeEmpty())

			By("Verifying the Deployment is created with correct replicas")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdDeploy.Spec.Template.Spec.Containers[0].Image).To(Equal("envoyproxy/envoy:v1.21.6"))
			Expect(*createdDeploy.Spec.Replicas).To(Equal(int32(2)))

			By("Verifying the Deployment has probes configured")
			container := createdDeploy.Spec.Template.Spec.Containers[0]
			Expect(container.LivenessProbe).ToNot(BeNil())
			Expect(container.ReadinessProbe).ToNot(BeNil())

			By("Verifying the Service is created")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSvc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(createdSvc.Spec.Ports[0].Port).To(Equal(int32(8081)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with custom config", func() {
		const (
			envoyName      = "test-envoy-custom"
			envoyNamespace = "envoy-custom-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: envoyNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should use custom config files", func() {
			By("Creating an EnvoyProxy with custom files")
			customConfig := `admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901
static_resources:
  listeners: []
  clusters: []
`
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image: "envoyproxy/envoy:v1.21.6",
					Files: map[string]string{
						"envoy.yaml": customConfig,
					},
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the ConfigMap contains custom config")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoyName), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data["envoy.yaml"]).To(Equal(customConfig))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When updating an EnvoyProxy", func() {
		const (
			envoyName      = "test-envoy-update"
			envoyNamespace = "envoy-update-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: envoyNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update replicas", func() {
			By("Creating an EnvoyProxy")
			replicas := int32(1)
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:    "envoyproxy/envoy:v1.21.6",
					Replicas: &replicas,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying initial replica count")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil && createdDeploy.Spec.Replicas != nil && *createdDeploy.Spec.Replicas == 1
			}, timeout, interval).Should(BeTrue())

			By("Updating replica count")
			envoyLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			updatedEnvoy := &operatorv1alpha1.EnvoyProxy{}
			Expect(k8sClient.Get(ctx, envoyLookupKey, updatedEnvoy)).Should(Succeed())

			newReplicas := int32(3)
			updatedEnvoy.Spec.Replicas = &newReplicas
			Expect(k8sClient.Update(ctx, updatedEnvoy)).Should(Succeed())

			By("Verifying deployment is updated")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil && createdDeploy.Spec.Replicas != nil && *createdDeploy.Spec.Replicas == 3
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, updatedEnvoy)).Should(Succeed())
		})
	})
})
