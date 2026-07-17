// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
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

	// LedgerId is the ledger identifier (swirld entry)
	// +kubebuilder:validation:MinLength=1
	LedgerId string `json:"ledgerId"`

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
