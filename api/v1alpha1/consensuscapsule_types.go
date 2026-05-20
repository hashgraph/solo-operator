// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceType is the Kubernetes service type for a consensus node.
// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceTypeNodePort     ServiceType = "NodePort"
)

// ConsensusCapsuleSpec defines the desired state of ConsensusCapsule
type ConsensusCapsuleSpec struct {
	// orbit this capsule belongs to
	Orbit string `json:"orbit"`

	// +kubebuilder:validation:Minimum=0
	NodeId int64 `json:"nodeId"`

	// node account ID
	AccountId string `json:"accountId"`

	// consensus weight
	// +kubebuilder:validation:Minimum=0
	Weight int `json:"weight"`

	// log4j2.xml content; mounted into the hapi-app ConfigMap alongside settings.txt.
	// +kubebuilder:validation:Required
	Log4j2XML string `json:"log4j2.xml"`

	// sets CONSENSUS_NODE_ALIAS env consumed by entrypoint.sh
	// +optional
	NodeAlias string `json:"nodeAlias,omitempty"`

	// secret containing hedera.crt and hedera.key; mounted via subPath into HapiApp2.0/
	// +optional
	HapiAppSecretsName string `json:"hapiAppSecretsName,omitempty"`

	// when set, creates a block-nodes ConfigMap and mounts block-nodes.json
	// +optional
	BlockNodes *BlockNodesConfig `json:"blockNodes,omitempty"`

	// pod-level configuration
	PodProperties ConsensusPodProperties `json:"podProperties"`

	// key/cert secret references
	// +optional
	Secrets *ConsensusSecrets `json:"secrets,omitempty"`

	// overrides the auto-computed gossip hostname ({name}-0.consensus.{namespace}.svc.cluster.local).
	// Useful when the node is reached via a custom DNS name, an external IP, a service mesh virtual service, or across clusters.
	// +optional
	InternalAddress string `json:"internalAddress,omitempty"`

	// external FQDN or IP
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`

	// gossip port; defaults to 50111 when not set
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=50111
	// +optional
	GossipPort int32 `json:"gossipPort,omitempty"`

	// gRPC port; defaults to 50211 when not set
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=50211
	// +optional
	GrpcPort int32 `json:"grpcPort,omitempty"`

	// node.properties key-value pairs
	// +optional
	NodeProperties map[string]string `json:"nodeProperties,omitempty"`

	// hostPath volume containing config.txt
	// +optional
	ConfigFolder *corev1.HostPathVolumeSource `json:"configFolder,omitempty"`

	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType ServiceType `json:"serviceType,omitempty"`

	// annotations for the consensus-node service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// PVC configs for node volumes
	// +optional
	PersistentVolumeClaims *ConsensusPVCConfig `json:"persistentVolumeClaims,omitempty"`

	// Offline, when true, instructs the reconciler to keep the consensus pod
	// scaled to zero. NetworkUpgradeExecute toggles this around the daemon
	// handoff window so the consensus node stays down while infrastructure
	// files and config CRs are applied, then comes back up cleanly on the
	// new image. Default false.
	// +optional
	// +kubebuilder:default=false
	Offline bool `json:"offline,omitempty"`
}

// BlockNodesConfig defines block-node connection settings.
type BlockNodesConfig struct {
	// +optional
	Nodes []BlockNodeEndpoint `json:"nodes,omitempty"`

	// block items per batch
	// +kubebuilder:default=256
	// +optional
	BlockItemBatchSize int `json:"blockItemBatchSize,omitempty"`
}

// BlockNodeEndpoint is a single block node target.
type BlockNodeEndpoint struct {
	// host:port
	Address string `json:"address"`
}

// ConsensusSecrets holds TLS and signing key references.
type ConsensusSecrets struct {
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`

	// +optional
	SecretResources *ConsensusSecretResources `json:"secretResources,omitempty"`
}

