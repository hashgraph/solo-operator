// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

// genesisServiceEndpoint mirrors the solo genesis-network.json endpoint structure.
type genesisServiceEndpoint struct {
	DomainName string `json:"domainName,omitempty"`
	Port       int32  `json:"port"`
}

// genesisNodeEntry is the "node" object inside a nodeMetadata element.
type genesisNodeEntry struct {
	NodeId              int64                    `json:"nodeId"`
	AccountId           genesisAccountId         `json:"accountId"`
	Description         string                   `json:"description"`
	GossipEndpoint      []genesisServiceEndpoint `json:"gossipEndpoint"`
	ServiceEndpoint     []genesisServiceEndpoint `json:"serviceEndpoint"`
	GossipCaCertificate string                   `json:"gossipCaCertificate"`
	GrpcCertificateHash string                   `json:"grpcCertificateHash"`
	Weight              int64                    `json:"weight"`
	Deleted             bool                     `json:"deleted"`
	AdminKey            string                   `json:"adminKey,omitempty"`
}

// genesisRosterEntry is the "rosterEntry" object inside a nodeMetadata element.
type genesisRosterEntry struct {
	NodeId              int64                    `json:"nodeId"`
	GossipEndpoint      []genesisServiceEndpoint `json:"gossipEndpoint"`
	GossipCaCertificate string                   `json:"gossipCaCertificate"`
	Weight              int64                    `json:"weight"`
}

type genesisAccountId struct {
	RealmNum   string `json:"realmNum"`
	ShardNum   string `json:"shardNum"`
	AccountNum string `json:"accountNum"`
}

type genesisNodeMetadata struct {
	Node        genesisNodeEntry   `json:"node"`
	RosterEntry genesisRosterEntry `json:"rosterEntry"`
}

type genesisNetworkFile struct {
	NodeMetadata []genesisNodeMetadata `json:"nodeMetadata"`
}

// CreateGenesisNetworkStep builds genesis-network.json from ConsensusCapsule secrets
// and stores it in the namespace-scoped "genesis-config" ConfigMap.
type CreateGenesisNetworkStep struct {
	client client.Client
}

// NewCreateGenesisNetworkStep creates a new CreateGenesisNetworkStep.
func NewCreateGenesisNetworkStep(c client.Client) *CreateGenesisNetworkStep {
	return &CreateGenesisNetworkStep{client: c}
}

// Name returns the step name.
func (s *CreateGenesisNetworkStep) Name() string {
	return "CreateGenesisNetwork"
}

// Run builds genesis-network.json and stores it in the genesis-config ConfigMap.
func (s *CreateGenesisNetworkStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
	logger := log.FromContext(ctx)

	namespace := resource.Namespace
	orbitName := resource.Spec.Orbit

	orbit := &operatorv1alpha1.Orbit{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		return s.fail(status, resource.Generation, fmt.Sprintf("orbit not found: %s: %v", orbitName, err))
	}

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

	// Derive shard/realm from orbit address book (defaults to 0/0)
	shardId, realmId := 0, 0
	if orbit.Spec.Consensus.Genesis != nil && orbit.Spec.Consensus.Genesis.AddressBook != nil {
		shardId = orbit.Spec.Consensus.Genesis.AddressBook.ShardId
		realmId = orbit.Spec.Consensus.Genesis.AddressBook.RealmId
	}

	var metadata []genesisNodeMetadata
	for _, capsule := range capsuleList.Items {
		entry, err := s.buildNodeMetadata(ctx, namespace, &capsule, shardId, realmId)
		if err != nil {
			logger.Error(err, "Failed to build node metadata", "capsule", capsule.Name)
			return s.fail(status, resource.Generation, fmt.Sprintf("capsule %s: %v", capsule.Name, err))
		}
		metadata = append(metadata, entry)
	}

	// Append external address entries from orbit
	if orbit.Spec.Consensus.Genesis != nil && orbit.Spec.Consensus.Genesis.AddressBook != nil {
		for _, ext := range orbit.Spec.Consensus.Genesis.AddressBook.ExternalAddresses {
			if ext.ExternalHostname == "" || ext.NodeAccount == "" {
				return s.fail(status, resource.Generation,
					fmt.Sprintf("external node %d: ExternalHostname and NodeAccount are required", ext.NodeId))
			}
			weight := ext.Weight
			if weight == 0 {
				weight = 1
			}
			gossipEndpoints := []genesisServiceEndpoint{{DomainName: ext.ExternalHostname, Port: ext.ExternalPort}}
			serviceEndpoints := []genesisServiceEndpoint{{DomainName: ext.ExternalHostname, Port: consensus.GrpcPort}}
			accountNum := common.AccountNum(ext.NodeAccount)

			// gossipCaCertificate: PEM → DER → base64 (optional; empty string if not provided)
			gossipCACert := ""
			if ext.PublicSigningCertificate != "" {
				derBytes, err := common.PemToDER([]byte(ext.PublicSigningCertificate))
				if err != nil {
					return s.fail(status, resource.Generation, fmt.Sprintf("external node %d: PEM→DER for gossipCaCertificate: %v", ext.NodeId, err))
				}
				gossipCACert = base64.StdEncoding.EncodeToString(derBytes)
			}

			node := genesisNodeEntry{
				NodeId:              ext.NodeId,
				AccountId:           genesisAccountId{RealmNum: fmt.Sprintf("%d", realmId), ShardNum: fmt.Sprintf("%d", shardId), AccountNum: accountNum},
				Description:         ext.SelfName,
				GossipEndpoint:      gossipEndpoints,
				ServiceEndpoint:     serviceEndpoints,
				GossipCaCertificate: gossipCACert,
				Weight:              int64(weight),
			}
			roster := genesisRosterEntry{
				NodeId:              ext.NodeId,
				GossipEndpoint:      gossipEndpoints,
				GossipCaCertificate: gossipCACert,
				Weight:              int64(weight),
			}
			metadata = append(metadata, genesisNodeMetadata{Node: node, RosterEntry: roster})
		}
		// Re-sort by nodeId after appending external entries
		sort.Slice(metadata, func(i, j int) bool {
			return metadata[i].Node.NodeId < metadata[j].Node.NodeId
		})
	}

	jsonBytes, err := json.MarshalIndent(genesisNetworkFile{NodeMetadata: metadata}, "", "  ")
	if err != nil {
		return s.fail(status, resource.Generation, fmt.Sprintf("failed to marshal genesis-network.json: %v", err))
	}

	if err := s.upsertConfigMap(ctx, namespace, orbitName, string(jsonBytes)); err != nil {
		logger.Error(err, "Failed to upsert genesis-config ConfigMap")
		return s.fail(status, resource.Generation, fmt.Sprintf("failed to upsert genesis-config: %v", err))
	}

	logger.Info("Created/updated genesis-config ConfigMap", "namespace", namespace)
	return s.succeed(status, resource.Generation, namespace)
}

