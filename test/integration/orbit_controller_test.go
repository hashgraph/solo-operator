// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("Orbit Controller", func() {
	Context("When creating an Orbit", func() {
		const (
			orbitName = "test-orbit"
		)

		ctx := context.Background()

		It("Should create dependent resources", func() {
			By("Creating a new Orbit")
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{
					Name: orbitName,
				},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			err := k8sClient.Create(ctx, orbit)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			// Verify Orbit is created (cluster-scoped, no namespace)
			orbitLookupKey := types.NamespacedName{Name: orbitName}
			createdOrbit := &operatorv1alpha1.Orbit{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, orbitLookupKey, createdOrbit)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the namespace is created with orbit name")
			namespaceLookupKey := types.NamespacedName{Name: orbitName}
			createdNamespace := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespaceLookupKey, createdNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the ConfigMap is created")
			cmLookupKey := types.NamespacedName{Name: "consensus-config", Namespace: orbitName}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cmLookupKey, createdCM)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Role is created")
			roleLookupKey := types.NamespacedName{Name: "consensus-capsule-role", Namespace: orbitName}
			createdRole := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleLookupKey, createdRole)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the headless Service is created")
			svcLookupKey := types.NamespacedName{Name: "consensus", Namespace: orbitName}
			createdSvc := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, svcLookupKey, createdSvc)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSvc.Spec.ClusterIP).To(Equal("None"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, orbitLookupKey, &operatorv1alpha1.Orbit{})
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When creating an Orbit with telemetry", func() {
		const (
			orbitName = "test-orbit-telemetry"
		)

		ctx := context.Background()

		It("Should create orbit with telemetry settings", func() {
			By("Creating a new Orbit with telemetry enabled")
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{
					Name: orbitName,
				},
				Spec: operatorv1alpha1.OrbitSpec{
					Telemetry: &operatorv1alpha1.OrbitTelemetry{
						Alloy: &operatorv1alpha1.OrbitAlloyDefaults{
							Enabled: true,
							Config: &operatorv1alpha1.AlloyCollectorConfig{
								Exporters: &operatorv1alpha1.AlloyExporters{
									Loki: &operatorv1alpha1.AlloyLokiExporter{
										Enabled:  true,
										Endpoint: "http://loki:3100/loki/api/v1/push",
									},
								},
							},
						},
					},
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).Should(Succeed())

			// Verify Orbit is created with telemetry config
			orbitLookupKey := types.NamespacedName{Name: orbitName}
			createdOrbit := &operatorv1alpha1.Orbit{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, orbitLookupKey, createdOrbit)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdOrbit.Spec.Telemetry).ToNot(BeNil())
			Expect(createdOrbit.Spec.Telemetry.Alloy.Enabled).To(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
		})
	})

	Context("When creating an Orbit with lifetime", func() {
		const (
			orbitName = "test-orbit-lifetime"
		)

		ctx := context.Background()

		It("Should create orbit with lifetime setting", func() {
			By("Creating a new Orbit with lifetime")
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{
					Name: orbitName,
				},
				Spec: operatorv1alpha1.OrbitSpec{
					Lifetime: "24h",
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).Should(Succeed())

			// Verify Orbit is created with lifetime
			orbitLookupKey := types.NamespacedName{Name: orbitName}
			createdOrbit := &operatorv1alpha1.Orbit{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, orbitLookupKey, createdOrbit)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdOrbit.Spec.Lifetime).To(Equal("24h"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
		})
	})
})
