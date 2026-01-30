// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkOperationMode represents the type of network operation
// +kubebuilder:validation:Enum=FirstInitialization;NetworkGenesis;NetworkCheck;NetworkUpgrade;NodeAdd;NodeDelete;NodeUpdate;ProxyUpgrade
type NetworkOperationMode string

const (
	// NetworkOperationModeFirstInitialization is deprecated, use NetworkGenesis
	NetworkOperationModeFirstInitialization NetworkOperationMode = "FirstInitialization"
	// NetworkOperationModeNetworkGenesis initializes a new network
	NetworkOperationModeNetworkGenesis NetworkOperationMode = "NetworkGenesis"
	// NetworkOperationModeNetworkCheck verifies network state
	NetworkOperationModeNetworkCheck NetworkOperationMode = "NetworkCheck"
	// NetworkOperationModeNetworkUpgrade upgrades the network
	NetworkOperationModeNetworkUpgrade NetworkOperationMode = "NetworkUpgrade"
	// NetworkOperationModeNodeAdd adds a node to the network
	NetworkOperationModeNodeAdd NetworkOperationMode = "NodeAdd"
	// NetworkOperationModeNodeDelete removes a node from the network
	NetworkOperationModeNodeDelete NetworkOperationMode = "NodeDelete"
	// NetworkOperationModeNodeUpdate updates a node
	NetworkOperationModeNodeUpdate NetworkOperationMode = "NodeUpdate"
	// NetworkOperationModeProxyUpgrade upgrades proxy components
	NetworkOperationModeProxyUpgrade NetworkOperationMode = "ProxyUpgrade"
)

// NetworkUpgradeWorkflowStep represents the current step in an upgrade workflow
// +kubebuilder:validation:Enum=ExecuteImmediate;FreezeScheduled;NowFrozen;FreezeAbort
type NetworkUpgradeWorkflowStep string

const (
	// ExecuteImmediate performs immediate validation and setup
	NetworkUpgradeWorkflowStepExecuteImmediate NetworkUpgradeWorkflowStep = "ExecuteImmediate"
	// FreezeScheduled is when freeze is scheduled
	NetworkUpgradeWorkflowStepFreezeScheduled NetworkUpgradeWorkflowStep = "FreezeScheduled"
	// NowFrozen is when the network is frozen and ready for upgrade
	NetworkUpgradeWorkflowStepNowFrozen NetworkUpgradeWorkflowStep = "NowFrozen"
	// FreezeAbort aborts the freeze
	NetworkUpgradeWorkflowStepFreezeAbort NetworkUpgradeWorkflowStep = "FreezeAbort"
)

// NetworkOperationSpec defines the desired state of NetworkOperation
type NetworkOperationSpec struct {
	// Orbit is the name of the orbit to operate on
	Orbit string `json:"orbit"`

	// Mode specifies the type of network operation
	Mode NetworkOperationMode `json:"mode"`

	// NetworkUpgrade contains upgrade-specific configuration
	// +optional
	NetworkUpgrade *NetworkUpgradeSpec `json:"networkUpgrade,omitempty"`

	// ProxyUpgrade contains proxy upgrade configuration
	// +optional
	ProxyUpgrade *ProxyUpgradeSpec `json:"proxyUpgrade,omitempty"`

	// NodeOperation contains node operation configuration
	// +optional
	NodeOperation *NodeOperationSpec `json:"nodeOperation,omitempty"`
}

// NodeOperationSpec defines node operation configuration
type NodeOperationSpec struct {
	// NodeAdd contains configuration for adding a node
	// +optional
	NodeAdd *NodeAddSpec `json:"nodeAdd,omitempty"`

	// NodeUpdate contains configuration for updating a node
	// +optional
	NodeUpdate *NodeUpdateSpec `json:"nodeUpdate,omitempty"`

	// NodeDelete contains configuration for deleting a node
	// +optional
	NodeDelete *NodeDeleteSpec `json:"nodeDelete,omitempty"`
}

// NodeAddSpec defines configuration for adding a new node
type NodeAddSpec struct {
	// NodeName is the name of the node to add
	NodeName string `json:"nodeName"`

	// NodeID is the unique ID for the new node
	NodeID int64 `json:"nodeId"`

	// AccountID is the account ID for the node
	AccountID string `json:"accountId"`

	// Endpoints are the network endpoints for the node
	// +optional
	Endpoints []NodeEndpoint `json:"endpoints,omitempty"`
}

// NodeUpdateSpec defines configuration for updating a node
type NodeUpdateSpec struct {
	// NodeName is the name of the node to update
	NodeName string `json:"nodeName"`

	// AccountID is the new account ID if changing
	// +optional
	AccountID string `json:"accountId,omitempty"`

	// Endpoints are the new endpoints if changing
	// +optional
	Endpoints []NodeEndpoint `json:"endpoints,omitempty"`
}

// NodeDeleteSpec defines configuration for deleting a node
type NodeDeleteSpec struct {
	// NodeName is the name of the node to delete
	NodeName string `json:"nodeName"`

	// NodeID is the ID of the node to delete
	NodeID int64 `json:"nodeId"`
}

// NodeEndpoint represents a network endpoint for a node
type NodeEndpoint struct {
	// Host is the hostname or IP address
	Host string `json:"host"`

	// Port is the port number
	Port int32 `json:"port"`

	// Type is the endpoint type (gossip, grpc, etc.)
	// +optional
	Type string `json:"type,omitempty"`
}

