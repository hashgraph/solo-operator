// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceType represents the Kubernetes service type
// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceTypeNodePort     ServiceType = "NodePort"
)

// ConsensusCapsuleSpec defines the desired state of ConsensusCapsule
type ConsensusCapsuleSpec struct {
	// Orbit is the name of the orbit this capsule belongs to
	Orbit string `json:"orbit"`

	// NodeId is the unique node identifier
	NodeId int64 `json:"nodeId"`

	// AccountId is the node account ID
	AccountId string `json:"accountId"`

	// Weight is the consensus weight
	Weight int `json:"weight"`

	// Log4j2XML is the logging configuration
	// +kubebuilder:validation:Required
	Log4j2XML string `json:"log4j2.xml"`

	// PodProperties contains pod-level configuration
	PodProperties ConsensusPodProperties `json:"podProperties"`

	// Secrets contains secret references for keys
	// +optional
	Secrets *ConsensusSecrets `json:"secrets,omitempty"`

	// InternalAddress is the internal FQDN or IP
	// +optional
	InternalAddress string `json:"internalAddress,omitempty"`

	// ExternalAddress is the external FQDN or IP
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`

	// NodeProperties is the node.properties configuration
	// +optional
	NodeProperties map[string]string `json:"nodeProperties,omitempty"`

	// ConfigFolder is a hostPath volume containing config.txt
	// +optional
	ConfigFolder *corev1.HostPathVolumeSource `json:"configFolder,omitempty"`

	// ServiceType specifies the consensus-node service type
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType ServiceType `json:"serviceType,omitempty"`

	// ServiceAnnotations are annotations for the service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// PersistentVolumeClaims contains PVC configurations
	// +optional
	PersistentVolumeClaims *ConsensusPVCConfig `json:"persistentVolumeClaims,omitempty"`
}

// ConsensusSecrets contains secret configuration for the capsule
type ConsensusSecrets struct {
	// HostPath references a hostPath volume containing keys
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`

	// SecretResources references Kubernetes secrets
	// +optional
	SecretResources *ConsensusSecretResources `json:"secretResources,omitempty"`
}

// ConsensusSecretResources references specific secret keys
type ConsensusSecretResources struct {
	// GrpcTlsPrivateKey references the gRPC TLS private key
	GrpcTlsPrivateKey KeyReference `json:"grpcTlsPrivateKey"`

	// GrpcTlsPublicCertificate references the gRPC TLS certificate
	GrpcTlsPublicCertificate KeyReference `json:"grpcTlsPublicCertificate"`

	// SigningPrivateKey references the signing private key
	SigningPrivateKey KeyReference `json:"signingPrivateKey"`

	// SigningPublicCertificate references the signing certificate
	SigningPublicCertificate KeyReference `json:"signingPublicCertificate"`

	// LedgerEncryptionKey references the ledger encryption key
	// +optional
	LedgerEncryptionKey *KeyReference `json:"ledgerEncryptionKey,omitempty"`

	// LedgerSigningKey references the ledger signing key
	// +optional
	LedgerSigningKey *KeyReference `json:"ledgerSigningKey,omitempty"`
}

