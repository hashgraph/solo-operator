// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
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
	RoleSuffix           = "-role"
	RoleBindingSuffix    = "-rb"
	ConfigMapSuffix      = "-cm"
)

// Resource names. The genesis package re-exports the ConfigMap-related
// constants from this block (single source of truth lives here so deployment.go
// can mount them without importing the genesis package, which would create a
// cycle since genesis imports consensus for hostname helpers).
const (
	// ClusterConfigMapName is the legacy cluster-config ConfigMap created by NetworkGenesis.
	ClusterConfigMapName = "cluster-config"
	// GenesisConfigMapName holds genesis-network.json — mounted by ConsensusCapsule pods.
	GenesisConfigMapName = "genesis-config"
	// GenesisNetworkFileName is the key inside GenesisConfigMapName.
	GenesisNetworkFileName = "genesis-network.json"
	// GenesisPropertiesFileName is the key inside data-config-cm for genesis.properties.
	GenesisPropertiesFileName = "genesis.properties"
	// PublicKeysConfigMapName holds all nodes' signing public certs — mounted into data/keys.
	PublicKeysConfigMapName = "public-keys"
)

// Checkpoint ConfigMap constants — [capsule-name]-checkpoint.
const (
	CheckpointKeyGenesisInitialized = "genesis-initialized"
	// Keep these in sync with the shell-script comparison in buildGenesisInitContainer.
	CheckpointValueTrue  = "true"
	CheckpointValueFalse = "false"
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
// Kept independent from CommonLabels so additions to CommonLabels never corrupt the immutable selector.
func SelectorLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	return map[string]string{
		LabelOrbit:  capsule.Spec.Orbit,
		LabelNodeId: fmt.Sprintf("%d", capsule.Spec.NodeId),
	}
}

// PodLabels returns the full label set for pod template metadata.
// LabelManagedBy + LabelPartOf are required for the consensus headless service selector to match.
func PodLabels(capsule *operatorv1alpha1.ConsensusCapsule) map[string]string {
	labels := SelectorLabels(capsule)
	labels[LabelManagedBy] = ManagedByValue
	labels[LabelPartOf] = PartOfConsensusNode
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

// RoleName returns the per-capsule Role name.
func RoleName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + RoleSuffix
}

// RoleBindingName returns the role binding name for a capsule
func RoleBindingName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + RoleBindingSuffix
}

// DataConfigMapName returns the genesis-properties ConfigMap name for a capsule.
// This CM holds genesis.properties rendered from Orbit.Spec.Consensus.GenesisProperties.
func DataConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-genesis-properties-cm"
}

// BlockNodesConfigMapName returns the block-nodes ConfigMap name for a capsule.
func BlockNodesConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-block-nodes-cm"
}

// CheckpointConfigMapName returns the checkpoint ConfigMap name for a capsule.
func CheckpointConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-checkpoint"
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

// DefaultApplicationProperties returns the baseline application.properties values required by Solo.
func DefaultApplicationProperties() map[string]string {
	return map[string]string{
		"hedera.config.version":                       "0",
		"netty.mode":                                  "TEST",
		"hedera.recordStream.logPeriod":               "1",
		"balances.exportPeriodSecs":                   "400",
		"files.maxSizeKb":                             "2048",
		"hedera.recordStream.compressFilesOnCreation": "true",
		"balances.compressOnCreation":                 "true",
		"contracts.maxNumWithHapiSigsAccess":          "0",
		"autoRenew.targetTypes":                       "",
		// FQDNs are required for gossip in Kubernetes — the CN default (true) breaks genesis.
		"nodes.gossipFqdnRestricted":          "false",
		"hedera.profiles.active":              "TEST",
		"staking.periodMins":                  "1",
		"nodes.updateAccountIdAllowed":        "true",
		"blockStream.streamMode":              "RECORDS",
		"scheduling.longTermEnabled":          "false",
		"bootstrap.throttleDefsJson.resource": "data/config/genesis-throttles.json",
		"cache.warmThreads":                   "6",
	}
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

// InternalHostname returns the per-capsule service FQDN for a capsule, or InternalAddress if set.
func InternalHostname(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return common.InternalHostname(capsule)
}

// NewSubReconcilers creates all consensus capsule subreconcilers in execution order
func NewSubReconcilers(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *subreconciler.Chain[*operatorv1alpha1.ConsensusCapsule] {
	return subreconciler.NewChain(
		NewServiceAccountSubReconciler(c, scheme),
		NewRoleSubReconciler(c, scheme),
		NewRoleBindingSubReconciler(c, scheme),
		NewDataConfigMapSubReconciler(c, scheme),
		NewBlockNodesConfigMapSubReconciler(c, scheme),
		NewCheetahConfigMapSubReconciler(c, scheme),
		NewAlloyConfigMapSubReconciler(c, scheme),
		NewServiceSubReconciler(c, scheme),
		NewPVCSubReconciler(c, scheme),
		// Checkpoint must precede StatefulSet so the ConfigMap exists before the
		// init container tries to resolve the GENESIS_INITIALIZED env var.
		NewCheckpointSubReconciler(c, scheme),
		NewStatefulSetSubReconciler(c, scheme, recorder),
	)
}
