// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrbitSpec defines the desired state of an Orbit
type OrbitSpec struct {
	// DeploymentModel specifies whether this is a single or multi-cluster deployment
	// +kubebuilder:validation:Enum=SingleKubernetesCluster;MultipleKubernetesClusters
	// +optional
	DeploymentModel string `json:"deploymentModel,omitempty"`

	// Lifetime specifies the duration for this Orbit (e.g., "P2DT15H" or "2 days 15 hours")
	// +optional
	Lifetime string `json:"lifetime,omitempty"`

	// Finalizers is a list of finalizers to attach to all resources in this Orbit
	// +optional
	Finalizers []string `json:"finalizers,omitempty"`

	// Telemetry contains telemetry configuration
	// +optional
	Telemetry *OrbitTelemetry `json:"telemetry,omitempty"`

	// Upgrade contains tuning knobs for the network upgrade lifecycle,
	// scoped per component type.
	// +optional
	Upgrade *OrbitUpgrade `json:"upgrade,omitempty"`

	// Consensus contains shared configuration for all ConsensusCapsules
	Consensus OrbitConsensus `json:"consensus"`
}

// OrbitTelemetry contains telemetry configuration for the Orbit.
// Alloy is the single telemetry sidecar that handles both logging (Loki) and metrics (Prometheus).
type OrbitTelemetry struct {
	// Alloy contains default Grafana Alloy telemetry sidecar configuration
	// applied to all ConsensusCapsules in this orbit. Capsule-level settings override these defaults.
	// +optional
	Alloy *OrbitAlloyDefaults `json:"alloy,omitempty"`
}

// OrbitAlloyDefaults provides default Alloy sidecar configuration for all ConsensusCapsules in the orbit.
// Individual capsules can override any of these fields via their own alloy spec.
type OrbitAlloyDefaults struct {
	// Enabled enables Alloy sidecar for all capsules in this orbit by default
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SoftwareVersion is the default Alloy container image spec
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// Resources is the default resource requirements for the Alloy sidecar
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Config contains default Alloy collector configuration (exporters, processors, etc.)
	// +optional
	Config *AlloyCollectorConfig `json:"config,omitempty"`
}

// OrbitUpgrade contains per-component upgrade tuning knobs.
type OrbitUpgrade struct {
	// ConsensusNode contains tuning knobs for the consensus node upgrade
	// execute phase. All fields are optional; when unset the operator uses
	// safe production defaults.
	// +optional
	ConsensusNode *ConsensusNodeUpgrade `json:"consensusNode,omitempty"`
}

// ConsensusNodeUpgrade contains tuning knobs for the consensus node upgrade
// execute phase.
type ConsensusNodeUpgrade struct {
	// PrereqRequeueInterval is how often the execute reconciler polls for
	// prerequisite CRs (Prepare/Freeze succeeded, ConfigCRsApplied).
	// Default: 30s.
	// +kubebuilder:validation:Format=duration
	// +optional
	PrereqRequeueInterval *metav1.Duration `json:"prereqRequeueInterval,omitempty"`

	// DaemonRequeueInterval is how often the execute reconciler polls for the
	// provisioner daemon result condition. Default: 10s.
	// +kubebuilder:validation:Format=duration
	// +optional
	DaemonRequeueInterval *metav1.Duration `json:"daemonRequeueInterval,omitempty"`

	// NodeReadyRequeueInterval is how often the execute reconciler polls for
	// the consensus node to reach ACTIVE after the pod is brought online.
	// Default: 10s.
	// +kubebuilder:validation:Format=duration
	// +optional
	NodeReadyRequeueInterval *metav1.Duration `json:"nodeReadyRequeueInterval,omitempty"`
}

const (
	DefaultExecutePrereqRequeueInterval    = 30 * time.Second
	DefaultExecuteDaemonRequeueInterval    = 10 * time.Second
	DefaultExecuteNodeReadyRequeueInterval = 10 * time.Second
	MinRequeueInterval                     = 1 * time.Second
)

// EffectiveUpgradeConfig holds the resolved upgrade tuning knobs for use by the
// execute reconciler: Orbit spec value when set, otherwise the hardcoded default.
type EffectiveUpgradeConfig struct {
	PrereqRequeueInterval    time.Duration
	DaemonRequeueInterval    time.Duration
	NodeReadyRequeueInterval time.Duration
}

