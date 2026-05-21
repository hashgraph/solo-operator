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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/operations/genesis"
)

var _ = Describe("NetworkGenesis Controller", func() {
	ctx := context.Background()

	Context("Spec.Roster path (no ConsensusCapsules)", func() {
		const (
			ns      = "ng-spec-roster"
			orbit   = "ng-spec-orbit"
			ngName  = "genesis-1"
			opID    = "genesis-spec-roster"
			nodeKey = 0
		)

		BeforeEach(func() {
			createNamespaceIgnoreExisting(ctx, ns)
			createSimpleOrbit(ctx, orbit)
		})

		It("produces all three ConfigMaps and reaches Phase=Succeeded", func() {
			ng := &operatorv1alpha1.NetworkGenesis{
				ObjectMeta: metav1.ObjectMeta{Name: ngName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkGenesisSpec{
					Orbit:       orbit,
					OperationID: opID,
					Roster: []operatorv1alpha1.NetworkGenesisRosterEntry{
						newRosterEntry(int64(nodeKey)),
					},
				},
			}
			Expect(k8sClient.Create(ctx, ng)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			for _, name := range []string{
				genesis.GenesisConfigMapName,
				genesis.AddressBookBinConfigMapName,
				genesis.ClusterConfigMapName,
			} {
				cm := &corev1.ConfigMap{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, cm)).
					To(Succeed(), "expected ConfigMap %q to exist", name)
			}

			Expect(k8sClient.Delete(ctx, ng)).To(Succeed())
		})
	})

	Context("Discovery path with Orbit external addresses", func() {
		const (
			ns        = "ng-discovery"
			orbitName = "ng-discovery-orbit"
			ngName    = "genesis-2"
		)

		BeforeEach(func() {
			createNamespaceIgnoreExisting(ctx, ns)

			// Orbit with one external address — no in-cluster capsules.
			extCertPEM := selfSignedPEM()
			orbit := &operatorv1alpha1.Orbit{
				ObjectMeta: metav1.ObjectMeta{Name: orbitName},
				Spec: operatorv1alpha1.OrbitSpec{
					Consensus: operatorv1alpha1.OrbitConsensus{
						NetworkSettings:          map[string]string{},
						ApplicationConfiguration: map[string]string{},
						PermissionConfiguration:  map[string]string{},
						Genesis: &operatorv1alpha1.OrbitGenesis{
							AddressBook: &operatorv1alpha1.OrbitAddressBook{
								ExternalAddresses: []operatorv1alpha1.AddressBookEntry{{
									NodeId:                   42,
									NickName:                 "ext-node42",
									SelfName:                 "ext-node42",
									Weight:                   1,
									InternalHostname:         "ext42.example.com",
									InternalPort:             50111,
									ExternalHostname:         "ext42.example.com",
									ExternalPort:             50111,
									NodeAccount:              "0.0.45",
									PublicSigningCertificate: string(extCertPEM),
								}},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, orbit)).To(Succeed())
		})

		It("builds the roster from Orbit ExternalAddresses alone", func() {
			ng := &operatorv1alpha1.NetworkGenesis{
				ObjectMeta: metav1.ObjectMeta{Name: ngName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkGenesisSpec{
					Orbit:       orbitName,
					OperationID: "genesis-discovery",
				},
			}
			Expect(k8sClient.Create(ctx, ng)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: genesis.GenesisConfigMapName, Namespace: ns}, cm)).To(Succeed())
			Expect(cm.Data[genesis.GenesisNetworkFileName]).To(ContainSubstring(`"nodeId": "42"`))
			Expect(cm.Data[genesis.GenesisNetworkFileName]).To(ContainSubstring("ext-node42"))
		})
	})

	Context("Idempotency", func() {
		const (
			ns        = "ng-idempotent"
			orbitName = "ng-idempotent-orbit"
			ngName    = "genesis-3"
		)

		BeforeEach(func() {
			createNamespaceIgnoreExisting(ctx, ns)
			createSimpleOrbit(ctx, orbitName)
		})

		It("produces byte-identical ConfigMaps when re-reconciled", func() {
			ng := &operatorv1alpha1.NetworkGenesis{
				ObjectMeta: metav1.ObjectMeta{Name: ngName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkGenesisSpec{
					Orbit:       orbitName,
					OperationID: "genesis-idempotent",
					Roster:      []operatorv1alpha1.NetworkGenesisRosterEntry{newRosterEntry(0)},
				},
			}
			Expect(k8sClient.Create(ctx, ng)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			first := snapshotCM(ctx, ns, genesis.GenesisConfigMapName)

			// Clear terminal Phase to force a second reconcile.
			got := &operatorv1alpha1.NetworkGenesis{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got)).To(Succeed())
			got.Status.Phase = ""
			got.Status.Conditions = nil
			Expect(k8sClient.Status().Update(ctx, got)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				g := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, g); err != nil {
					return ""
				}
				return g.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Expect(snapshotCM(ctx, ns, genesis.GenesisConfigMapName)).To(Equal(first))
		})
	})

	Context("Missing Orbit recovers when Orbit is created", func() {
		const (
			ns        = "ng-missing-orbit"
			orbitName = "ng-missing-orbit-orbit"
			ngName    = "genesis-4"
		)

		BeforeEach(func() {
			createNamespaceIgnoreExisting(ctx, ns)
			// Intentionally do NOT create the orbit yet.
		})

		It("transitions Failed → Succeeded once the Orbit appears", func() {
			ng := &operatorv1alpha1.NetworkGenesis{
				ObjectMeta: metav1.ObjectMeta{Name: ngName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkGenesisSpec{
					Orbit:       orbitName,
					OperationID: "genesis-missing-orbit",
					Roster:      []operatorv1alpha1.NetworkGenesisRosterEntry{newRosterEntry(0)},
				},
			}
			Expect(k8sClient.Create(ctx, ng)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseFailed))

			createSimpleOrbit(ctx, orbitName)

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, 2*timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))
		})
	})

	Context("Finalizer", func() {
		const (
			ns        = "ng-finalizer"
			orbitName = "ng-finalizer-orbit"
			ngName    = "genesis-5"
		)

		BeforeEach(func() {
			createNamespaceIgnoreExisting(ctx, ns)
			createSimpleOrbit(ctx, orbitName)
		})

		It("removes itself on delete and lets generated ConfigMaps persist", func() {
			ng := &operatorv1alpha1.NetworkGenesis{
				ObjectMeta: metav1.ObjectMeta{Name: ngName, Namespace: ns},
				Spec: operatorv1alpha1.NetworkGenesisSpec{
					Orbit:       orbitName,
					OperationID: "genesis-finalizer",
					Roster:      []operatorv1alpha1.NetworkGenesisRosterEntry{newRosterEntry(0)},
				},
			}
			Expect(k8sClient.Create(ctx, ng)).To(Succeed())

			Eventually(func() operatorv1alpha1.Phase {
				got := &operatorv1alpha1.NetworkGenesis{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, got); err != nil {
					return ""
				}
				return got.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.PhaseSucceeded))

			Expect(k8sClient.Delete(ctx, ng)).To(Succeed())

			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: ngName, Namespace: ns}, &operatorv1alpha1.NetworkGenesis{}))
			}, timeout, interval).Should(BeTrue())

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: genesis.GenesisConfigMapName, Namespace: ns}, cm)).To(Succeed())
		})
	})
})

