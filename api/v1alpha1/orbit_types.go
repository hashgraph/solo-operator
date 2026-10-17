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

	// RequireDigestOnDeploy, when true, requires every ConsensusCapsule in this Orbit to run
	// only digest-pinned images on deploy: the consensus node and every enabled sidecar/
	// uploader must set softwareVersion.digest (an immutable "sha256:..." content digest).
	// Any image that is not explicitly digest-pinned is rejected — including a tag-only
	// softwareVersion and an enabled container that would fall back to an operator-managed
	// default image (relying on a default is not permitted under this policy). This is the
	// opt-in, fresh-deploy counterpart to the always-on digest enforcement of the network
	// upgrade flow; use it to guarantee digest-pinned images from day one rather than only
	// after the first upgrade. Default false (tag-only images are allowed on deploy).
	// +optional
	RequireDigestOnDeploy bool `json:"requireDigestOnDeploy,omitempty"`

	// ProvisionerDaemonEnabled indicates that a host-level provisioner daemon
	// (solo-provisioner) runs on bare-metal nodes rather than inside Kubernetes.
	// When true the consensus UC sidecar runs as UC_MODE=mainnet (defers the execute
	// phase to the host daemon); when false (default) the UC runs as UC_MODE=cluster-only.
	// This is a host-level, cross-component infra fact, so it is network-wide config.
	// +kubebuilder:default=false
	// +optional
	ProvisionerDaemonEnabled bool `json:"provisionerDaemonEnabled,omitempty"`

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
	// Default: 30s. Values below 1s are ignored and the default is used.
	// +kubebuilder:validation:Format=duration
	// +optional
	PrereqRequeueInterval *metav1.Duration `json:"prereqRequeueInterval,omitempty"`

	// DaemonRequeueInterval is how often the execute reconciler polls for the
	// provisioner daemon result condition. Default: 10s. Values below 1s are
	// ignored and the default is used.
	// +kubebuilder:validation:Format=duration
	// +optional
	DaemonRequeueInterval *metav1.Duration `json:"daemonRequeueInterval,omitempty"`

	// NodeReadyRequeueInterval is how often the execute reconciler polls for
	// the consensus node to reach ACTIVE after the pod is brought online.
	// Default: 10s. Values below 1s are ignored and the default is used.
	// +kubebuilder:validation:Format=duration
	// +optional
	NodeReadyRequeueInterval *metav1.Duration `json:"nodeReadyRequeueInterval,omitempty"`

	// Prefetch configures best-effort image prefetch during the prepare phase.
	// +optional
	Prefetch *PrefetchConfig `json:"prefetch,omitempty"`

	// SkipScaleDown controls how the execute phase restarts the consensus pod
	// onto the new image. When true (default) the operator patches the
	// ConsensusCapsule images in-place and lets the StatefulSet RollingUpdate
	// strategy restart the pod, skipping the scale-down-to-0 / scale-up-to-1
	// sequence and saving ~10s. When false it uses the scale-down sequence.
	// Applies to both cluster-only and mainnet (provisioner-daemon) modes: a
	// config-only upgrade behaves the same in either, because the config step is
	// pod-state-agnostic. The operator forces the scale-down path regardless of
	// this flag when the upgrade requires host-level infrastructure work (the pod
	// must be fully absent before the provisioner daemon can proceed).
	// +kubebuilder:default=true
	// +optional
	SkipScaleDown *bool `json:"skipScaleDown,omitempty"`
}

// PrefetchConfig configures best-effort image prefetch during the prepare phase.
type PrefetchConfig struct {
	// Enabled turns image prefetch on or off. Default: true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// JobTTL is how long completed prefetch Jobs linger before Kubernetes
	// auto-deletes them via the TTL controller. Default: 90d.
	// Caution: this TTL applies to both succeeded and failed Jobs. Setting
	// a short duration causes failed Jobs to be deleted, losing debug info
	// (logs, events) needed to diagnose prefetch failures.
	// +kubebuilder:validation:Format=duration
	// +optional
	JobTTL *metav1.Duration `json:"jobTTL,omitempty"`

	// MaxNodes is the maximum number of nodes matching capsule nodeSelectors for
	// which the operator creates node-pinned prefetch Jobs (selector coverage).
	// When the matched pool exceeds this threshold, the operator falls back to
	// pinning one Job per node currently running a consensus pod. Default: 30.
	// Set to 0 to always use pod-node targeting.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxNodes *int32 `json:"maxNodes,omitempty"`
}

const (
	DefaultExecutePrereqRequeueInterval    = 30 * time.Second
	DefaultExecuteDaemonRequeueInterval    = 10 * time.Second
	DefaultExecuteNodeReadyRequeueInterval = 10 * time.Second
	DefaultPrefetchEnabled                 = true
	DefaultPrefetchJobTTL                  = 90 * 24 * time.Hour
	DefaultMaxPrefetchNodes                = int32(30)
	MinRequeueInterval                     = 1 * time.Second
)

// EffectiveUpgradeConfig holds the resolved upgrade tuning knobs for use by the
// execute reconciler: Orbit spec value when set, otherwise the hardcoded default.
type EffectiveUpgradeConfig struct {
	PrereqRequeueInterval    time.Duration
	DaemonRequeueInterval    time.Duration
	NodeReadyRequeueInterval time.Duration
	PrefetchEnabled          bool
	PrefetchJobTTL           time.Duration
	MaxPrefetchNodes         int32
	SkipScaleDown            bool
}