// buildNodeMetadata constructs the nodeMetadata entry for one capsule.
// All sensitive material is read from referenced Secrets — never stored on the CR.
func (s *CreateGenesisNetworkStep) buildNodeMetadata(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule, shardId, realmId int) (genesisNodeMetadata, error) {
	hostname := consensus.InternalHostname(capsule)

	gossipPort := capsule.Spec.GossipPort
	if gossipPort == 0 {
		gossipPort = consensus.GossipPort
	}
	grpcPort := capsule.Spec.GrpcPort
	if grpcPort == 0 {
		grpcPort = consensus.GrpcPort
	}

	gossipEndpoints := []genesisServiceEndpoint{{DomainName: hostname, Port: gossipPort}}
	serviceEndpoints := []genesisServiceEndpoint{{DomainName: hostname, Port: grpcPort}}

	// Parse accountNum from "shard.realm.account" — fall back to derived value
	accountNum := common.AccountNum(capsule.Spec.AccountId)
	if accountNum == "" {
		accountNum = fmt.Sprintf("%d", 3+capsule.Spec.NodeId)
	}

	weight := capsule.Spec.Weight
	if weight == 0 {
		weight = 1
	}

	// gossipCaCertificate: PEM signing cert → DER → base64
	gossipCACert, err := s.readSigningCertDER(ctx, namespace, capsule)
	if err != nil {
		return genesisNodeMetadata{}, fmt.Errorf("gossipCaCertificate: %w", err)
	}

	// grpcCertificateHash: SHA-384 of the DER-encoded gRPC TLS cert
	grpcHash, err := s.readGrpcCertHash(ctx, namespace, capsule)
	if err != nil {
		return genesisNodeMetadata{}, fmt.Errorf("grpcCertificateHash: %w", err)
	}

	// adminKey: ED25519 public key bytes from secret, hex-encoded (optional)
	adminKey, err := s.readAdminKey(ctx, namespace, capsule)
	if err != nil {
		return genesisNodeMetadata{}, fmt.Errorf("adminKey: %w", err)
	}

	node := genesisNodeEntry{
		NodeId:              capsule.Spec.NodeId,
		AccountId:           genesisAccountId{RealmNum: fmt.Sprintf("%d", realmId), ShardNum: fmt.Sprintf("%d", shardId), AccountNum: accountNum},
		Description:         capsule.Name,
		GossipEndpoint:      gossipEndpoints,
		ServiceEndpoint:     serviceEndpoints,
		GossipCaCertificate: gossipCACert,
		GrpcCertificateHash: grpcHash,
		Weight:              int64(weight),
		Deleted:             false,
		AdminKey:            adminKey,
	}

	roster := genesisRosterEntry{
		NodeId:              capsule.Spec.NodeId,
		GossipEndpoint:      gossipEndpoints,
		GossipCaCertificate: gossipCACert,
		Weight:              int64(weight),
	}

	return genesisNodeMetadata{Node: node, RosterEntry: roster}, nil
}

