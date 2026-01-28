// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
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

// OrbitTelemetry contains telemetry configuration for the Orbit
type OrbitTelemetry struct {
	// Metrics configuration
	// +optional
	Metrics *OrbitMetrics `json:"metrics,omitempty"`

	// Logging configuration
	// +optional
	Logging *OrbitLogging `json:"logging,omitempty"`
}

// OrbitMetrics contains metrics configuration
type OrbitMetrics struct {
	// Enabled enables metrics collection
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Interval specifies the collection interval (e.g., "5s")
	// +kubebuilder:default="5s"
	// +optional
	Interval string `json:"interval,omitempty"`

	// Labels to attach to metrics
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Port on which metrics are exposed
	// +kubebuilder:default=9998
	// +optional
	Port int `json:"port,omitempty"`

	// RemoteWrite enables remote write
	// +kubebuilder:default=false
	// +optional
	RemoteWrite bool `json:"remoteWrite,omitempty"`
}

// OrbitLogging contains logging configuration
type OrbitLogging struct {
	// Enabled enables logging
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Labels to attach to logs
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// OrbitConsensus contains shared consensus configuration
type OrbitConsensus struct {
	// ProvisioningModel specifies how consensus nodes are deployed
	// +kubebuilder:validation:Enum=InitContainerDinD;InitContainerJava;DeterministicContainerImage
	// +optional
	ProvisioningModel string `json:"provisioningModel,omitempty"`

	// NetworkSettings is equivalent to settings.txt
	NetworkSettings map[string]string `json:"networkSettings"`

	// ApplicationConfiguration is equivalent to application.properties
	ApplicationConfiguration map[string]string `json:"applicationConfiguration"`

	// PermissionConfiguration is equivalent to api-permission.properties
	PermissionConfiguration map[string]string `json:"permissionConfiguration"`

	// GenesisProperties is equivalent to genesis.properties
	// +optional
	GenesisProperties map[string]string `json:"genesisProperties,omitempty"`

	// Genesis contains genesis-specific configuration
	// +optional
	Genesis *OrbitGenesis `json:"genesis,omitempty"`
}

// OrbitGenesis contains genesis initialization configuration
type OrbitGenesis struct {
	// AddressBook contains the address book configuration
	// +optional
	AddressBook *OrbitAddressBook `json:"addressBook,omitempty"`
}

// OrbitAddressBook contains address book configuration
type OrbitAddressBook struct {
	// RealmId is the numeric realm ID
	// +kubebuilder:default=0
	// +optional
	RealmId int `json:"realmId,omitempty"`

	// ShardId is the numeric shard ID
	// +kubebuilder:default=0
	// +optional
	ShardId int `json:"shardId,omitempty"`

	// LedgerId is the ledger identifier (swirld entry)
	// +optional
	LedgerId string `json:"ledgerId,omitempty"`

	// ExternalAddresses lists external nodes not managed by this operator
	// +optional
	ExternalAddresses []AddressBookEntry `json:"externalAddresses,omitempty"`
}

// OrbitStatus defines the observed state of Orbit
type OrbitStatus struct {
	// Phase is the current phase of the Orbit
	// +optional
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