// ResolveUpgradeConfig returns the effective upgrade tuning values from an Orbit,
// falling back to production-safe defaults for any field not set.
func ResolveUpgradeConfig(orbit *Orbit) EffectiveUpgradeConfig {
	cfg := EffectiveUpgradeConfig{
		PrereqRequeueInterval:    DefaultExecutePrereqRequeueInterval,
		DaemonRequeueInterval:    DefaultExecuteDaemonRequeueInterval,
		NodeReadyRequeueInterval: DefaultExecuteNodeReadyRequeueInterval,
		PrefetchEnabled:          DefaultPrefetchEnabled,
		PrefetchJobTTL:           DefaultPrefetchJobTTL,
		MaxPrefetchNodes:         DefaultMaxPrefetchNodes,
		SkipScaleDown:            true,
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
	if u.Prefetch != nil {
		if u.Prefetch.Enabled != nil {
			cfg.PrefetchEnabled = *u.Prefetch.Enabled
		}
		if u.Prefetch.JobTTL != nil && u.Prefetch.JobTTL.Duration >= 0 {
			cfg.PrefetchJobTTL = u.Prefetch.JobTTL.Duration
		}
		if u.Prefetch.MaxNodes != nil && *u.Prefetch.MaxNodes >= 0 {
			cfg.MaxPrefetchNodes = *u.Prefetch.MaxNodes
		}
	}
	if u.SkipScaleDown != nil {
		cfg.SkipScaleDown = *u.SkipScaleDown
	}
	return cfg
}

// OrbitConsensus contains shared consensus configuration
type OrbitConsensus struct {
	// ProvisioningModel specifies how consensus nodes are deployed
	// +kubebuilder:validation:Enum=InitContainerDinD;InitContainerJava;DeterministicContainerImage
	// +optional
	ProvisioningModel string `json:"provisioningModel,omitempty"`

	// FeatureGates holds transitional, rollout-only consensus feature gates. See
	// ConsensusFeatureGates.
	// +optional
	FeatureGates *ConsensusFeatureGates `json:"featureGates,omitempty"`

	// CanAdoptRosterFromVersion is the first consensus-node package version whose
	// candidate roster the consensus node is expected to adopt. Version means
	// images.consensusNode.version in the upgrade package's consensus-node-components.yaml,
	// compared as a semantic version. It is set once and never needs updating per upgrade.
	//
	// For an upgrade to a version at or above this one, the operator acts on the roster: it
	// swaps the node's signing key to the staged Secret that matches the roster's
	// certificate, computes the firewall plan and applies it. For any lower version, or when
	// this is empty (the default), the operator only logs a warning when the roster changes
	// this node or the firewall, keeps the old key and changes no firewall rules.
	//
	// It mirrors the CN's own canAdoptRoster check (TSS readiness). Set it only for versions
	// where the candidate will be adopted: the TSS gate is off (both tss.hintsEnabled and
	// tss.historyEnabled are false, or the network has no TSS ledger id yet), or TSS
	// readiness is always complete before the council freezes the network. Do not derive it
	// from genesis.addressBook.ledgerId, which is the HAPI ledger id, not the TSS one.
	//
	// This is configuration, not a feature gate: it describes the deployed network and is
	// not removed on a schedule. Whoever enables roster automation (solo-provisioner or an
	// upgrade job) sets it; the operator only reads it. Because it holds for every later
	// upgrade, it cannot notice a single upgrade whose candidate the CN refuses: the UC's
	// roster-adoption result is the safety net, and a wrong setting leaves a node on a new
	// key against a roster the CN refused. To switch automation off, clear it or raise it.
	// +kubebuilder:validation:Pattern=`^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`
	// +kubebuilder:validation:MaxLength=64
	// +optional
	CanAdoptRosterFromVersion string `json:"canAdoptRosterFromVersion,omitempty"`

	// Genesis contains genesis-specific configuration
	Genesis OrbitGenesis `json:"genesis"`
}

// ConsensusFeatureGates holds transitional, rollout-only feature gates for consensus
// behavior. Unlike configuration (permanent desired state), each gate is removed once
// its behavior is everywhere. Convention: every gate's doc comment states its default
// and when to remove it.
type ConsensusFeatureGates struct {
	// NodeNameZeroBased switches gossip key-file naming node<id+1> → node<id>.
	// Default false. Remove once 0-based naming is the only convention the CN supports.
	// Set to true only with a CN version that loads 0-based names — solo-provisioner
	// sets it to match the deployed CN; the operator only honors it.
	// +kubebuilder:default=false
	// +optional
	NodeNameZeroBased bool `json:"nodeNameZeroBased,omitempty"`
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

// OrbitConditionTypeNamespaceAllowed reports whether the Orbit's namespace is in the operator's allowedNamespaces.
const OrbitConditionTypeNamespaceAllowed = "NamespaceAllowed"

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

// IsNodeNameZeroBased resolves the consensus nodeNameZeroBased feature gate,
// defaulting to false (node<id+1>) when the gate is unset. The operator only honors
// the value; solo-provisioner sets it to match the deployed CN.
func (o *Orbit) IsNodeNameZeroBased() bool {
	if fg := o.Spec.Consensus.FeatureGates; fg != nil {
		return fg.NodeNameZeroBased
	}
	return false
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
