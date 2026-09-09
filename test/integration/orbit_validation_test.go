// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("Orbit Invariants Validation", func() {
	ctx := context.Background()
	const (
		testNS1 = "orbit-val-ns1"
		testNS2 = "orbit-val-ns2"
	)

	BeforeEach(func() {
		for _, n := range []string{testNS1, testNS2} {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
			if err := k8sClient.Create(ctx, ns); err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		}
	})

	Context("CRD Schema Validations", func() {
		It("Rejects HelmCapsule when orbitRef is omitted", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "hc-no-orbit", Namespace: testNS1},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			err := k8sClient.Create(ctx, hc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.orbitRef"))
		})

		It("Rejects HAProxyCapsule when orbit is empty", func() {
			hp := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "hp-no-orbit", Namespace: testNS1},
				Spec: operatorv1alpha1.HAProxyCapsuleSpec{
					Orbit:       "",
					Image:       "haproxy:2.8.10-alpine3.20",
					ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
				},
			}
			err := k8sClient.Create(ctx, hp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.orbit"))
		})

		It("Rejects EnvoyProxy when orbit is empty", func() {
			ep := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{Name: "ep-no-orbit", Namespace: testNS1},
				Spec: operatorv1alpha1.EnvoyProxySpec{
					Orbit:       "",
					Image:       "envoyproxy/envoy-distroless:v1.21.6",
					ServiceType: operatorv1alpha1.ProxyServiceTypeClusterIP,
				},
			}
			err := k8sClient.Create(ctx, ep)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.orbit"))
		})

		It("Rejects BlockNodesConfig when orbit is empty", func() {
			bn := &operatorv1alpha1.BlockNodesConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "bn-no-orbit", Namespace: testNS1},
				Spec: operatorv1alpha1.BlockNodesConfigSpec{
					Orbit: "",
				},
			}
			err := k8sClient.Create(ctx, bn)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.orbit"))
		})

		It("Rejects Config CRDs when orbit is empty", func() {
			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "appprops-no-orbit", Namespace: testNS1},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
					Orbit:   "",
					Scope:   "node0",
					Content: "key=val",
				},
			}
			err := k8sClient.Create(ctx, appProps)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.orbit"))
		})
	})

	Context("ValidatingAdmissionPolicy - orbit-namespace-invariants", func() {
		It("Rejects ConsensusCapsule when spec.orbit does not match metadata.namespace", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node0-mismatch",
					Namespace: testNS1,
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    testNS2, // Mismatch!
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
						},
					},
				},
			}
			Eventually(func() string {
				err := k8sClient.Create(ctx, capsule)
				if err == nil {
					_ = k8sClient.Delete(ctx, capsule)
					return ""
				}
				return err.Error()
			}, timeout, interval).Should(ContainSubstring("must equal the target namespace"))
		})

		It("Admits ConsensusCapsule when spec.orbit matches metadata.namespace", func() {
			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node0-match",
					Namespace: testNS1,
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:                    testNS1, // Matches!
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
						},
					},
				},
			}
			err := k8sClient.Create(ctx, capsule)
			Expect(err).NotTo(HaveOccurred())
			_ = k8sClient.Delete(ctx, capsule)
		})

		It("Rejects HelmCapsule when spec.orbitRef.name does not match metadata.namespace", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "hc-mismatch", Namespace: testNS1},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					OrbitRef: &corev1.LocalObjectReference{Name: testNS2}, // Mismatch!
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			Eventually(func() string {
				err := k8sClient.Create(ctx, hc)
				if err == nil {
					_ = k8sClient.Delete(ctx, hc)
					return ""
				}
				return err.Error()
			}, timeout, interval).Should(And(
				ContainSubstring("spec.orbitRef.name (%s) must equal the target namespace (%s)", testNS2, testNS1),
				// Regression guard: the remediation must reference spec.orbitRef.name,
				// not spec.orbit, for a HelmCapsule (#1309 review).
				ContainSubstring("set spec.orbitRef.name to the namespace"),
			))
		})

		It("Admits HelmCapsule when spec.orbitRef.name matches metadata.namespace", func() {
			hc := &operatorv1alpha1.HelmCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: "hc-match", Namespace: testNS1},
				Spec: operatorv1alpha1.HelmCapsuleSpec{
					OrbitRef: &corev1.LocalObjectReference{Name: testNS1}, // Matches!
					Chart: operatorv1alpha1.HelmChartSpec{
						Repo:    "oci://ghcr.io/hashgraph",
						Chart:   "hedera-mirror",
						Version: "0.129.1",
					},
				},
			}
			err := k8sClient.Create(ctx, hc)
			Expect(err).NotTo(HaveOccurred())
			_ = k8sClient.Delete(ctx, hc)
		})

		It("Rejects ApplicationProperties when spec.orbit does not match metadata.namespace", func() {
			appProps := &operatorv1alpha1.ApplicationProperties{
				ObjectMeta: metav1.ObjectMeta{Name: "appprops-mismatch", Namespace: testNS1},
				Spec: operatorv1alpha1.ApplicationPropertiesSpec{
					Orbit:   testNS2, // Mismatch!
					Scope:   "node0",
					Content: "key=val",
				},
			}
			Eventually(func() string {
				err := k8sClient.Create(ctx, appProps)
				if err == nil {
					_ = k8sClient.Delete(ctx, appProps)
					return ""
				}
				return err.Error()
			}, timeout, interval).Should(ContainSubstring("must equal the target namespace"))
		})
	})
})