// ConsensusSecretResources references specific key/cert secrets.
type ConsensusSecretResources struct {
	// +kubebuilder:validation:Required
	GrpcTlsPrivateKey KeyReference `json:"grpcTlsPrivateKey"`
	// +kubebuilder:validation:Required
	GrpcTlsPublicCertificate KeyReference `json:"grpcTlsPublicCertificate"`
	// +kubebuilder:validation:Required
	SigningPrivateKey KeyReference `json:"signingPrivateKey"`
	// +kubebuilder:validation:Required
	SigningPublicCertificate KeyReference `json:"signingPublicCertificate"`

	// AdminPublicKey references the ED25519 admin public key for this node.
	// Used when building genesis-network.json
	// +optional
	AdminPublicKey *KeyReference `json:"adminPublicKey,omitempty"`

	// +optional
	LedgerEncryptionKey *KeyReference `json:"ledgerEncryptionKey,omitempty"`

	// +optional
	LedgerSigningKey *KeyReference `json:"ledgerSigningKey,omitempty"`
}

// ConsensusPodProperties contains pod-level configuration.
type ConsensusPodProperties struct {
	Containers ConsensusContainers `json:"containers"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// +optional
	ResourceClaims []corev1.PodResourceClaim `json:"resourceClaims,omitempty"`

	// +kubebuilder:validation:Enum=Never;PreemptLowerPriority
	// +optional
	PreemptionPolicy string `json:"preemptionPolicy,omitempty"`

	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// +optional
	ServiceAccount *ServiceAccountConfig `json:"serviceAccount,omitempty"`

	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// +optional
	Volumes *ConsensusPodVolumes `json:"volumes,omitempty"`
}

type ServiceAccountConfig struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ConsensusContainers holds the container specs for the StatefulSet.
type ConsensusContainers struct {
	ConsensusNode ConsensusNodeContainer `json:"consensusNode"`

	// +optional
	BlocksUploader *CheetahUploader `json:"blocksUploader,omitempty"`

	// +optional
	RecordsUploader *CheetahUploader `json:"recordsUploader,omitempty"`

	// +optional
	EventsUploader *CheetahUploader `json:"eventsUploader,omitempty"`

	// +optional
	BackupUploader *StreamUploader `json:"backupUploader,omitempty"`

	// +optional
	Alloy *AlloyConfig `json:"alloy,omitempty"`
}

// ConsensusNodeContainer configures the main consensus node container.
type ConsensusNodeContainer struct {
	// +kubebuilder:default="consensus-node"
	// +optional
	Name string `json:"name,omitempty"`

	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// container image spec
	// +kubebuilder:validation:Required
	SoftwareVersion *SoftwareVersion `json:"softwareVersion"`

	// sets JAVA_HEAP_MIN (e.g. "256m" → -Xms256m)
	// +optional
	JavaHeapMin string `json:"javaHeapMin,omitempty"`

	// sets JAVA_HEAP_MAX (e.g. "8g" → -Xmx8g)
	// +optional
	JavaHeapMax string `json:"javaHeapMax,omitempty"`

	// sets JAVA_OPTS (e.g. "-XX:+UseG1GC -XX:MaxDirectMemorySize=1500m")
	// +optional
	JavaOpts string `json:"javaOpts,omitempty"`

	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// AlloyConfig is the Grafana Alloy telemetry sidecar config.
type AlloyConfig struct {
	// +kubebuilder:default="alloy"
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`

	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	Config *AlloyCollectorConfig `json:"config,omitempty"`

	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// AlloyCollectorConfig holds telemetry pipeline configuration.
type AlloyCollectorConfig struct {
	// +kubebuilder:default="hedera-node"
	// +optional
	AppName string `json:"appName,omitempty"`

	// +kubebuilder:default="integration"
	// +optional
	Environment string `json:"environment,omitempty"`

	// +kubebuilder:default="info"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// extra static labels merged with built-in labels (app, environment, node)
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Processors *AlloyProcessors `json:"processors,omitempty"`

	// +optional
	Exporters *AlloyExporters `json:"exporters,omitempty"`

	// +optional
	Receivers *AlloyReceivers `json:"receivers,omitempty"`
}

// AlloyProcessors holds processor configuration.
type AlloyProcessors struct {
	// +optional
	MemoryLimiter *AlloyMemoryLimiter `json:"memoryLimiter,omitempty"`
}

// AlloyMemoryLimiter configures the memory limiter processor.
type AlloyMemoryLimiter struct {
	// memory limit in MiB
	// +kubebuilder:default="400"
	// +optional
	LimitMib string `json:"limitMib,omitempty"`

	// spike limit in MiB (~20% of limit)
	// +kubebuilder:default="80"
	// +optional
	SpikeLimitMib string `json:"spikeLimitMib,omitempty"`
}

// AlloyExporters holds exporter configuration.
type AlloyExporters struct {
	// +optional
	Loki *AlloyLokiExporter `json:"loki,omitempty"`

	// when enabled, Alloy scrapes node metrics at :9999 and remote-writes them
	// +optional
	PrometheusRemote *AlloyPrometheusRemoteExporter `json:"prometheusRemote,omitempty"`
}

// AlloyPrometheusRemoteExporter configures Prometheus remote-write via Alloy.
type AlloyPrometheusRemoteExporter struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// remote-write URL
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// +kubebuilder:default="15s"
	// +optional
	ScrapeInterval string `json:"scrapeInterval,omitempty"`
}

