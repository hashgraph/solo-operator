// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

func MaterialsFromSpec(entries []operatorv1alpha1.NetworkGenesisRosterEntry) ([]RosterMaterial, error) {
	out := make([]RosterMaterial, 0, len(entries))
	for _, e := range entries {
		m, err := materialFromSpecEntry(e)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", e.NodeID, err)
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out, nil
}

func materialFromSpecEntry(e operatorv1alpha1.NetworkGenesisRosterEntry) (RosterMaterial, error) {
	signingPEM := []byte(e.GossipCaCertificate)
	signingDER, err := common.PemToDER(signingPEM)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gossipCaCertificate PEM→DER: %w", err)
	}
	rsaHex, err := common.ExtractPubKeyHex(signingPEM)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gossipCaCertificate pubkey: %w", err)
	}
	grpcHash, err := common.CertHashHex([]byte(e.GrpcCertificate))
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("grpcCertificate hash: %w", err)
	}

	weight := e.Weight
	if weight == 0 {
		weight = 1
	}
	return RosterMaterial{
		NodeID:           e.NodeID,
		AccountID:        e.AccountID,
		Description:      e.Description,
		GossipHost:       e.GossipHost,
		GossipPort:       e.GossipPort,
		ServiceHost:      e.ServiceHost,
		ServicePort:      e.ServicePort,
		Weight:           weight,
		AdminKeyHex:      e.AdminKey,
		SigningCertPEM:   signingPEM,
		SigningCertDER:   signingDER,
		SigningRSAPubKey: rsaHex,
		GrpcCertHashHex:  grpcHash,
		ExternalAddress:  e.GossipHost,
	}, nil
}

// MaterialsFromCluster discovers nodes from ConsensusCapsule CRs in the
// namespace and the orbit's external addresses. The caller passes the
// already-fetched orbit so discovery and cluster-config see the same
// snapshot (avoids a stale-read window).
func MaterialsFromCluster(ctx context.Context, c client.Client, namespace string, orbit *operatorv1alpha1.Orbit) ([]RosterMaterial, error) {
	capsules := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := c.List(ctx, capsules, client.InNamespace(namespace)); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "list capsules in %s", namespace), ReasonCapsuleListFailed),
			"Confirm the ServiceAccount has list permission on consensuscapsules.hedera.hashgraph.com",
		)
	}

	out := make([]RosterMaterial, 0, len(capsules.Items))
	for i := range capsules.Items {
		m, err := materialFromCapsule(ctx, c, namespace, &capsules.Items[i])
		if err != nil {
			return nil, fmt.Errorf("capsule %s: %w", capsules.Items[i].Name, err)
		}
		out = append(out, m)
	}

	for _, ext := range orbit.Spec.Consensus.Genesis.AddressBook.ExternalAddresses {
		m, err := materialFromExternal(ext)
		if err != nil {
			return nil, fmt.Errorf("external node %d: %w", ext.NodeId, err)
		}
		out = append(out, m)
	}

	if len(out) == 0 {
		return nil, errx.WithHints(
			errx.WithReason(errValidation.New("no consensus capsules in namespace %s and no external addresses on orbit %q", namespace, orbit.Name), ReasonRosterEmpty),
			"Create at least one ConsensusCapsule in the namespace, or configure externalAddresses on the orbit",
		)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out, nil
}

