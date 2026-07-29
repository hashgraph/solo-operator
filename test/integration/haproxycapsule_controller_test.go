// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"fmt"
	"strings"

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
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("test-cert-data"),
					"tls.key": []byte("test-key-data"),
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
					Image:         "haproxy:2.8.10-alpine3.20",
					ServiceType:   operatorv1alpha1.HAProxyServiceTypeClusterIP,
					TLSSecretName: tlsSecretName,
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

	Context("When creating an HAProxyCapsule with default health checks", func() {
		const (
			haproxyName      = "test-haproxy-health"
			haproxyNamespace = "haproxy-health-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: haproxyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should configure startup, liveness, and readiness probes by default", func() {
			By("Creating an HAProxyCapsule with no explicit health check config")
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

			By("Verifying probes are set on the container")
			deployLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			container := createdDeploy.Spec.Template.Spec.Containers[0]
			Expect(container.StartupProbe).ToNot(BeNil(), "startup probe should be set by default")
			Expect(container.LivenessProbe).ToNot(BeNil(), "liveness probe should be set by default")
			Expect(container.ReadinessProbe).ToNot(BeNil(), "readiness probe should be set by default")

			By("Verifying startup probe uses default values")
			Expect(container.StartupProbe.InitialDelaySeconds).To(Equal(int32(5)))
			Expect(container.StartupProbe.PeriodSeconds).To(Equal(int32(5)))
			Expect(container.StartupProbe.FailureThreshold).To(Equal(int32(12)))

			By("Verifying liveness probe uses default values")
			Expect(container.LivenessProbe.PeriodSeconds).To(Equal(int32(10)))
			Expect(container.LivenessProbe.FailureThreshold).To(Equal(int32(3)))
			Expect(container.LivenessProbe.TimeoutSeconds).To(Equal(int32(1)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})

	})

	Context("When creating an HAProxyCapsule with node scheduling", func() {
		const (
			haproxyName      = "test-haproxy-sched"
			haproxyNamespace = "haproxy-sched-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: haproxyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should propagate nodeSelector and tolerations to the pod spec", func() {
			By("Creating an HAProxyCapsule with scheduling constraints")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
					NodeSelector: map[string]string{
						"sphere.hashgraph.io/role": "auxiliary",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "sphere.hashgraph.io/role",
							Operator: corev1.TolerationOpEqual,
							Value:    "auxiliary",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying nodeSelector is applied to the pod spec")
			deployLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			podSpec := createdDeploy.Spec.Template.Spec
			Expect(podSpec.NodeSelector).To(HaveKeyWithValue("sphere.hashgraph.io/role", "auxiliary"))
			Expect(podSpec.Tolerations).To(HaveLen(1))
			Expect(podSpec.Tolerations[0].Key).To(Equal("sphere.hashgraph.io/role"))
			Expect(podSpec.Tolerations[0].Value).To(Equal("auxiliary"))
			Expect(podSpec.Tolerations[0].Effect).To(Equal(corev1.TaintEffectNoSchedule))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})
	})

	Context("When creating an HAProxyCapsule with additional labels", func() {
		const (
			haproxyName      = "test-haproxy-labels"
			haproxyNamespace = "haproxy-labels-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: haproxyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should apply additional labels to deployment and service without affecting selector", func() {
			By("Creating an HAProxyCapsule with additional labels")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
					AdditionalLabels: map[string]string{
						"operator.solo.hiero.org/node-id":   "0",
						"operator.solo.hiero.org/node-name": "node1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying additional labels appear on the Deployment metadata")
			deployLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeploy.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-id", "0"))
			Expect(createdDeploy.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-name", "node1"))

			By("Verifying additional labels appear on the pod template")
			Expect(createdDeploy.Spec.Template.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-id", "0"))
			Expect(createdDeploy.Spec.Template.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-name", "node1"))

			By("Verifying additional labels do NOT appear in the pod selector")
			Expect(createdDeploy.Spec.Selector.MatchLabels).ToNot(HaveKey("operator.solo.hiero.org/node-id"))
			Expect(createdDeploy.Spec.Selector.MatchLabels).ToNot(HaveKey("operator.solo.hiero.org/node-name"))

			By("Verifying additional labels appear on the Service")
			svcLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-id", "0"))
			Expect(createdSvc.Labels).To(HaveKeyWithValue("operator.solo.hiero.org/node-name", "node1"))

			By("Verifying additional labels do NOT appear in the service selector")
			Expect(createdSvc.Spec.Selector).ToNot(HaveKey("operator.solo.hiero.org/node-id"))
			Expect(createdSvc.Spec.Selector).ToNot(HaveKey("operator.solo.hiero.org/node-name"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})
	})

	Context("When creating an HAProxyCapsule with service configuration", func() {
		const (
			haproxyName      = "test-haproxy-svc"
			haproxyNamespace = "haproxy-svc-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: haproxyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should set ExternalTrafficPolicy=Local by default for LoadBalancer services", func() {
			By("Creating an HAProxyCapsule with LoadBalancer service type (default)")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeLoadBalancer,
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying ExternalTrafficPolicy=Local on the Service")
			svcLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Spec.ExternalTrafficPolicy).To(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})

		It("Should propagate service annotations to the Service", func() {
			By("Creating an HAProxyCapsule with service annotations")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName + "-annot",
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeLoadBalancer,
					ServiceAnnotations: map[string]string{
						"cloud.google.com/neg": `{"ingress":true}`,
					},
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying annotations appear on the Service")
			svcLookupKey := types.NamespacedName{Name: haproxy.Name, Namespace: haproxyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil && createdSvc.Annotations != nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Annotations).To(HaveKeyWithValue("cloud.google.com/neg", `{"ingress":true}`))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})

		It("Should include stats listener on port 9090 in the ConfigMap", func() {
			By("Creating an HAProxyCapsule")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName + "-stats",
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Verifying the ConfigMap contains stats on port 9090")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", haproxy.Name), Namespace: haproxyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(strings.Contains(createdCM.Data["haproxy.cfg"], "bind *:9090")).To(BeTrue(), "stats listener should bind on port 9090")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, haproxy)).Should(Succeed())
		})
	})

	Context("When updating an HAProxyCapsule's service annotations", func() {
		const (
			haproxyName      = "test-haproxy-update-svc"
			haproxyNamespace = "haproxy-update-svc-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: haproxyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update service annotations when CR is updated", func() {
			By("Creating an HAProxyCapsule")
			haproxy := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haproxyName,
					Namespace: haproxyNamespace,
				},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeLoadBalancer,
				},
			}
			Expect(k8sClient.Create(ctx, haproxy)).Should(Succeed())

			By("Waiting for Service to be created")
			svcLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating the HAProxyCapsule with service annotations")
			haproxyLookupKey := types.NamespacedName{Name: haproxyName, Namespace: haproxyNamespace}
			updatedHAProxy := &operatorv1alpha1.HAProxyCapsule{}
			Eventually(func() error {
				if err := k8sClient.Get(ctx, haproxyLookupKey, updatedHAProxy); err != nil {
					return err
				}
				updatedHAProxy.Spec.ServiceAnnotations = map[string]string{
					"cloud.google.com/neg": `{"ingress":true}`,
				}
				return k8sClient.Update(ctx, updatedHAProxy)
			}, timeout, interval).Should(Succeed())

			By("Verifying the Service is updated with the new annotations")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				if err != nil {
					return false
				}
				return createdSvc.Annotations != nil && createdSvc.Annotations["cloud.google.com/neg"] == `{"ingress":true}`
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, updatedHAProxy)).Should(Succeed())
		})
	})
})
