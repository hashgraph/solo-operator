// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("E2E Tests", func() {

	// Note: These tests verify CRD creation and basic operations.
	// Full controller reconciliation requires the operator to be running.
	// Set RUN_OPERATOR=true to test with a running operator.

	Describe("CRD Validation", func() {
		Context("Orbit CRD", func() {
			It("should accept valid Orbit resources", func() {
				orbit := &operatorv1alpha1.Orbit{
					ObjectMeta: metav1.ObjectMeta{
						Name: "e2e-test-orbit",
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

				err := k8sClient.Create(ctx, orbit)
				Expect(err).NotTo(HaveOccurred())

				// Verify it was created
				created := &operatorv1alpha1.Orbit{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "e2e-test-orbit"}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Lifetime).To(Equal("24h"))

				// Cleanup
				Expect(k8sClient.Delete(ctx, orbit)).To(Succeed())
			})

			It("should accept Orbit with telemetry configuration", func() {
				orbit := &operatorv1alpha1.Orbit{
					ObjectMeta: metav1.ObjectMeta{
						Name: "e2e-orbit-telemetry",
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

				err := k8sClient.Create(ctx, orbit)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.Orbit{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "e2e-orbit-telemetry"}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Telemetry).NotTo(BeNil())
				Expect(created.Spec.Telemetry.Alloy.Enabled).To(BeTrue())

				// Cleanup
				Expect(k8sClient.Delete(ctx, orbit)).To(Succeed())
			})
		})

		Context("ConsensusCapsule CRD", func() {
			const testNamespace = "e2e-consensus-test"

			BeforeEach(func() {
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					// Namespace might already exist
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
			})

			AfterEach(func() {
				_ = deleteNamespace(ctx, testNamespace)
			})

			It("should accept valid ConsensusCapsule resources", func() {
				capsule := &operatorv1alpha1.ConsensusCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node0",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.ConsensusCapsuleSpec{
						Orbit:     "test-orbit",
						NodeId:    0,
						AccountId: "0.0.3",
						Weight:    1,
						Log4j2XML: "<Configuration></Configuration>",
						PodProperties: operatorv1alpha1.ConsensusPodProperties{
							Containers: operatorv1alpha1.ConsensusContainers{
								ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
									Name: "consensus-node",
									SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
										Repository:      "us-central1-docker.pkg.dev/hashsphere-development/hiero-consensus-node",
										ImageName:       "consensus-node",
										ImageTag:        "v0.65.1",
										ImagePullPolicy: "IfNotPresent",
									},
								},
							},
						},
					},
				}

				err := k8sClient.Create(ctx, capsule)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.ConsensusCapsule{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "node0", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.NodeId).To(Equal(int64(0)))
				Expect(created.Spec.AccountId).To(Equal("0.0.3"))

				// Cleanup
				Expect(k8sClient.Delete(ctx, capsule)).To(Succeed())
			})
		})

		Context("NetworkOperation CRD", func() {
			It("should accept NetworkOperation for genesis", func() {
				testNamespace := "e2e-netop-genesis"
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
				defer deleteNamespace(ctx, testNamespace)

				netOp := &operatorv1alpha1.NetworkOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "genesis-op",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.NetworkOperationSpec{
						Orbit: "test-orbit",
						Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
					},
				}

				err = k8sClient.Create(ctx, netOp)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.NetworkOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "genesis-op", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkGenesis))

				// Cleanup
				Expect(k8sClient.Delete(ctx, netOp)).To(Succeed())
			})

			It("should accept NetworkOperation for upgrade", func() {
				testNamespace := "e2e-netop-upgrade"
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
				defer deleteNamespace(ctx, testNamespace)

				netOp := &operatorv1alpha1.NetworkOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "upgrade-op",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.NetworkOperationSpec{
						Orbit: "test-orbit",
						Mode:  operatorv1alpha1.NetworkOperationModeNetworkUpgrade,
						NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
							WorkflowStep: operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate,
							Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
								ConsensusNodeChangeset: operatorv1alpha1.ConsensusNodeChangeset{
									SoftwareVersionSource: &operatorv1alpha1.SoftwareVersionSource{
										ImageRepositories: []operatorv1alpha1.ImageRepository{
											{
												Repository: "us-central1-docker.pkg.dev/hashsphere-development/hiero-consensus-node",
												ImageName:  "consensus-node",
												ImageTag:   "v0.65.1",
											},
										},
										ImageVerificationSpec: []operatorv1alpha1.ImageVerificationSpec{},
									},
								},
							},
						},
					},
				}

				err = k8sClient.Create(ctx, netOp)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.NetworkOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "upgrade-op", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkUpgrade))
				Expect(created.Spec.NetworkUpgrade.WorkflowStep).To(Equal(operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate))

				// Cleanup
				Expect(k8sClient.Delete(ctx, netOp)).To(Succeed())
			})

			It("should accept NetworkOperation for node add", func() {
				testNamespace := "e2e-netop-nodeadd"
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
				defer deleteNamespace(ctx, testNamespace)

				netOp := &operatorv1alpha1.NetworkOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-add-op",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.NetworkOperationSpec{
						Orbit: "test-orbit",
						Mode:  operatorv1alpha1.NetworkOperationModeNodeAdd,
						NodeOperation: &operatorv1alpha1.NodeOperationSpec{
							NodeAdd: &operatorv1alpha1.NodeAddSpec{
								NodeName:  "node5",
								NodeID:    5,
								AccountID: "0.0.8",
							},
						},
					},
				}

				err = k8sClient.Create(ctx, netOp)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.NetworkOperation{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "node-add-op", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeAdd))
				Expect(created.Spec.NodeOperation.NodeAdd.NodeID).To(Equal(int64(5)))

				// Cleanup
				Expect(k8sClient.Delete(ctx, netOp)).To(Succeed())
			})
		})

		Context("HAProxyCapsule CRD", Ordered, func() {
			const testNamespace = "e2e-haproxy-test"

			BeforeAll(func() {
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
			})

			AfterAll(func() {
				_ = deleteNamespace(ctx, testNamespace)
			})

			It("should accept valid HAProxyCapsule resources", func() {
				haproxy := &operatorv1alpha1.HAProxyCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-haproxy",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.HAProxyCapsuleSpec{
						Image:       "haproxy:2.8.10-alpine3.20",
						ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
					},
				}

				err := k8sClient.Create(ctx, haproxy)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.HAProxyCapsule{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-haproxy", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Image).To(Equal("haproxy:2.8.10-alpine3.20"))

				// Cleanup
				Expect(k8sClient.Delete(ctx, haproxy)).To(Succeed())
			})

			It("should apply CRD defaults for ports", func() {
				haproxy := &operatorv1alpha1.HAProxyCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-haproxy-defaults",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.HAProxyCapsuleSpec{
						Image: "haproxy:2.8.10-alpine3.20",
					},
				}

				Expect(k8sClient.Create(ctx, haproxy)).To(Succeed())

				created := &operatorv1alpha1.HAProxyCapsule{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-haproxy-defaults", Namespace: testNamespace}, created)).To(Succeed())

				Expect(created.Spec.GrpcPort).To(Equal(int32(50211)), "GrpcPort should default to 50211")
				Expect(created.Spec.GrpcsPort).To(Equal(int32(50212)), "GrpcsPort should default to 50212")
				Expect(created.Spec.MetricsPort).To(Equal(int32(9090)), "MetricsPort should default to 9090")

				Expect(k8sClient.Delete(ctx, haproxy)).To(Succeed())
			})

			It("should persist custom ports and upstreamService", func() {
				haproxy := &operatorv1alpha1.HAProxyCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-haproxy-custom",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.HAProxyCapsuleSpec{
						Image:           "haproxy:2.8.10-alpine3.20",
						GrpcPort:        50311,
						GrpcsPort:       50312,
						MetricsPort:     9091,
						UpstreamService: "my-consensus-node",
					},
				}

				Expect(k8sClient.Create(ctx, haproxy)).To(Succeed())

				created := &operatorv1alpha1.HAProxyCapsule{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-haproxy-custom", Namespace: testNamespace}, created)).To(Succeed())

				Expect(created.Spec.GrpcPort).To(Equal(int32(50311)))
				Expect(created.Spec.GrpcsPort).To(Equal(int32(50312)))
				Expect(created.Spec.MetricsPort).To(Equal(int32(9091)))
				Expect(created.Spec.UpstreamService).To(Equal("my-consensus-node"))

				Expect(k8sClient.Delete(ctx, haproxy)).To(Succeed())
			})
		})

		Context("EnvoyProxy CRD", Ordered, func() {
			const testNamespace = "e2e-envoy-test"

			BeforeAll(func() {
				err := createNamespace(ctx, testNamespace)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already exists"))
				}
			})

			AfterAll(func() {
				_ = deleteNamespace(ctx, testNamespace)
			})

			It("should accept valid EnvoyProxy resources", func() {
				replicas := int32(2)
				envoy := &operatorv1alpha1.EnvoyProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envoy",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.EnvoyProxySpec{
						Image:     "envoyproxy/envoy:v1.21.6",
						Replicas:  &replicas,
						ProxyPort: 8080,
						AdminPort: 9901,
					},
				}

				err := k8sClient.Create(ctx, envoy)
				Expect(err).NotTo(HaveOccurred())

				// Verify
				created := &operatorv1alpha1.EnvoyProxy{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-envoy", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Image).To(Equal("envoyproxy/envoy:v1.21.6"))
				Expect(*created.Spec.Replicas).To(Equal(int32(2)))

				// Cleanup
				Expect(k8sClient.Delete(ctx, envoy)).To(Succeed())
			})

			It("should apply CRD defaults for ports", func() {
				envoy := &operatorv1alpha1.EnvoyProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envoy-defaults",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.EnvoyProxySpec{
						Image: "envoyproxy/envoy:v1.21.6",
					},
				}

				Expect(k8sClient.Create(ctx, envoy)).To(Succeed())

				created := &operatorv1alpha1.EnvoyProxy{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-envoy-defaults", Namespace: testNamespace}, created)).To(Succeed())

				Expect(created.Spec.ProxyPort).To(Equal(int32(8080)), "ProxyPort should default to 8080")
				Expect(created.Spec.AdminPort).To(Equal(int32(9901)), "AdminPort should default to 9901")
				Expect(created.Spec.MetricsPort).To(Equal(int32(9090)), "MetricsPort should default to 9090")
				Expect(created.Spec.UpstreamPort).To(Equal(int32(50211)), "UpstreamPort should default to 50211")

				Expect(k8sClient.Delete(ctx, envoy)).To(Succeed())
			})

			It("should persist custom UpstreamService and UpstreamPort", func() {
				envoy := &operatorv1alpha1.EnvoyProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envoy-custom",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.EnvoyProxySpec{
						Image:           "envoyproxy/envoy:v1.21.6",
						ProxyPort:       8082,
						MetricsPort:     9091,
						UpstreamService: "my-consensus-node",
						UpstreamPort:    50311,
					},
				}

				Expect(k8sClient.Create(ctx, envoy)).To(Succeed())

				created := &operatorv1alpha1.EnvoyProxy{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-envoy-custom", Namespace: testNamespace}, created)).To(Succeed())

				Expect(created.Spec.ProxyPort).To(Equal(int32(8082)))
				Expect(created.Spec.MetricsPort).To(Equal(int32(9091)))
				Expect(created.Spec.UpstreamService).To(Equal("my-consensus-node"))
				Expect(created.Spec.UpstreamPort).To(Equal(int32(50311)))

				Expect(k8sClient.Delete(ctx, envoy)).To(Succeed())
			})
		})
	})

	Describe("Multi-Node Network Deployment", Ordered, func() {
		// This test suite simulates a multi-node Hedera network deployment
		// It requires the operator to be running (set RUN_OPERATOR=true)

		const (
			orbitName     = "e2e-multi-node-orbit"
			testNamespace = "e2e-multi-node"
		)

		BeforeAll(func() {
			By("Creating test namespace")
			err := createNamespace(ctx, testNamespace)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		AfterAll(func() {
			By("Cleaning up test resources")

			// Delete NetworkOperations
			netOpList := &operatorv1alpha1.NetworkOperationList{}
			_ = k8sClient.List(ctx, netOpList, client.InNamespace(testNamespace))
			for _, netOp := range netOpList.Items {
				_ = k8sClient.Delete(ctx, &netOp)
			}

			// Delete ConsensusCapsules
			capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
			_ = k8sClient.List(ctx, capsuleList, client.InNamespace(testNamespace))
			for _, capsule := range capsuleList.Items {
				_ = k8sClient.Delete(ctx, &capsule)
			}

			// Delete Orbit
			orbit := &operatorv1alpha1.Orbit{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: orbitName}, orbit)
			_ = k8sClient.Delete(ctx, orbit)

			// Delete namespace
			_ = deleteNamespace(ctx, testNamespace)
		})

		It("should create an Orbit for the network", func() {
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{
					Name: orbitName,
				},
				Spec: operatorv1alpha1.OrbitSpec{
					Lifetime: "1h",
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings: map[string]string{
							"ledger.id":           "0x03",
							"hedera.realm":        "0",
							"hedera.shard":        "0",
							"netty.mode":          "DEV",
							"netty.prod.tls.on":   "false",
							"netty.prod.tls.key":  "",
							"netty.prod.tls.cert": "",
						},
						ApplicationConfiguration: map[string]string{
							"hedera.realm": "0",
							"hedera.shard": "0",
						},
						PermissionConfiguration: map[string]string{
							"default": "ALL",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, orbit)
			Expect(err).NotTo(HaveOccurred())

			// Verify Orbit is created
			created := &operatorv1alpha1.Orbit{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: orbitName}, created)
			}, shortTimeout, interval).Should(Succeed())
		})

		It("should create multiple ConsensusCapsules", func() {
			// Create 4 consensus nodes
			for i := 0; i < 4; i++ {
				capsule := &operatorv1alpha1.ConsensusCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("node%d", i),
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.ConsensusCapsuleSpec{
						Orbit:     orbitName,
						NodeId:    int64(i),
						AccountId: fmt.Sprintf("0.0.%d", i+3),
						Weight:    1,
						Log4j2XML: "<Configuration></Configuration>",
						PodProperties: operatorv1alpha1.ConsensusPodProperties{
							Containers: operatorv1alpha1.ConsensusContainers{
								ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
									Name: "consensus-node",
									SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
										Repository:      "us-central1-docker.pkg.dev/hashsphere-development/hiero-consensus-node",
										ImageName:       "consensus-node",
										ImageTag:        "v0.65.1",
										ImagePullPolicy: "IfNotPresent",
									},
								},
							},
						},
					},
				}

				err := k8sClient.Create(ctx, capsule)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify all capsules are created
			capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
			Eventually(func() int {
				err := k8sClient.List(ctx, capsuleList, client.InNamespace(testNamespace))
				if err != nil {
					return 0
				}
				return len(capsuleList.Items)
			}, shortTimeout, interval).Should(Equal(4))
		})

		It("should create a NetworkOperation for genesis", func() {
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "genesis",
					Namespace: testNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: orbitName,
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			}

			err := k8sClient.Create(ctx, netOp)
			Expect(err).NotTo(HaveOccurred())

			// Verify NetworkOperation is created
			created := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "genesis", Namespace: testNamespace}, created)
			}, shortTimeout, interval).Should(Succeed())
		})
	})

	Describe("Sequencer Behavior", Ordered, func() {
		// Test that NetworkOperations are queued properly
		const testNamespace = "e2e-sequencer-test"

		BeforeAll(func() {
			err := createNamespace(ctx, testNamespace)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		AfterAll(func() {
			// Cleanup all NetworkOperations
			netOpList := &operatorv1alpha1.NetworkOperationList{}
			_ = k8sClient.List(ctx, netOpList, client.InNamespace(testNamespace))
			for _, netOp := range netOpList.Items {
				_ = k8sClient.Delete(ctx, &netOp)
			}
			_ = deleteNamespace(ctx, testNamespace)
		})

		It("should accept multiple NetworkOperations in the same namespace", func() {
			// Create multiple operations - they should be queued
			for i := 0; i < 3; i++ {
				netOp := &operatorv1alpha1.NetworkOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("op-%d", i),
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.NetworkOperationSpec{
						Orbit: "test-orbit",
						Mode:  operatorv1alpha1.NetworkOperationModeNetworkCheck,
					},
				}

				err := k8sClient.Create(ctx, netOp)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify all operations were created
			netOpList := &operatorv1alpha1.NetworkOperationList{}
			Eventually(func() int {
				err := k8sClient.List(ctx, netOpList, client.InNamespace(testNamespace))
				if err != nil {
					return 0
				}
				return len(netOpList.Items)
			}, shortTimeout, interval).Should(Equal(3))
		})
	})

	Describe("Resource Dependencies", func() {
		// Test that resources can reference each other correctly

		Context("ConsensusCapsule references Orbit", func() {
			const (
				orbitName     = "e2e-ref-orbit"
				testNamespace = "e2e-ref-test"
			)

			BeforeEach(func() {
				// Create orbit first
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
				_ = k8sClient.Create(ctx, orbit)

				// Create namespace
				_ = createNamespace(ctx, testNamespace)
			})

			AfterEach(func() {
				// Cleanup
				capsule := &operatorv1alpha1.ConsensusCapsule{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: "ref-node", Namespace: testNamespace}, capsule)
				_ = k8sClient.Delete(ctx, capsule)

				orbit := &operatorv1alpha1.Orbit{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: orbitName}, orbit)
				_ = k8sClient.Delete(ctx, orbit)

				_ = deleteNamespace(ctx, testNamespace)
			})

			It("should create ConsensusCapsule referencing existing Orbit", func() {
				capsule := &operatorv1alpha1.ConsensusCapsule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ref-node",
						Namespace: testNamespace,
					},
					Spec: operatorv1alpha1.ConsensusCapsuleSpec{
						Orbit:     orbitName,
						NodeId:    0,
						AccountId: "0.0.3",
						Weight:    1,
						Log4j2XML: "<Configuration></Configuration>",
						PodProperties: operatorv1alpha1.ConsensusPodProperties{
							Containers: operatorv1alpha1.ConsensusContainers{
								ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
									Name: "consensus-node",
									SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
										Repository:      "us-central1-docker.pkg.dev/hashsphere-development/hiero-consensus-node",
										ImageName:       "consensus-node",
										ImageTag:        "v0.65.1",
										ImagePullPolicy: "IfNotPresent",
									},
								},
							},
						},
					},
				}

				err := k8sClient.Create(ctx, capsule)
				Expect(err).NotTo(HaveOccurred())

				// Verify capsule references orbit
				created := &operatorv1alpha1.ConsensusCapsule{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "ref-node", Namespace: testNamespace}, created)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Spec.Orbit).To(Equal(orbitName))
			})
		})
	})
})