func materialFromCapsule(ctx context.Context, c client.Client, namespace string, capsule *operatorv1alpha1.ConsensusCapsule) (RosterMaterial, error) {
	if capsule.Spec.Secrets == nil || capsule.Spec.Secrets.SecretResources == nil {
		return RosterMaterial{}, errx.WithHints(
			errx.WithReason(errValidation.New("no secrets configured"), ReasonSecretsMissing),
			"Set spec.secrets.secretResources on the ConsensusCapsule",
		)
	}
	secrets := capsule.Spec.Secrets.SecretResources

	signingPEM, err := readSecretKey(ctx, c, namespace, secrets.SigningPublicCertificate)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gossip signing cert: %w", err)
	}
	signingDER, err := common.PemToDER(signingPEM)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gossip signing cert PEM→DER: %w", err)
	}
	rsaHex, err := common.ExtractPubKeyHex(signingPEM)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gossip signing cert pubkey: %w", err)
	}

	grpcPEM, err := readSecretKey(ctx, c, namespace, secrets.GrpcTlsPublicCertificate)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gRPC TLS cert: %w", err)
	}
	grpcHash, err := common.CertHashHex(grpcPEM)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("gRPC TLS cert hash: %w", err)
	}

	adminHex, err := readAdminKey(ctx, c, namespace, secrets.AdminPublicKey)
	if err != nil {
		return RosterMaterial{}, fmt.Errorf("admin key: %w", err)
	}

	accountID := capsule.Spec.AccountId
	if accountID == "" {
		accountID = fmt.Sprintf("0.0.%d", 3+capsule.Spec.NodeId)
	}
	weight := capsule.Spec.Weight
	if weight == 0 {
		weight = 1
	}
	gossipPort := capsule.Spec.GossipPort
	if gossipPort == 0 {
		gossipPort = consensus.GossipPort
	}
	grpcPort := capsule.Spec.GrpcPort
	if grpcPort == 0 {
		grpcPort = consensus.GrpcPort
	}
	host := consensus.InternalHostname(capsule)

	external := capsule.Spec.ExternalAddress
	if external == "" {
		external = host
	}

	return RosterMaterial{
		NodeID:           capsule.Spec.NodeId,
		AccountID:        accountID,
		Description:      capsule.Name,
		GossipHost:       host,
		GossipPort:       gossipPort,
		ServiceHost:      host,
		ServicePort:      grpcPort,
		Weight:           int64(weight),
		AdminKeyHex:      adminHex,
		SigningCertPEM:   signingPEM,
		SigningCertDER:   signingDER,
		SigningRSAPubKey: rsaHex,
		GrpcCertHashHex:  grpcHash,
		ExternalAddress:  external,
	}, nil
}

func materialFromExternal(ext operatorv1alpha1.AddressBookEntry) (RosterMaterial, error) {
	if ext.ExternalHostname == "" || ext.NodeAccount == "" {
		return RosterMaterial{}, errx.WithHints(
			errx.WithReason(errValidation.New("ExternalHostname and NodeAccount are required"), ReasonExternalAddrInvalid),
			"Set externalHostname and nodeAccount on the orbit external address entry",
		)
	}
	weight := ext.Weight
	if weight == 0 {
		weight = 1
	}
	var signingPEM, signingDER []byte
	var rsaHex string
	if ext.PublicSigningCertificate != "" {
		signingPEM = []byte(ext.PublicSigningCertificate)
		d, err := common.PemToDER(signingPEM)
		if err != nil {
			return RosterMaterial{}, fmt.Errorf("gossipCaCertificate PEM→DER: %w", err)
		}
		signingDER = d
		if rsa, err := common.ExtractPubKeyHex(signingPEM); err == nil {
			rsaHex = rsa
		}
	}
	return RosterMaterial{
		NodeID:           ext.NodeId,
		AccountID:        "0.0." + common.AccountNum(ext.NodeAccount),
		Description:      ext.SelfName,
		GossipHost:       ext.ExternalHostname,
		GossipPort:       ext.ExternalPort,
		ServiceHost:      ext.ExternalHostname,
		ServicePort:      consensus.GrpcPort,
		Weight:           int64(weight),
		SigningCertPEM:   signingPEM,
		SigningCertDER:   signingDER,
		SigningRSAPubKey: rsaHex,
		ExternalAddress:  ext.ExternalHostname,
	}, nil
}

func readSecretKey(ctx context.Context, c client.Client, namespace string, ref operatorv1alpha1.KeyReference) ([]byte, error) {
	if ref.SecretName == "" {
		return nil, errx.WithHints(
			errx.WithReason(errValidation.New("empty secretName"), ReasonSecretNameEmpty),
			"Set secretName on the key reference",
		)
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.SecretName}, secret); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "secret %s", ref.SecretName), ReasonSecretGetFailed),
			"Confirm the secret exists in the namespace and the ServiceAccount can get it",
		)
	}
	key := ref.KeyName
	if key == "" {
		key = "value"
	}
	data, ok := secret.Data[key]
	if !ok {
		return nil, errx.WithHints(
			errx.WithReason(errValidation.New("key %q not found in secret %s", key, ref.SecretName), ReasonSecretKeyMissing),
			"Confirm the secret contains the referenced key",
		)
	}
	return data, nil
}

func readAdminKey(ctx context.Context, c client.Client, namespace string, ref *operatorv1alpha1.KeyReference) (string, error) {
	if ref == nil {
		return "", nil
	}
	keyBytes, err := readSecretKey(ctx, c, namespace, *ref)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(keyBytes), nil
}
