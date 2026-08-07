// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("Orbit Controller", func() {
	ctx := context.Background()

	Context("Configuration-only object", func() {
		const orbitName = "orbit-cfg-only"

		It("Reaches Ready=True without touching any namespace", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Lifetime: "24h",
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x03"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).Should(Succeed())

			Eventually(func() string {
				got := &operatorv1alpha1.Orbit{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, got); err != nil {
					return ""
				}
				for _, c := range got.Status.Conditions {
					if c.Type == "Ready" {
						return string(c.Status)
					}
				}
				return ""
			}, timeout, interval).Should(Equal("True"))

			Expect(k8sClient.Delete(ctx, orbit)).Should(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, &operatorv1alpha1.Orbit{}))
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Cascade-delete of referencing Capsules and Proxies (cluster-wide)", func() {
		const (
			orbitName = "orbit-cascade"
			nsA       = "orbit-cascade-a"
			nsB       = "orbit-cascade-b"
		)

		BeforeEach(func() {
			for _, n := range []string{nsA, nsB} {
				if err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}); err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
			}
		})

		It("Deletes references in any namespace, not just one", func() {
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						Genesis: operatorv1alpha1.OrbitGenesis{
							AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x03"},
						},
					},
				},
			})).Should(Succeed())

			Expect(k8sClient.Create(ctx, &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-log4j2", Namespace: nsA},
				Spec: operatorv1alpha1.Log4j2ConfigSpec{
					Scope:   "node0",
					Content: `<?xml version="1.0" encoding="UTF-8"?><Configuration status="WARN"><Appenders/><Loggers><Root level="WARN"/></Loggers></Configuration>`,
				},
			})).Should(Succeed())
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NodeSettings{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-settings", Namespace: nsA},
				Spec: operatorv1alpha1.NodeSettingsSpec{
					Scope:   "node0",
					Content: "checkSignedStateFromDisk,              1",
				},
			})).Should(Succeed())
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "node0-application-properties", Namespace: nsA},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
					Scope:   "node0",
					Content: "hedera.config.version=0\nnetty.mode=TEST",
				},
			})).Should(Succeed())

			cc := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: nsA},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    orbitName,
					NodeId:                   0,
					AccountId:                "0.0.3",
					Weight:                   1,
					Log4j2ConfigRef:          "node0-log4j2",
					SettingsConfigRef:        "node0-settings",
					ApplicationPropertiesRef: "node0-application-properties",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "gcr.io/hedera-registry",
									ImageName:  "consensus-node",
									ImageTag:   "0.65.1",
								},
							},
							UC: &operatorv1alpha1.UcSidecar{Enabled: true},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cc)).Should(Succeed())

			hp := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "haproxy-node0", Namespace: nsB},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Orbit:       orbitName,
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
				},
			}
			Expect(k8sClient.Create(ctx, hp)).Should(Succeed())

			ep := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{Name: "envoy-node0", Namespace: nsB},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Orbit:       orbitName,
					Image:       "envoyproxy/envoy-distroless:v1.21.6",
					ServiceType: operatorv1alpha1.ProxyServiceTypeClusterIP,
				},
			}
			Expect(k8sClient.Create(ctx, ep)).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: orbitName}})).Should(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "node0", Namespace: nsA}, &operatorv1alpha1.ConsensusCapsule{})) &&
					apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "haproxy-node0", Namespace: nsB}, &operatorv1alpha1.HAProxyCapsule{})) &&
					apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "envoy-node0", Namespace: nsB}, &operatorv1alpha1.EnvoyProxy{}))
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: orbitName}, &operatorv1alpha1.Orbit{}))
			}, timeout, interval).Should(BeTrue())
		})
	})
})