// readSigningCertDER reads the PEM signing certificate from the capsule's secret,
// converts it to DER, and returns it as a base64 string.
func (s *CreateGenesisNetworkStep) readSigningCertDER(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) (string, error) {
	if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
		return "", fmt.Errorf("no secrets configured on capsule %s", capsule.Name)
	}
	ref := capsule.Spec.Secrets.SecretResources.SigningPublicCertificate
	pemBytes, err := s.readSecretKey(ctx, namespace, ref)
	if err != nil {
		return "", err
	}
	derBytes, err := common.PemToDER(pemBytes)
	if err != nil {
		return "", fmt.Errorf("PEM→DER conversion failed for %s: %w", capsule.Name, err)
	}
	return base64.StdEncoding.EncodeToString(derBytes), nil
}

// readGrpcCertHash reads the gRPC TLS public certificate, decodes it to DER,
// and returns the SHA-384 hash as a hex string.
func (s *CreateGenesisNetworkStep) readGrpcCertHash(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) (string, error) {
	if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
		return "", fmt.Errorf("no secrets configured on capsule %s", capsule.Name)
	}
	ref := capsule.Spec.Secrets.SecretResources.GrpcTlsPublicCertificate
	pemBytes, err := s.readSecretKey(ctx, namespace, ref)
	if err != nil {
		return "", err
	}
	derBytes, err := common.PemToDER(pemBytes)
	if err != nil {
		return "", fmt.Errorf("PEM→DER conversion failed for %s: %w", capsule.Name, err)
	}
	sum := sha512.Sum384(derBytes)
	return fmt.Sprintf("%x", sum), nil
}

// readAdminKey reads the ED25519 admin public key from the referenced secret and returns
// it as a hex string. Returns "" (no error) when the reference is absent — the field is optional.
func (s *CreateGenesisNetworkStep) readAdminKey(ctx context.Context, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) (string, error) {
	if capsule.Spec.Secrets == nil ||
		capsule.Spec.Secrets.SecretResources == nil ||
		capsule.Spec.Secrets.SecretResources.AdminPublicKey == nil {
		return "", nil
	}
	ref := *capsule.Spec.Secrets.SecretResources.AdminPublicKey
	keyBytes, err := s.readSecretKey(ctx, namespace, ref)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", keyBytes), nil
}

// readSecretKey fetches a single key from a Kubernetes Secret.
func (s *CreateGenesisNetworkStep) readSecretKey(ctx context.Context, namespace string, ref operatorv1alpha1.KeyReference) ([]byte, error) {
	if ref.SecretName == "" {
		return nil, fmt.Errorf("secret reference has no secretName")
	}
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.SecretName}, secret); err != nil {
		return nil, fmt.Errorf("secret %s not found: %w", ref.SecretName, err)
	}
	keyName := ref.KeyName
	if keyName == "" {
		keyName = "value"
	}
	data, ok := secret.Data[keyName]
	if !ok {
		return nil, fmt.Errorf("key %q not found in secret %s", keyName, ref.SecretName)
	}
	return data, nil
}

// upsertConfigMap creates or updates the genesis-config ConfigMap.
func (s *CreateGenesisNetworkStep) upsertConfigMap(ctx context.Context, namespace, orbitName, genesisJSON string) error {
	data := map[string]string{
		consensus.GenesisNetworkFileName: genesisJSON,
	}

	cm := &corev1.ConfigMap{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: consensus.GenesisConfigMapName}, cm)
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consensus.GenesisConfigMapName,
				Namespace: namespace,
				Labels:    common.StandardLabels(consensus.GenesisConfigMapName, orbitName),
			},
			Data: data,
		}
		return s.client.Create(ctx, cm)
	}
	if err != nil {
		return err
	}
	cm.Data = data
	cm.Labels = common.StandardLabels(consensus.GenesisConfigMapName, orbitName)
	return s.client.Update(ctx, cm)
}

func (s *CreateGenesisNetworkStep) succeed(status *GenesisStatus, generation int64, namespace string) *GenesisStatus {
	status.Phase = operatorv1alpha1.PhaseSucceeded
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             StepConditionReasonGenesisNetwork,
		Message:            fmt.Sprintf("genesis-network.json written to %s/%s", namespace, consensus.GenesisConfigMapName),
		ObservedGeneration: generation,
	})
	return status
}

func (s *CreateGenesisNetworkStep) fail(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             StepConditionReasonGenesisNetwork,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step completed successfully.
func (s *CreateGenesisNetworkStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonGenesisNetwork)
}

// ShouldSkip skips the step if already succeeded or a prior step failed.
func (s *CreateGenesisNetworkStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