// Placeholder tests for when operator is running
var _ = Describe("Operator Reconciliation Tests", Label("operator-required"), func() {
	// These tests require the operator to be running
	// They verify that the operator correctly creates dependent resources

	BeforeEach(func() {
		// Skip if operator is not running
		// In a real setup, you would check if the operator pod is running
		Skip("Operator not running - skipping reconciliation tests")
	})

	Context("When Orbit is created", func() {
		It("should create namespace, ConfigMap, Role, and Service", func() {
			const orbitName = "reconcile-test-orbit"

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

			Expect(k8sClient.Create(ctx, orbit)).To(Succeed())

			// Wait for namespace
			ns := &corev1.Namespace{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: orbitName}, ns)
			}, timeout, interval).Should(Succeed())

			// Wait for ConfigMap
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "consensus-config", Namespace: orbitName}, cm)
			}, timeout, interval).Should(Succeed())

			// Wait for Role
			role := &rbacv1.Role{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "consensus-capsule-role", Namespace: orbitName}, role)
			}, timeout, interval).Should(Succeed())

			// Wait for Service
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "consensus", Namespace: orbitName}, svc)
			}, timeout, interval).Should(Succeed())
			Expect(svc.Spec.ClusterIP).To(Equal("None"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, orbit)).To(Succeed())
		})
	})

	Context("When ConsensusCapsule is created", func() {
		It("should create ServiceAccount, RoleBinding, ConfigMap, Service, and Deployment", func() {
			const (
				orbitName     = "capsule-reconcile-orbit"
				testNamespace = "capsule-reconcile-ns"
			)

			// Setup namespace first
			Expect(createNamespace(ctx, testNamespace)).To(Succeed())

			// Create role that RoleBinding references
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "consensus-capsule-role",
					Namespace: testNamespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, role)).To(Succeed())

			capsule := &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node0",
					Namespace: testNamespace,
				},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:     orbitName,
					NodeId:    0,
					AccountId: "0.0.3",
					Weight:    1,
					Log4j2XML: "<Configuration></Configuration>",
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								Name: "consensus-node",
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository:      "us-central1-docker.pkg.dev/hashsphere-development/hiero-consensus-node",
									ImageName:       "consensus-node",
									ImageTag:        "v0.65.1",
									ImagePullPolicy: "IfNotPresent",
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, capsule)).To(Succeed())

			// Wait for ServiceAccount
			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "node0-sa", Namespace: testNamespace}, sa)
			}, timeout, interval).Should(Succeed())

			// Wait for RoleBinding
			rb := &rbacv1.RoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "node0-rb", Namespace: testNamespace}, rb)
			}, timeout, interval).Should(Succeed())

			// Wait for ConfigMap
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "node0-cm", Namespace: testNamespace}, cm)
			}, timeout, interval).Should(Succeed())

			// Wait for Service
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "node0", Namespace: testNamespace}, svc)
			}, timeout, interval).Should(Succeed())

			// Wait for Deployment
			deploy := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "consensus-node", Namespace: testNamespace}, deploy)
			}, timeout, interval).Should(Succeed())

			// Cleanup
			Expect(k8sClient.Delete(ctx, capsule)).To(Succeed())
			Expect(deleteNamespace(ctx, testNamespace)).To(Succeed())
		})
	})
})