// ConsensusPodProperties contains pod-level properties
type ConsensusPodProperties struct {
	// Containers contains container specifications
	Containers ConsensusContainers `json:"containers"`

	// Affinity specifies pod affinity rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// NodeSelector for pod scheduling
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints for pod distribution
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// ResourceClaims for dynamic resource allocation
	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`

	// PreemptionPolicy for pod preemption
	// +kubebuilder:validation:Enum=Never;PreemptLowerPriority
	// +optional
	PreemptionPolicy string `json:"preemptionPolicy,omitempty"`

	// Priority is the pod priority
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// PriorityClassName is the priority class name
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// ServiceAccountName for the pod
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// SecurityContext for the pod
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// TerminationGracePeriodSeconds for pod termination
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Volumes contains volume configurations
	// +optional
	Volumes *ConsensusPodVolumes `json:"volumes,omitempty"`
}

// ConsensusContainers contains container specifications
type ConsensusContainers struct {
	// ConsensusNode is the main consensus node container
	ConsensusNode ConsensusNodeContainer `json:"consensusNode"`

	// BlocksUploader is the Cheetah blocks uploader sidecar
	// +optional
	BlocksUploader *CheetahUploader `json:"blocksUploader,omitempty"`

	// RecordsUploader is the Cheetah records uploader sidecar (record streams + sidecar files)
	// +optional
	RecordsUploader *CheetahUploader `json:"recordsUploader,omitempty"`

	// EventsUploader is the Cheetah events uploader sidecar
	// +optional
	EventsUploader *CheetahUploader `json:"eventsUploader,omitempty"`

	// BackupUploader is the backup uploader sidecar (rclone-based)
	// +optional
	BackupUploader *StreamUploader `json:"backupUploader,omitempty"`

	// RecordStreamUploader is deprecated, use RecordsUploader
	// +optional
	RecordStreamUploader *StreamUploader `json:"recordStreamUploader,omitempty"`

	// RecordStreamSidecarFilesUploader is deprecated, use RecordsUploader
	// +optional
	RecordStreamSidecarFilesUploader *StreamUploader `json:"recordStreamSidecarFilesUploader,omitempty"`

	// EventsStreamUploader is deprecated, use EventsUploader
	// +optional
	EventsStreamUploader *StreamUploader `json:"eventsStreamUploader,omitempty"`

	// Alloy is the Grafana Alloy telemetry sidecar
	// +optional
	Alloy *AlloyConfig `json:"alloy,omitempty"`
}

// ConsensusNodeContainer is the main consensus node container
type ConsensusNodeContainer struct {
	// Name is the container name
	// +kubebuilder:default="consensus-node"
	// +optional
	Name string `json:"name,omitempty"`

	// Image is the container image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// SoftwareVersion specifies the image (deprecated, use Image instead)
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// AdditionalVolumeMounts for the container
	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// AlloyConfig is the Grafana Alloy telemetry sidecar container config
type AlloyConfig struct {
	// Name is the container name
	// +kubebuilder:default="alloy"
	// +optional
	Name string `json:"name,omitempty"`

	// Image is the container image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// SoftwareVersion specifies the image (deprecated, use Image instead)
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Enabled enables the Alloy sidecar
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Config contains Alloy configuration
	// +optional
	Config *AlloyCollectorConfig `json:"config,omitempty"`

	// AdditionalVolumeMounts for the container
	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// AlloyCollectorConfig contains Alloy telemetry configuration
type AlloyCollectorConfig struct {
	// AppName is the application name
	// +kubebuilder:default="hedera-node"
	// +optional
	AppName string `json:"appName,omitempty"`

	// Environment is the network environment
	// +kubebuilder:default="integration"
	// +optional
	Environment string `json:"environment,omitempty"`

	// LogLevel is the logging level
	// +kubebuilder:default="DEBUG"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// Processors contains processor configuration
	// +optional
	Processors *AlloyProcessors `json:"processors,omitempty"`

	// Exporters contains exporter configuration
	// +optional
	Exporters *AlloyExporters `json:"exporters,omitempty"`

	// Receivers contains receiver configuration
	// +optional
	Receivers *AlloyReceivers `json:"receivers,omitempty"`
}

// AlloyProcessors contains processor configuration
type AlloyProcessors struct {
	// MemoryLimiter configuration
	// +optional
	MemoryLimiter *AlloyMemoryLimiter `json:"memoryLimiter,omitempty"`
}

// AlloyMemoryLimiter contains memory limiter configuration
type AlloyMemoryLimiter struct {
	// LimitMib is the memory limit in MiB
	// +kubebuilder:default="400"
	// +optional
	LimitMib string `json:"limitMib,omitempty"`

	// SpikeLimitMib is the spike limit in MiB (20% of limit)
	// +kubebuilder:default="80"
	// +optional
	SpikeLimitMib string `json:"spikeLimitMib,omitempty"`
}

// AlloyExporters contains exporter configuration
type AlloyExporters struct {
	// Loki exporter configuration
	// +optional
	Loki *AlloyLokiExporter `json:"loki,omitempty"`
}

// AlloyLokiExporter contains Loki exporter configuration
type AlloyLokiExporter struct {
	// Enabled enables Loki logging
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Auth contains authentication configuration
	// +optional
	Auth *AlloyLokiAuth `json:"auth,omitempty"`

	// Endpoint is the Loki endpoint URL
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// AlloyLokiAuth contains Loki authentication configuration
type AlloyLokiAuth struct {
	// Enabled enables authentication
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Secret references the secret containing credentials
	// +optional
	Secret *corev1.LocalObjectReference `json:"secret,omitempty"`
}

// AlloyReceivers contains receiver configuration
type AlloyReceivers struct {
	// SwirldsVMap receiver configuration
	// +optional
	SwirldsVMap *AlloyReceiver `json:"swirldsVMap,omitempty"`

	// Hashstream receiver configuration
	// +optional
	Hashstream *AlloyReceiver `json:"hashstream,omitempty"`
}

// AlloyReceiver is a generic receiver configuration
type AlloyReceiver struct {
	// Enabled enables the receiver
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// ConsensusPodVolumes contains volume configurations
type ConsensusPodVolumes struct {
	// Logs volume
	// +optional
	Logs *corev1.Volume `json:"logs,omitempty"`

	// Stats volume
	// +optional
	Stats *corev1.Volume `json:"stats,omitempty"`

	// States volume
	// +optional
	States *corev1.Volume `json:"states,omitempty"`

	// Upgrade volume
	// +optional
	Upgrade *corev1.Volume `json:"upgrade,omitempty"`

	// Streams contains stream volume configurations
	// +optional
	Streams *ConsensusStreamVolumes `json:"streams,omitempty"`

	// AdditionalVolumes for custom mounts
	// +optional
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"`
}

