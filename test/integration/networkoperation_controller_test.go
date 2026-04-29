// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func genSelfSignedCert() (pemBytes, derBytes []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	derBytes, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	pemBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return
}

func randSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var _ = Describe("NetworkOperation Controller", func() {
	Context("When creating a NetworkOperation for genesis", func() {
		const (
			netOpName      = "test-genesis"
			netOpNamespace = "netop-genesis-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created and have initial status", func() {
			By("Creating a NetworkOperation for genesis")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkGenesis))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When creating a NetworkOperation for check", func() {
		const (
			netOpName      = "test-check"
			netOpNamespace = "netop-check-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created for network check mode", func() {
			By("Creating a NetworkOperation for check")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkCheck,
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkCheck))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When creating a NetworkOperation for upgrade", func() {
		const (
			netOpName      = "test-upgrade"
			netOpNamespace = "netop-upgrade-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be created with upgrade workflow step", func() {
			By("Creating a NetworkOperation for upgrade")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netOpName,
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Mode: operatorv1alpha1.NetworkOperationModeNetworkUpgrade,
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						WorkflowStep: operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate,
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: netOpName, Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNetworkUpgrade))
			Expect(createdNetOp.Spec.NetworkUpgrade.WorkflowStep).To(Equal(operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})

	Context("When multiple NetworkOperations exist in same namespace", func() {
		const (
			netOpNamespace = "netop-multi-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should be handled by the sequencer", func() {
			By("Creating first NetworkOperation")
			netOp1 := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "netop-1",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			}
			Expect(k8sClient.Create(ctx, netOp1)).Should(Succeed())

			By("Creating second NetworkOperation")
			netOp2 := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "netop-2",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkCheck,
				},
			}
			Expect(k8sClient.Create(ctx, netOp2)).Should(Succeed())

			// Verify both are created
			Eventually(func() bool {
				op1 := &operatorv1alpha1.NetworkOperation{}
				op2 := &operatorv1alpha1.NetworkOperation{}
				err1 := k8sClient.Get(ctx, types.NamespacedName{Name: "netop-1", Namespace: netOpNamespace}, op1)
				err2 := k8sClient.Get(ctx, types.NamespacedName{Name: "netop-2", Namespace: netOpNamespace}, op2)
				return err1 == nil && err2 == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, netOp2)).Should(Succeed())
		})
	})

	Context("When creating node operations", func() {
		const (
			netOpNamespace = "netop-node-test"
		)

		ctx := context.Background()

		BeforeEach(func() {
			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: netOpNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should handle node add operation", func() {
			By("Creating a NodeAdd NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-add-op",
					Namespace: netOpNamespace,
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
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-add-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeAdd))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})

		It("Should handle node delete operation", func() {
			By("Creating a NodeDelete NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-delete-op",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNodeDelete,
					NodeOperation: &operatorv1alpha1.NodeOperationSpec{
						NodeDelete: &operatorv1alpha1.NodeDeleteSpec{
							NodeName: "node3",
							NodeID:   3,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-delete-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeDelete))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})

		It("Should handle node update operation", func() {
			By("Creating a NodeUpdate NetworkOperation")
			netOp := &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-update-op",
					Namespace: netOpNamespace,
				},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: "test-orbit",
					Mode:  operatorv1alpha1.NetworkOperationModeNodeUpdate,
					NodeOperation: &operatorv1alpha1.NodeOperationSpec{
						NodeUpdate: &operatorv1alpha1.NodeUpdateSpec{
							NodeName: "node2",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, netOp)).Should(Succeed())

			// Verify NetworkOperation is created
			netOpLookupKey := types.NamespacedName{Name: "node-update-op", Namespace: netOpNamespace}
			createdNetOp := &operatorv1alpha1.NetworkOperation{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, netOpLookupKey, createdNetOp)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdNetOp.Spec.Mode).To(Equal(operatorv1alpha1.NetworkOperationModeNodeUpdate))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, netOp)).Should(Succeed())
		})
	})
})