// ResolveUpgradeConfig returns the effective upgrade tuning values from an Orbit,
// falling back to production-safe defaults for any field not set.
func ResolveUpgradeConfig(orbit *Orbit) EffectiveUpgradeConfig {
	cfg := EffectiveUpgradeConfig{
		PrereqRequeueInterval:    DefaultExecutePrereqRequeueInterval,
		DaemonRequeueInterval:    DefaultExecuteDaemonRequeueInterval,
		NodeReadyRequeueInterval: DefaultExecuteNodeReadyRequeueInterval,
	}
	if orbit == nil || orbit.Spec.Upgrade == nil || orbit.Spec.Upgrade.ConsensusNode == nil {
		return cfg
	}
	u := orbit.Spec.Upgrade.ConsensusNode

	if u.PrereqRequeueInterval != nil && u.PrereqRequeueInterval.Duration >= MinRequeueInterval {
		cfg.PrereqRequeueInterval = u.PrereqRequeueInterval.Duration
	}
	if u.DaemonRequeueInterval != nil && u.DaemonRequeueInterval.Duration >= MinRequeueInterval {
		cfg.DaemonRequeueInterval = u.DaemonRequeueInterval.Duration
	}
	if u.NodeReadyRequeueInterval != nil && u.NodeReadyRequeueInterval.Duration >= MinRequeueInterval {
		cfg.NodeReadyRequeueInterval = u.NodeReadyRequeueInterval.Duration
	}
	return cfg
}

// OrbitConsensus contains shared consensus configuration
type OrbitConsensus struct {
	// ProvisioningModel specifies how consensus nodes are deployed
	// +kubebuilder:validation:Enum=InitContainerDinD;InitContainerJava;DeterministicContainerImage
	// +optional
	ProvisioningModel string `json:"provisioningModel,omitempty"`

	// ProvisionerDaemonEnabled indicates that a host-level provisioner daemon
	// (solo-provisioner) runs on bare-metal nodes rather than inside Kubernetes.
	// When true the UC sidecar runs as UC_MODE=mainnet (defers the execute phase
	// to the host daemon). When false (default) the UC runs as UC_MODE=cluster-only
	// and fulfils the provisioner role itself.
	// +kubebuilder:default=false
	// +optional
	ProvisionerDaemonEnabled bool `json:"provisionerDaemonEnabled,omitempty"`

	// Genesis contains genesis-specific configuration
	Genesis OrbitGenesis `json:"genesis"`
}

// OrbitGenesis contains genesis initialization configuration
type OrbitGenesis struct {
	// AddressBook contains the address book configuration
	AddressBook OrbitAddressBook `json:"addressBook"`
}

// OrbitAddressBook contains address book configuration
type OrbitAddressBook struct {
	// RealmId is the numeric realm ID
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	RealmId int `json:"realmId,omitempty"`

	// ShardId is the numeric shard ID
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	ShardId int `json:"shardId,omitempty"`

	// LedgerId is the network's ledger identity, written verbatim to the
	// consensus node's application.properties (and bootstrap.properties)
	// ledger.id. It is a 0x-prefixed hex byte: mainnet 0x00, testnet 0x01,
	// previewnet 0x02; local/dev networks use 0x01. This is distinct from
	// ChainId — do not put an EVM chain id here.
	//
	// NOTE: this static ledger.id is expected to be superseded by the TSS ledger
	// id (a public key) in a future consensus-node release; treat this field as
	// transitional and avoid building new coupling on its byte format.
	// +kubebuilder:validation:MinLength=1
	LedgerId string `json:"ledgerId"`

	// ChainId is the network's EVM chain id (EIP-155), written to the consensus
	// node's contracts.chainId and used as the JSON-RPC relay's CHAIN_ID. It is a
	// decimal number: mainnet 295, testnet 296, previewnet 297; local/dev 298.
	// This is distinct from LedgerId. Optional; when empty it is treated as "0".
	// +optional
	ChainId string `json:"chainId,omitempty"`

	// ExternalAddresses lists external nodes not managed by this operator
	// +optional
	ExternalAddresses []AddressBookEntry `json:"externalAddresses,omitempty"`
}

// OrbitStatus defines the observed state of Orbit
type OrbitStatus struct {
	// Phase is the current phase of the Orbit
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Unknown
	Phase Phase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// StartTime is when the Orbit was started
	// +optional
	StartTime string `json:"startTime,omitempty"`

	// ExpirationTime is when the Orbit will expire
	// +optional
	ExpirationTime string `json:"expirationTime,omitempty"`

	// ObservedGeneration is the generation observed by the operator
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=orbit
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Orbit is the Schema for the orbits API
type Orbit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrbitSpec   `json:"spec,omitempty"`
	Status OrbitStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrbitList contains a list of Orbit
type OrbitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Orbit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Orbit{}, &OrbitList{})
}