func createNamespaceIgnoreExisting(ctx context.Context, name string) {
	err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})
	if err != nil {
		Expect(err.Error()).To(ContainSubstring("already exists"))
	}
}

func createSimpleOrbit(ctx context.Context, name string) {
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: operatorv1alpha1.OrbitSpec{
			Consensus: operatorv1alpha1.OrbitConsensus{
				NetworkSettings:          map[string]string{},
				ApplicationConfiguration: map[string]string{},
				PermissionConfiguration:  map[string]string{},
			},
		},
	}
	err := k8sClient.Create(ctx, orbit)
	if err != nil {
		Expect(err.Error()).To(ContainSubstring("already exists"))
	}
}

func newRosterEntry(nodeID int64) operatorv1alpha1.NetworkGenesisRosterEntry {
	sigPEM := selfSignedPEM()
	grpcPEM := selfSignedPEM()
	return operatorv1alpha1.NetworkGenesisRosterEntry{
		NodeID:              nodeID,
		AccountID:           fmt.Sprintf("0.0.%d", 3+nodeID),
		Description:         fmt.Sprintf("node%d", nodeID),
		GossipHost:          fmt.Sprintf("node%d.svc", nodeID),
		GossipPort:          50111,
		ServiceHost:         fmt.Sprintf("node%d.svc", nodeID),
		ServicePort:         50211,
		GossipCaCertificate: string(sigPEM),
		GrpcCertificate:     string(grpcPEM),
		Weight:              1,
	}
}

func selfSignedPEM() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func snapshotCM(ctx context.Context, ns, name string) string {
	cm := &corev1.ConfigMap{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, cm)).To(Succeed())
	return cm.Data[genesis.GenesisNetworkFileName]
}
