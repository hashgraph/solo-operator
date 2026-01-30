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

var _ = Describe("HAProxyCapsule Controller", func() {
	Context("When creating an HAProxyCapsule", func() {
		const (
			haproxyName      = "test-haproxy"
			haproxyNamespace = "haproxy-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: haproxyNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create all dependent resources", func() {
			By("Creating a new HAProxyCapsule")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			// Verify HAProxyCapsule is created
			haproxyLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdHAProxy := &operatorv1alpha1.HAProxyCapsule{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, haproxyLookupKey, createdHAProxy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the ConfigMap is created")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", haproxyName), Namespace: haproxyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data["haproxy.cfg"]).ToNot(BeEmpty())

			By("Verifying the Deployment is created")
			deployLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdDeploy.Spec.Template.Spec.Containers[0].Image).To(Equal("haproxy:2.8.10-alpine3.20"))

			By("Verifying the Service is created")
			svcLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSvc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})
	})

	Context("When creating an HAProxyCapsule with TLS secret", func() {
		const (
			haproxyName      = "test-haproxy-tls"
			haproxyNamespace = "haproxy-tls-test"
			tlsSecretName    = "tls-secret"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: haproxyNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			// Create TLS secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tlsSecretName,
					Namespace: haproxyNamespace,
				},
				Data: map[string][]byte{
					"hedera.pem": []byte("test-cert-data"),
				},
			}
			err = k8sClient.Create(ctx, secret)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create deployment with TLS volume", func() {
			By("Creating an HAProxyCapsule with TLS secret")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
					SecretName:  tlsSecretName,
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying the Deployment has TLS volume")
			deployLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				if err != nil {
					return false
				}
				// Check for TLS volume
				for _, vol := range createdDeploy.Spec.Template.Spec.Volumes {
					if vol.Name == "tls-secrets" && vol.Secret != nil {
						return vol.Secret.SecretName == tlsSecretName
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})
	})
})
