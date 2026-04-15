// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// Label keys — generic keys alias common; consensus-specific keys defined here.
const (
	LabelManagedBy = common.LabelManagedBy
	LabelPartOf    = common.LabelPartOf
	LabelOrbit     = common.LabelOrbit
	LabelComponent = common.LabelComponent

	LabelNodeId        = "operator.solo.hedera.com/node-id"
	LabelSoloNodeName  = "operator.solo.hedera.com/node-name"
	LabelSoloAccountId = "operator.solo.hedera.com/account-id"
	LabelSoloType      = "operator.solo.hedera.com/type"
	LabelSoloPromEndpt = "operator.solo.hedera.com/prometheus-endpoint"
)

// Label values — generic values alias common; consensus-specific values defined here.
const (
	ManagedByValue         = common.ManagedByValue
	PartOfConsensusNode    = "consensus-node"
	ComponentConsensusNode = common.ComponentConsensusNode
)

// Resource name suffixes
const (
	ServiceAccountSuffix = "-sa"
	RoleBindingSuffix    = "-rb"
	ConfigMapSuffix      = "-cm"
)

// Resource names
const (
	// ConsensusCapsuleRoleName is the name of the role created by Orbit
	ConsensusCapsuleRoleName = "consensus-capsule-role"
	// ConsensusConfigMapName is the name of the ConfigMap created by Orbit
	ConsensusConfigMapName = "consensus-config"
	// ClusterConfigMapName is the name of the ConfigMap created by genesis
	ClusterConfigMapName = "cluster-config"
	// PublicKeysConfigMapName is the name of the public keys ConfigMap created by genesis
	PublicKeysConfigMapName = "public-keys"
	// GenesisConfigMapName is the namespace-scoped ConfigMap containing genesis-network.json
	GenesisConfigMapName = "genesis-config"
	// GenesisNetworkFileName is the key/filename for the roster JSON inside GenesisConfigMapName
	GenesisNetworkFileName = "genesis-network.json"
)

// Port numbers — gossip/gRPC alias common; consensus-specific ports defined here.
const (
	GossipPort     = common.GossipPort
	GrpcPort       = common.GrpcPort
	GrpcsPort      = common.GrpcsPort
	PrometheusPort = 9999
	AlloyHTTPPort  = 12345
)

// ResolveGossipPort returns the gossip port from the capsule spec, falling back to the default.
func ResolveGossipPort(capsule *operatorv1alpha1.ConsensusCapsule) int32 {
	if capsule.Spec.GossipPort != 0 {
		return capsule.Spec.GossipPort
	}
	return GossipPort
}

// ResolveGrpcPort returns the gRPC port from the capsule spec, falling back to the default.
func ResolveGrpcPort(capsule *operatorv1alpha1.ConsensusCapsule) int32 {
	if capsule.Spec.GrpcPort != 0 {
		return capsule.Spec.GrpcPort
	}
	return GrpcPort
}

// Volume types
const (
	VolumeTypeLogs    = "logs"
	VolumeTypeStats   = "stats"
	VolumeTypeSaved   = "saved"
	VolumeTypeState   = "state"
	VolumeTypeUpgrade = "upgrade"
	VolumeTypeBlocks  = "blocks"
	VolumeTypeRecords = "records"
	VolumeTypeEvents  = "events"
)

// Stream mount paths — single source of truth shared by volume builders and getVolumeNameForPath.
const (
	PathBlockStreams  = "/opt/hgcapp/blockStreams"
	PathRecordStreams = "/opt/hgcapp/recordStreams"
	PathEventsStreams = "/opt/hgcapp/eventsStreams"
)

// CommonLabels returns the common labels for consensus capsule resources
func CommonLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	return map[string]string{
		LabelManagedBy: ManagedByValue,
		LabelPartOf:    PartOfConsensusNode,
		LabelOrbit:     capsule.Spec.Orbit,
		LabelNodeId:    fmt.Sprintf("%d", capsule.Spec.NodeId),
	}
}

// SelectorLabels returns the minimal stable labels used for StatefulSet MatchLabels and pod listing.
// These must never include mutable fields (e.g. AccountId) as StatefulSet selectors are immutable.
func SelectorLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	return CommonLabels(capsule)
}

// PodLabels returns the full label set for pod template metadata.
func PodLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	labels := SelectorLabels(capsule)
	labels[LabelComponent] = ComponentConsensusNode
	labels[LabelSoloNodeName] = capsule.Name
	labels[LabelSoloAccountId] = capsule.Spec.AccountId
	labels[LabelSoloType] = "network-node"
	return labels
}

// ServiceAccountName returns the service account name for a capsule
func ServiceAccountName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + ServiceAccountSuffix
}

// RoleBindingName returns the role binding name for a capsule
func RoleBindingName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + RoleBindingSuffix
}

// HapiAppConfigMapName returns the hapi-app ConfigMap name for a capsule.
// This ConfigMap contains log4j2.xml (from the capsule spec) and settings.txt (from the Orbit).
func HapiAppConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-hapi-app-cm"
}

// DataConfigMapName returns the data-config ConfigMap name for a capsule.
// This ConfigMap contains application.properties and api-permission.properties sourced from the Orbit.
func DataConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-data-config-cm"
}

// BlockNodesConfigMapName returns the block-nodes ConfigMap name for a capsule.
// Only created when capsule.Spec.BlockNodes is non-nil.
func BlockNodesConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-block-nodes-cm"
}

// PVCName returns the PVC name for a capsule and volume type
func PVCName(capsule *operatorv1alpha1.ConsensusCapsule, volumeType string) string {
	return fmt.Sprintf("%s-%s", capsule.Name, volumeType)
}

// StatefulSetName returns the StatefulSet name for a capsule.
func StatefulSetName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name
}

// RenderSettingsTxt renders a settings map in the padded-comma format expected by the Hedera platform:
//
//	"checkSignedStateFromDisk,              1"
//
// Keys are sorted for deterministic output so ConfigMap content is stable across reconciles.
func RenderSettingsTxt(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%-46s %s\n", k+",", m[k])
	}
	return sb.String()
}

// RenderProperties renders a map[string]string as a sorted Java-properties-style key=value file.
func RenderProperties(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s=%s\n", k, m[k])
	}
	return sb.String()
}

// InternalHostname returns the pod FQDN for a capsule, or InternalAddress if set.
func InternalHostname(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return common.InternalHostname(capsule, common.ConsensusSubdomain)
}

// NewSubReconcilers creates all consensus capsule subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme) *subreconciler.Chain[*operatorv1alpha1.ConsensusCapsule] {
	return subreconciler.NewChain(
		NewServiceAccountSubReconciler(c, scheme),
		NewRoleBindingSubReconciler(c, scheme),
		NewHapiAppConfigMapSubReconciler(c, scheme),
		NewDataConfigMapSubReconciler(c, scheme),
		NewBlockNodesConfigMapSubReconciler(c, scheme),
		NewCheetahConfigMapSubReconciler(c, scheme),
		NewAlloyConfigMapSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewPVCSubReconciler(c, scheme),
		NewStatefulSetSubReconciler(c, scheme),
	)
}