// AlloyLokiExporter configures Loki log shipping.
type AlloyLokiExporter struct {
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled"`

	// +optional
	Auth *AlloyLokiAuth `json:"auth,omitempty"`

	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// AlloyLokiAuth holds Loki authentication configuration.
type AlloyLokiAuth struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// secret containing credentials
	// +optional
	Secret *corev1.LocalObjectReference `json:"secret,omitempty"`
}

// AlloyReceivers holds receiver configuration.
type AlloyReceivers struct {
	// +optional
	SwirldsVMap *AlloyReceiver `json:"swirldsVMap,omitempty"`

	// +optional
	Hashstream *AlloyReceiver `json:"hashstream,omitempty"`
}

// AlloyReceiver is a generic on/off receiver toggle.
type AlloyReceiver struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// ConsensusPodVolumes holds volume overrides for the pod.
type ConsensusPodVolumes struct {
	// +optional
	Logs *corev1.Volume `json:"logs,omitempty"`

	// +optional
	Stats *corev1.Volume `json:"stats,omitempty"`

	// +optional
	Saved *corev1.Volume `json:"saved,omitempty"`

	// +optional
	Upgrade *corev1.Volume `json:"upgrade,omitempty"`

	// +optional
	Streams *ConsensusStreamVolumes `json:"streams,omitempty"`

	// +optional
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty"`
}

// ConsensusStreamVolumes holds stream-specific volume overrides.
type ConsensusStreamVolumes struct {
	// +optional
	Block *corev1.Volume `json:"block,omitempty"`

	// +optional
	Record *corev1.Volume `json:"record,omitempty"`

	// +optional
	Events *corev1.Volume `json:"events,omitempty"`
}

// ConsensusPVCConfig holds PVC specs for each node volume.
// Volumes without a spec fall back to emptyDir.
type ConsensusPVCConfig struct {
	// → HapiApp2.0/output
	// +optional
	Logs *corev1.PersistentVolumeClaimSpec `json:"logs,omitempty"`

	// → HapiApp2.0/data/stats
	// +optional
	Stats *corev1.PersistentVolumeClaimSpec `json:"stats,omitempty"`

	// → HapiApp2.0/data/saved
	// +optional
	Saved *corev1.PersistentVolumeClaimSpec `json:"saved,omitempty"`

	// → HapiApp2.0/state
	// +optional
	State *corev1.PersistentVolumeClaimSpec `json:"state,omitempty"`

	// → HapiApp2.0/data/upgrade
	// +optional
	Upgrade *corev1.PersistentVolumeClaimSpec `json:"upgrade,omitempty"`

	// +optional
	Streams *ConsensusPVCStreams `json:"streams,omitempty"`

	// +optional
	AdditionalVolumes []corev1.PersistentVolumeClaim `json:"additionalVolumes,omitempty"`
}

// ConsensusPVCStreams holds PVC specs for stream output volumes.
type ConsensusPVCStreams struct {
	// +optional
	Block *corev1.PersistentVolumeClaimSpec `json:"block,omitempty"`

	// +optional
	Record *corev1.PersistentVolumeClaimSpec `json:"record,omitempty"`

	// +optional
	Events *corev1.PersistentVolumeClaimSpec `json:"events,omitempty"`
}

// ConsensusCapsuleStatus defines the observed state of ConsensusCapsule
type ConsensusCapsuleStatus struct {
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

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