// NetworkUpgradeSpec defines upgrade operation configuration
type NetworkUpgradeSpec struct {
	// Changesets contains the changes to apply
	Changesets NetworkUpgradeChangesets `json:"changesets"`

	// WorkflowStep is the current step in the upgrade workflow
	WorkflowStep NetworkUpgradeWorkflowStep `json:"workflowStep"`

	// ConfigurationFiles contains configuration file changes
	// +optional
	ConfigurationFiles *NetworkUpgradeConfigFiles `json:"configurationFiles,omitempty"`

	// AddressBook is the updated address book
	// +optional
	AddressBook []AddressBookEntry `json:"addressBook,omitempty"`
}

// NetworkUpgradeChangesets contains all changesets for an upgrade
type NetworkUpgradeChangesets struct {
	// ConsensusNodeChangeset for consensus nodes
	ConsensusNodeChangeset ConsensusNodeChangeset `json:"consensusNodeChangeset"`

	// RecordStreamUploaderChangeset for record stream uploader
	// +optional
	RecordStreamUploaderChangeset *StreamUploaderChangeset `json:"recordStreamUploaderChangeset,omitempty"`

	// RecordStreamSidecarUploaderChangeset for sidecar uploader
	// +optional
	RecordStreamSidecarUploaderChangeset *StreamUploaderChangeset `json:"recordStreamSidecarUploaderChangeset,omitempty"`

	// EventsStreamUploaderChangeset for events uploader
	// +optional
	EventsStreamUploaderChangeset *StreamUploaderChangeset `json:"eventsStreamUploaderChangeset,omitempty"`

	// BackupUploaderChangeset for backup uploader
	// +optional
	BackupUploaderChangeset *StreamUploaderChangeset `json:"backupUploaderChangeset,omitempty"`

	// OtelCollector changeset
	// +optional
	OtelCollector *OtelCollectorChangeset `json:"otelCollector,omitempty"`
}

// OtelCollectorChangeset contains changes for otel collector
type OtelCollectorChangeset struct {
	// SoftwareVersionSource specifies the image source
	SoftwareVersionSource SoftwareVersionSource `json:"softwareVersionSource"`
}

// NetworkUpgradeConfigFiles contains configuration file changes
type NetworkUpgradeConfigFiles struct {
	// NetworkSettings (settings.txt)
	// +optional
	NetworkSettings map[string]string `json:"networkSettings,omitempty"`

	// ApplicationProperties (application.properties)
	// +optional
	ApplicationProperties map[string]string `json:"applicationProperties,omitempty"`

	// PermissionConfiguration (api-permissions.properties)
	// +optional
	PermissionConfiguration map[string]string `json:"permissionConfiguration,omitempty"`

	// GenesisProperties (genesis.properties)
	// +optional
	GenesisProperties map[string]string `json:"genesisProperties,omitempty"`

	// Logging (log4j2.xml)
	// +optional
	Logging string `json:"logging,omitempty"`
}

// ProxyUpgradeSpec defines proxy upgrade configuration
type ProxyUpgradeSpec struct {
	// Changesets for proxy upgrades
	// +optional
	Changesets *ProxyChangesets `json:"changesets,omitempty"`
}

// ProxyChangesets contains proxy change configurations
type ProxyChangesets struct {
	// HaproxyChangeset for HAProxy changes
	// +optional
	HaproxyChangeset *HaproxyChangeset `json:"haproxyChangeset,omitempty"`
}

// NetworkOperationStatus defines the observed state of NetworkOperation
type NetworkOperationStatus struct {
	// Phase is the current phase
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation observed by the operator
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConsensusCapsules lists capsules to initialize
	// +optional
	ConsensusCapsules []string `json:"consensusCapsules,omitempty"`

	// SequenceNumber is used for ordering reconciliation
	// +optional
	SequenceNumber int `json:"sequenceNumber,omitempty"`

	// SoftwareSourceStatus contains image validation status
	// +optional
	SoftwareSourceStatus *SoftwareSourceStatus `json:"softwareSourceStatus,omitempty"`

	// UpgradedNodes lists node IDs that have been upgraded
	// +optional
	UpgradedNodes []int64 `json:"upgradedNodes,omitempty"`
}

// SoftwareSourceStatus contains status for all software sources
type SoftwareSourceStatus struct {
	// ConsensusNode status for consensus node images
	// +optional
	ConsensusNode []ImageRepositoryStatus `json:"consensusNode,omitempty"`

	// RecordStreamUploader status
	// +optional
	RecordStreamUploader []ImageRepositoryStatus `json:"recordStreamUploader,omitempty"`

	// RecordStreamSidecarUploader status
	// +optional
	RecordStreamSidecarUploader []ImageRepositoryStatus `json:"recordStreamSidecarUploader,omitempty"`

	// EventsStreamUploader status
	// +optional
	EventsStreamUploader []ImageRepositoryStatus `json:"eventsStreamUploader,omitempty"`

	// BackupUploader status
	// +optional
	BackupUploader []ImageRepositoryStatus `json:"backupUploader,omitempty"`

	// OtelCollector status
	// +optional
	OtelCollector []ImageRepositoryStatus `json:"otelCollector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=netop
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkOperation is the Schema for the networkoperations API
type NetworkOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkOperationSpec   `json:"spec,omitempty"`
	Status NetworkOperationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkOperationList contains a list of NetworkOperation
type NetworkOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkOperation{}, &NetworkOperationList{})
}
