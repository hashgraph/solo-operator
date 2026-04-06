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
			Expect(container.StartupProbe).ToNot(BeNil())
			Expect(container.StartupProbe.HTTPGet).ToNot(BeNil())
			Expect(container.StartupProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(container.LivenessProbe).ToNot(BeNil())
			Expect(container.LivenessProbe.InitialDelaySeconds).To(Equal(int32(0)), "liveness should have no initial delay — startup probe gates it")
			Expect(container.ReadinessProbe).ToNot(BeNil())

			By("Verifying the Service is created")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSvc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
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

	Context("When creating an EnvoyProxy with metrics port", func() {
		const (
			envoyName      = "test-envoy-metrics"
			envoyNamespace = "envoy-metrics-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should expose default metrics port 9090 on the deployment and service", func() {
			By("Creating an EnvoyProxy without explicit metricsPort")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:     "envoyproxy/envoy:v1.21.6",
					ProxyPort: 8081,
					AdminPort: 9901,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the metrics container port is 9090")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			container := createdDeploy.Spec.Template.Spec.Containers[0]
			var metricsPort *corev1.ContainerPort
			for i := range container.Ports {
				if container.Ports[i].Name == "metrics" {
					metricsPort = &container.Ports[i]
					break
				}
			}
			Expect(metricsPort).ToNot(BeNil(), "metrics container port should be present")
			Expect(metricsPort.ContainerPort).To(Equal(int32(9090)))

			By("Verifying the metrics service port is 9090")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			var metricsSvcPort *corev1.ServicePort
			for i := range createdSvc.Spec.Ports {
				if createdSvc.Spec.Ports[i].Name == "metrics" {
					metricsSvcPort = &createdSvc.Spec.Ports[i]
					break
				}
			}
			Expect(metricsSvcPort).ToNot(BeNil(), "metrics service port should be present")
			Expect(metricsSvcPort.Port).To(Equal(int32(9090)))

			By("Verifying the ConfigMap contains a prometheus_listener on port 9090")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoyName), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			cfg := createdCM.Data["envoy.yaml"]
			Expect(strings.Contains(cfg, "prometheus_listener")).To(BeTrue(), "config should contain prometheus_listener")
			Expect(strings.Contains(cfg, "port_value: 9090")).To(BeTrue(), "prometheus listener should use port 9090")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})

		It("Should expose custom metrics port when specified", func() {
			By("Creating an EnvoyProxy with custom metricsPort")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName + "-custom",
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:       "envoyproxy/envoy:v1.21.6",
					ProxyPort:   8081,
					AdminPort:   9901,
					MetricsPort: 9100,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the metrics container port is the custom port")
			deployLookupKey := types.NamespacedName{Name: envoy.Name, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			container := createdDeploy.Spec.Template.Spec.Containers[0]
			var metricsPort *corev1.ContainerPort
			for i := range container.Ports {
				if container.Ports[i].Name == "metrics" {
					metricsPort = &container.Ports[i]
					break
				}
			}
			Expect(metricsPort).ToNot(BeNil())
			Expect(metricsPort.ContainerPort).To(Equal(int32(9100)))

			By("Verifying the ConfigMap uses the custom port in the prometheus_listener")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoy.Name), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(strings.Contains(createdCM.Data["envoy.yaml"], "port_value: 9100")).To(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with CORS config", func() {
		const (
			envoyName      = "test-envoy-cors"
			envoyNamespace = "envoy-cors-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should apply CORS overrides to the generated envoy.yaml", func() {
			By("Creating an EnvoyProxy with custom CORS config")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:     "envoyproxy/envoy:v1.21.6",
					ProxyPort: 8081,
					AdminPort: 9901,
					CORSConfig: &operatorv1alpha1.CORSConfig{
						AllowOrigins: []string{"https://example.com"},
						AllowMethods: "GET, POST",
						MaxAge:       "3600",
					},
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the ConfigMap contains the CORS overrides")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoyName), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			cfg := createdCM.Data["envoy.yaml"]
			Expect(strings.Contains(cfg, "https://example.com")).To(BeTrue(), "config should contain the custom origin")
			Expect(strings.Contains(cfg, "GET, POST")).To(BeTrue(), "config should contain the custom methods")
			Expect(strings.Contains(cfg, `"3600"`)).To(BeTrue(), "config should contain the custom max age")

			By("Verifying the default wildcard origin is not present")
			Expect(strings.Contains(cfg, "prefix: '*'")).To(BeFalse(), "default wildcard origin should be replaced by custom origins")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with node scheduling", func() {
		const (
			envoyName      = "test-envoy-sched"
			envoyNamespace = "envoy-sched-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should propagate nodeSelector and tolerations to the pod spec", func() {
			By("Creating an EnvoyProxy with scheduling constraints")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:     "envoyproxy/envoy:v1.21.6",
					ProxyPort: 8081,
					AdminPort: 9901,
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
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying nodeSelector is applied to the pod spec")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
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
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with additional labels", func() {
		const (
			envoyName      = "test-envoy-labels"
			envoyNamespace = "envoy-labels-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should apply additional labels to deployment and service without affecting selector", func() {
			By("Creating an EnvoyProxy with additional labels")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:     "envoyproxy/envoy:v1.21.6",
					ProxyPort: 8081,
					AdminPort: 9901,
					AdditionalLabels: map[string]string{
						"solo.hedera.com/node-id":   "0",
						"solo.hedera.com/node-name": "node1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying additional labels appear on the Deployment metadata")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeploy.Labels).To(HaveKeyWithValue("solo.hedera.com/node-id", "0"))
			Expect(createdDeploy.Labels).To(HaveKeyWithValue("solo.hedera.com/node-name", "node1"))

			By("Verifying additional labels appear on the pod template")
			Expect(createdDeploy.Spec.Template.Labels).To(HaveKeyWithValue("solo.hedera.com/node-id", "0"))
			Expect(createdDeploy.Spec.Template.Labels).To(HaveKeyWithValue("solo.hedera.com/node-name", "node1"))

			By("Verifying additional labels do NOT appear in the pod selector")
			Expect(createdDeploy.Spec.Selector.MatchLabels).ToNot(HaveKey("solo.hedera.com/node-id"))
			Expect(createdDeploy.Spec.Selector.MatchLabels).ToNot(HaveKey("solo.hedera.com/node-name"))

			By("Verifying additional labels appear on the Service")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Labels).To(HaveKeyWithValue("solo.hedera.com/node-id", "0"))
			Expect(createdSvc.Labels).To(HaveKeyWithValue("solo.hedera.com/node-name", "node1"))

			By("Verifying additional labels do NOT appear in the service selector")
			Expect(createdSvc.Spec.Selector).ToNot(HaveKey("solo.hedera.com/node-id"))
			Expect(createdSvc.Spec.Selector).ToNot(HaveKey("solo.hedera.com/node-name"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with service configuration", func() {
		const (
			envoyName      = "test-envoy-svc"
			envoyNamespace = "envoy-svc-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should set ExternalTrafficPolicy=Local by default for LoadBalancer services", func() {
			By("Creating an EnvoyProxy with LoadBalancer service type (default)")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:       "envoyproxy/envoy:v1.21.6",
					ProxyPort:   8081,
					AdminPort:   9901,
					ServiceType: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying ExternalTrafficPolicy=Local on the Service")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Spec.ExternalTrafficPolicy).To(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})

		It("Should propagate service annotations to the Service", func() {
			By("Creating an EnvoyProxy with service annotations")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName + "-annot",
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:       "envoyproxy/envoy:v1.21.6",
					ProxyPort:   8081,
					AdminPort:   9901,
					ServiceType: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
					ServiceAnnotations: map[string]string{
						"cloud.google.com/neg": `{"ingress":true}`,
					},
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying annotations appear on the Service")
			svcLookupKey := types.NamespacedName{Name: envoy.Name, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil && createdSvc.Annotations != nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdSvc.Annotations).To(HaveKeyWithValue("cloud.google.com/neg", `{"ingress":true}`))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When creating an EnvoyProxy with TLS", func() {
		const (
			envoyName      = "test-envoy-tls"
			envoyNamespace = "envoy-tls-test"
			tlsSecretName  = "envoy-tls-secret"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should mount TLS secret and include transport_socket in config", func() {
			By("Creating a TLS secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tlsSecretName,
					Namespace: envoyNamespace,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("test-cert-data"),
					"tls.key": []byte("test-key-data"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			By("Creating an EnvoyProxy with TLSSecretName")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:         "envoyproxy/envoy:v1.21.6",
					TLSSecretName: tlsSecretName,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the ConfigMap contains transport_socket")
			cmLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-config", envoyName), Namespace: envoyNamespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdCM.Data["envoy.yaml"]).To(ContainSubstring("transport_socket"))
			Expect(createdCM.Data["envoy.yaml"]).To(ContainSubstring("DownstreamTlsContext"))

			By("Verifying the Deployment has a TLS volume")
			deployLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			volumes := createdDeploy.Spec.Template.Spec.Volumes
			Expect(volumes).To(HaveLen(2), "expected config + tls volumes")
			tlsVolumeFound := false
			for _, v := range volumes {
				if v.Name == "tls" {
					tlsVolumeFound = true
					Expect(v.Secret).ToNot(BeNil())
					Expect(v.Secret.SecretName).To(Equal(tlsSecretName))
				}
			}
			Expect(tlsVolumeFound).To(BeTrue(), "TLS volume not found in deployment")

			container := createdDeploy.Spec.Template.Spec.Containers[0]
			tlsMountFound := false
			for _, m := range container.VolumeMounts {
				if m.Name == "tls" {
					tlsMountFound = true
					Expect(m.MountPath).To(Equal("/etc/envoy/certs"))
				}
			}
			Expect(tlsMountFound).To(BeTrue(), "TLS volume mount not found in container")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})

	Context("When updating an EnvoyProxy's service annotations", func() {
		const (
			envoyName      = "test-envoy-update-svc"
			envoyNamespace = "envoy-update-svc-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envoyNamespace},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update service annotations when CR is updated", func() {
			By("Creating an EnvoyProxy")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName,
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:       "envoyproxy/envoy:v1.21.6",
					ProxyPort:   8081,
					AdminPort:   9901,
					ServiceType: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Waiting for Service to be created")
			svcLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating the EnvoyProxy with service annotations")
			envoyLookupKey := types.NamespacedName{Name: envoyName, Namespace: envoyNamespace}
			updatedEnvoy := &operatorv1alpha1.EnvoyProxy{}
			Expect(k8sClient.Get(ctx, envoyLookupKey, updatedEnvoy)).Should(Succeed())

			updatedEnvoy.Spec.ServiceAnnotations = map[string]string{
				"cloud.google.com/neg": `{"ingress":true}`,
			}
			Expect(k8sClient.Update(ctx, updatedEnvoy)).Should(Succeed())

			By("Verifying the Service is updated with the new annotations")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				if err != nil {
					return false
				}
				return createdSvc.Annotations != nil && createdSvc.Annotations["cloud.google.com/neg"] == `{"ingress":true}`
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, updatedEnvoy)).Should(Succeed())
		})

		It("Should expose admin port on container when ExposeAdmin=true", func() {
			By("Creating an EnvoyProxy with ExposeAdmin=true")
			envoy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      envoyName + "-admin",
					Namespace: envoyNamespace,
				},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Image:       "envoyproxy/envoy:v1.21.6",
					ProxyPort:   8081,
					AdminPort:   9901,
					ServiceType: operatorv1alpha1.ProxyServiceTypeLoadBalancer,
					ExposeAdmin: true,
				},
			}
			Expect(k8sClient.Create(ctx, envoy)).Should(Succeed())

			By("Verifying the Deployment has the admin container port")
			deployLookupKey := types.NamespacedName{Name: envoy.Name, Namespace: envoyNamespace}
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deployLookupKey, createdDeploy)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			container := createdDeploy.Spec.Template.Spec.Containers[0]
			var adminPortFound bool
			for _, p := range container.Ports {
				if p.Name == "admin" && p.ContainerPort == 9901 {
					adminPortFound = true
					break
				}
			}
			Expect(adminPortFound).To(BeTrue(), "expected admin container port to be present when ExposeAdmin=true")
			Expect(container.Ports).To(HaveLen(3))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, envoy)).Should(Succeed())
		})
	})
})
