// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("Orbit Controller", func() {
	Context("With a pre-existing namespace (CreateNamespace=false)", func() {
		const (
			orbitName = "orbit-precreated-ns"
			nsName    = "orbit-precreated-ns"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			if err := k8sClient.Create(ctx, ns); err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create dependent resources in the provided namespace", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Namespace: nsName,
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			if err := k8sClient.Create(ctx, orbit); err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}

			By("Verifying the Orbit becomes ready")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, &operatorv1alpha1.Orbit{})
			}, timeout, interval).Should(Succeed())

			By("Verifying owned resources live in the provided namespace")
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "consensus-config", Namespace: nsName}, &corev1.ConfigMap{})
			}, timeout, interval).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "consensus-capsule-role", Namespace: nsName}, &rbacv1.Role{})
			}, timeout, interval).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "consensus", Namespace: nsName}, &corev1.Service{})
			}, timeout, interval).Should(Succeed())

			By("Deleting the Orbit and verifying the namespace survives")
			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, &operatorv1alpha1.Orbit{}))
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, &corev1.Namespace{})).Should(Succeed())
		})
	})

	Context("With CreateNamespace=true (dev/ci convenience)", func() {
		const (
			orbitName = "test-orbit-created"
			nsName    = "test-orbit-created-ns"
		)

		ctx := context.Background()

		It("Should provision the namespace and tear it down on delete", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Namespace:       nsName,
					CreateNamespace: true,
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).Should(Succeed())

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, &corev1.Namespace{})
			}, timeout, interval).Should(Succeed())

			By("Cleaning up: envtest doesn't drain namespace finalizers, so strip them manually")
			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
			Eventually(func() bool {
				ns := &corev1.Namespace{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, ns)
				if apierrors.IsNotFound(err) {
					return true
				}
				if err != nil {
					return false
				}
				ns.Spec.Finalizers = nil
				_ = k8sClient.SubResource("finalize").Update(ctx, ns)
				return false
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, &operatorv1alpha1.Orbit{}))
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When creating an Orbit with telemetry", func() {
		const (
			orbitName = "test-orbit-telemetry"
			nsName    = "test-orbit-telemetry-ns"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			if err := k8sClient.Create(ctx, ns); err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create orbit with telemetry settings", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Namespace: nsName,
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

			createdOrbit := &operatorv1alpha1.Orbit{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, createdOrbit)
			}, timeout, interval).Should(Succeed())

			Expect(createdOrbit.Spec.Telemetry).ToNot(BeNil())
			Expect(createdOrbit.Spec.Telemetry.Alloy.Enabled).To(BeTrue())

			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
		})
	})

	Context("When creating an Orbit with lifetime", func() {
		const (
			orbitName = "test-orbit-lifetime"
			nsName    = "test-orbit-lifetime-ns"
		)

		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			if err := k8sClient.Create(ctx, ns); err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create orbit with lifetime setting", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Namespace: nsName,
					Lifetime:  "24h",
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{"ledger.id": "0x03"},
						ApplicationConfiguration: map[string]string{"hedera.realm": "0"},
						PermissionConfiguration:  map[string]string{"permission": "default"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).Should(Succeed())

			createdOrbit := &operatorv1alpha1.Orbit{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, createdOrbit)
			}, timeout, interval).Should(Succeed())

			Expect(createdOrbit.Spec.Lifetime).To(Equal("24h"))

			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
		})
	})
})
