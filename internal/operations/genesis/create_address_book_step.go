// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"fmt"
	"sort"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

const (
	// AddressBookBinConfigMapName is the ConfigMap holding the binary address book for the mirror importer.
	AddressBookBinConfigMapName = "addressbook-bin"
	// AddressBookBinKey is the key within the ConfigMap.
	AddressBookBinKey = "addressbook.bin"

	StepConditionReasonAddressBook = "AddressBookBin"
)

// CreateAddressBookStep builds a binary NodeAddressBook protobuf from ConsensusCapsule secrets
// and stores it in the addressbook-bin ConfigMap for the mirror importer bootstrap.
type CreateAddressBookStep struct {
	client client.Client
}

func NewCreateAddressBookStep(c client.Client) *CreateAddressBookStep {
	return &CreateAddressBookStep{client: c}
}

func (s *CreateAddressBookStep) Name() string { return "CreateAddressBook" }

func (s *CreateAddressBookStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
	logger := log.FromContext(ctx)
	namespace := resource.Namespace
	orbitName := resource.Spec.Orbit

	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		return s.fail(status, resource.Generation, fmt.Sprintf("failed to list capsules: %v", err))
	}
	if len(capsuleList.Items) == 0 {
		return s.fail(status, resource.Generation, fmt.Sprintf("no consensus capsules in namespace %s", namespace))
	}

	sort.Slice(capsuleList.Items, func(i, j int) bool {
		return capsuleList.Items[i].Spec.NodeId < capsuleList.Items[j].Spec.NodeId
	})

	var nodes []*services.NodeAddress
	for i := range capsuleList.Items {
		node, err := s.buildNodeAddress(ctx, namespace, &capsuleList.Items[i])
		if err != nil {
			return s.fail(status, resource.Generation, fmt.Sprintf("capsule %s: %v", capsuleList.Items[i].Name, err))
		}
		nodes = append(nodes, node)
	}

	encoded, err := proto.Marshal(&services.NodeAddressBook{NodeAddress: nodes})
	if err != nil {
		return s.fail(status, resource.Generation, fmt.Sprintf("marshal NodeAddressBook: %v", err))
	}

	if err := s.upsertConfigMap(ctx, namespace, orbitName, encoded); err != nil {
		logger.Error(err, "Failed to upsert addressbook-bin ConfigMap")
		return s.fail(status, resource.Generation, fmt.Sprintf("failed to upsert addressbook-bin: %v", err))
	}

	logger.Info("Created/updated addressbook-bin ConfigMap", "namespace", namespace, "nodes", len(nodes))
	return s.succeed(status, resource.Generation, namespace)
}

func (s *CreateAddressBookStep) buildNodeAddress(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) (*services.NodeAddress, error) {
	if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
		return nil, fmt.Errorf("no secrets configured")
	}

	gossipPEM, err := s.readSecretKey(ctx, namespace, capsule.Spec.Secrets.SecretResources.SigningPublicCertificate)
	if err != nil {
		return nil, fmt.Errorf("gossip cert: %w", err)
	}
	rsaPubHex, err := common.ExtractPubKeyHex(gossipPEM)
	if err != nil {
		return nil, fmt.Errorf("gossip cert pubkey: %w", err)
	}

	tlsPEM, err := s.readSecretKey(ctx, namespace, capsule.Spec.Secrets.SecretResources.GrpcTlsPublicCertificate)
	if err != nil {
		return nil, fmt.Errorf("gRPC TLS cert: %w", err)
	}
	certHashHex, err := common.CertHashHex(tlsPEM)
	if err != nil {
		return nil, fmt.Errorf("gRPC TLS cert hash: %w", err)
	}

	host := consensus.InternalHostname(capsule)
	port := capsule.Spec.GrpcPort
	if port == 0 {
		port = consensus.GrpcPort
	}

	shardNum, realmNum, accountNum := common.ParseAccountIDParts(capsule.Spec.AccountId, capsule.Spec.NodeId)
	weight := capsule.Spec.Weight
	if weight == 0 {
		weight = 1
	}

	return &services.NodeAddress{
		NodeId: capsule.Spec.NodeId,
		NodeAccountId: &services.AccountID{
			ShardNum: shardNum,
			RealmNum: realmNum,
			Account:  &services.AccountID_AccountNum{AccountNum: accountNum},
		},
		NodeCertHash: []byte(certHashHex),
		RSA_PubKey:   rsaPubHex,
		ServiceEndpoint: []*services.ServiceEndpoint{
			{DomainName: host, Port: port},
		},
		Description: capsule.Name,
		Stake:       int64(weight),
	}, nil
}

func (s *CreateAddressBookStep) readSecretKey(ctx context.Context, namespace string, ref operatorv1alpha1.KeyReference) ([]byte, error) {
	if ref.SecretName == "" {
		return nil, fmt.Errorf("secret reference has no secretName")
	}
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.SecretName}, secret); err != nil {
		return nil, fmt.Errorf("secret %s: %w", ref.SecretName, err)
	}
	key := ref.KeyName
	if key == "" {
		key = "value"
	}
	data, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found in secret %s", key, ref.SecretName)
	}
	return data, nil
}

func (s *CreateAddressBookStep) upsertConfigMap(ctx context.Context, namespace, orbitName string, data []byte) error {
	cm := &corev1.ConfigMap{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: AddressBookBinConfigMapName}, cm)
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AddressBookBinConfigMapName,
				Namespace: namespace,
				Labels:    common.StandardLabels(AddressBookBinConfigMapName, orbitName),
			},
			BinaryData: map[string][]byte{AddressBookBinKey: data},
		}
		return s.client.Create(ctx, cm)
	}
	if err != nil {
		return err
	}
	cm.BinaryData = map[string][]byte{AddressBookBinKey: data}
	cm.Labels = common.StandardLabels(AddressBookBinConfigMapName, orbitName)
	return s.client.Update(ctx, cm)
}

func (s *CreateAddressBookStep) succeed(status *GenesisStatus, generation int64, namespace string) *GenesisStatus {
	status.Phase = operatorv1alpha1.PhaseSucceeded
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             StepConditionReasonAddressBook,
		Message:            fmt.Sprintf("addressbook-bin written to %s/%s", namespace, AddressBookBinConfigMapName),
		ObservedGeneration: generation,
	})
	return status
}

func (s *CreateAddressBookStep) fail(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             StepConditionReasonAddressBook,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

func (s *CreateAddressBookStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonAddressBook)
}

func (s *CreateAddressBookStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}

// ── helpers ──────────────────────────────────────────────────────────────────
// (cert and account helpers live in internal/common)