var _ = Describe("NetworkOperation Genesis — AddressBook", func() {
	const (
		ns        = "netop-addressbook-test"
		orbitName = "addressbook-orbit"
	)

	ctx := context.Background()

	BeforeEach(func() {
		ns_ := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		err := k8sClient.Create(ctx, ns_)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}

		orbit := &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: orbitName},
			Spec: operatorv1alpha1.OrbitSpec{
				Consensus: operatorv1alpha1.OrbitConsensus{
					NetworkSettings:          map[string]string{},
					ApplicationConfiguration: map[string]string{},
					PermissionConfiguration:  map[string]string{},
				},
			},
		}
		err = k8sClient.Create(ctx, orbit)
		if err != nil {
			Expect(err.Error()).To(ContainSubstring("already exists"))
		}
	})

	Context("When a genesis NetworkOperation completes with valid capsule secrets", func() {
		It("Should create the addressbook-bin ConfigMap with valid proto content", func() {
			suffix := randSuffix()
			netOpName := "genesis-ab-" + suffix
			sigSecretName := "ab-sig-" + suffix
			tlsSecretName := "ab-tls-" + suffix
			capsuleName := "ab-node-" + suffix

			sigPEM, _ := genSelfSignedCert()
			grpcPEM, _ := genSelfSignedCert()

			By("Creating capsule secrets")
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: sigSecretName, Namespace: ns},
				Data: map[string][]byte{
					"s-public.pem":  sigPEM,
					"s-private.pem": []byte("placeholder"),
				},
			})).Should(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: ns},
				Data: map[string][]byte{
					"hedera.crt": grpcPEM,
					"hedera.key": []byte("placeholder"),
				},
			})).Should(Succeed())

			By("Creating ConsensusCapsule")
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: ns},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:     orbitName,
					NodeId:    0,
					AccountId: "0.0.3",
					Weight:    1,
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "docker.io/hashgraph",
									ImageName:  "hedera-services",
									ImageTag:   "0.58.0",
								},
							},
						},
					},
					Secrets: &operatorv1alpha1.ConsensusSecrets{
						SecretResources: &operatorv1alpha1.ConsensusSecretResources{
							SigningPublicCertificate: operatorv1alpha1.KeyReference{SecretName: sigSecretName, KeyName: "s-public.pem"},
							SigningPrivateKey:        operatorv1alpha1.KeyReference{SecretName: sigSecretName, KeyName: "s-private.pem"},
							GrpcTlsPublicCertificate: operatorv1alpha1.KeyReference{SecretName: tlsSecretName, KeyName: "hedera.crt"},
							GrpcTlsPrivateKey:        operatorv1alpha1.KeyReference{SecretName: tlsSecretName, KeyName: "hedera.key"},
						},
					},
				},
			})).Should(Succeed())

			By("Creating genesis NetworkOperation")
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{Name: netOpName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: orbitName,
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			})).Should(Succeed())

			By("Waiting for NetworkOperation to reach Succeeded")
			Eventually(func() operatorv1alpha1.Phase {
				op := &operatorv1alpha1.NetworkOperation{}
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: netOpName, Namespace: ns}, op)
				return op.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			By("Verifying addressbook-bin ConfigMap exists with binary data")
			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "addressbook-bin", Namespace: ns}, cm)).Should(Succeed())
			Expect(cm.BinaryData).To(HaveKey("addressbook.bin"))
			Expect(cm.BinaryData["addressbook.bin"]).NotTo(BeEmpty())

			By("Verifying the binary data is a valid NodeAddressBook proto")
			var book services.NodeAddressBook
			Expect(proto.Unmarshal(cm.BinaryData["addressbook.bin"], &book)).Should(Succeed())
			Expect(book.NodeAddress).To(HaveLen(1))
			Expect(book.NodeAddress[0].NodeId).To(Equal(int64(0)))
			Expect(book.NodeAddress[0].NodeAccountId.GetAccountNum()).To(Equal(int64(3)))
			Expect(book.NodeAddress[0].RSA_PubKey).NotTo(BeEmpty())
			Expect(book.NodeAddress[0].NodeCertHash).NotTo(BeEmpty())
		})
	})

	Context("When a genesis NetworkOperation runs without capsule secrets", func() {
		It("Should fail and not create the addressbook-bin ConfigMap", func() {
			suffix := randSuffix()
			netOpName := "genesis-ab-nosec-" + suffix
			capsuleName := "ab-nosec-" + suffix

			By("Creating ConsensusCapsule without secrets")
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.ConsensusCapsule{
				ObjectMeta: metav1.ObjectMeta{Name: capsuleName, Namespace: ns},
				Spec: operatorv1alpha1.ConsensusCapsuleSpec{
					Orbit:  orbitName,
					NodeId: 0,
					Weight: 1,
					PodProperties: operatorv1alpha1.ConsensusPodProperties{
						Containers: operatorv1alpha1.ConsensusContainers{
							ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
								SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
									Repository: "docker.io/hashgraph",
									ImageName:  "hedera-services",
									ImageTag:   "0.58.0",
								},
							},
						},
					},
				},
			})).Should(Succeed())

			By("Creating genesis NetworkOperation")
			Expect(k8sClient.Create(ctx, &operatorv1alpha1.NetworkOperation{
				ObjectMeta: metav1.ObjectMeta{Name: netOpName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkOperationSpec{
					Orbit: orbitName,
					Mode:  operatorv1alpha1.NetworkOperationModeNetworkGenesis,
				},
			})).Should(Succeed())

			By("Waiting for NetworkOperation to reach Failed")
			Eventually(func() operatorv1alpha1.Phase {
				op := &operatorv1alpha1.NetworkOperation{}
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: netOpName, Namespace: ns}, op)
				return op.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseFailed))

			By("Verifying addressbook-bin ConfigMap was not created")
			cm := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "addressbook-bin", Namespace: ns}, cm)
			Expect(err).To(HaveOccurred())
		})
	})
})