// ConsensusStreamVolumes contains stream-specific volumes
type ConsensusStreamVolumes struct {
	// Block stream volume
	// +optional
	Block *corev1.Volume `json:"block,omitempty"`

	// Record stream volume
	// +optional
	Record *corev1.Volume `json:"record,omitempty"`

	// Events stream volume
	// +optional
	Events *corev1.Volume `json:"events,omitempty"`
}

// ConsensusPVCConfig contains PVC configurations
type ConsensusPVCConfig struct {
	// Logs PVC spec
	// +optional
	Logs *corev1.PersistentVolumeClaimSpec `json:"logs,omitempty"`

	// Stats PVC spec
	// +optional
	Stats *corev1.PersistentVolumeClaimSpec `json:"stats,omitempty"`

	// States PVC spec
	// +optional
	States *corev1.PersistentVolumeClaimSpec `json:"states,omitempty"`

	// Upgrade PVC spec
	// +optional
	Upgrade *corev1.PersistentVolumeClaimSpec `json:"upgrade,omitempty"`

	// Streams contains stream-specific PVC specs
	// +optional
	Streams *ConsensusPVCStreams `json:"streams,omitempty"`

	// AdditionalVolumes for custom PVCs
	// +optional
	AdditionalVolumes []corev1.PersistentVolumeClaim `json:"additionalVolumes,omitempty"`
}

// ConsensusPVCStreams contains stream-specific PVC specs
type ConsensusPVCStreams struct {
	// Block stream PVC
	// +optional
	Block *corev1.PersistentVolumeClaimSpec `json:"block,omitempty"`

	// Record stream PVC
	// +optional
	Record *corev1.PersistentVolumeClaimSpec `json:"record,omitempty"`

	// Events stream PVC
	// +optional
	Events *corev1.PersistentVolumeClaimSpec `json:"events,omitempty"`
}

// ConsensusCapsuleStatus defines the observed state of ConsensusCapsule
type ConsensusCapsuleStatus struct {
	// Phase is the current phase
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation observed by the operator
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AddressBookEntry is the address book entry for this capsule
	// +optional
	AddressBookEntry *AddressBookEntry `json:"addressBookEntry,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cc
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="NodeId",type="integer",JSONPath=".spec.nodeId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ConsensusCapsule is the Schema for the consensuscapsules API
type ConsensusCapsule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsensusCapsuleSpec   `json:"spec,omitempty"`
	Status ConsensusCapsuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConsensusCapsuleList contains a list of ConsensusCapsule
type ConsensusCapsuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConsensusCapsule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConsensusCapsule{}, &ConsensusCapsuleList{})
}
