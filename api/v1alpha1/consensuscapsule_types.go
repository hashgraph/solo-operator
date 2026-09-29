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

	// references a Log4j2Config CR in the same namespace whose managed ConfigMap
	// is mounted as log4j2.xml. Changes take effect on the next pod restart.
	// +kubebuilder:validation:MinLength=1
	Log4j2ConfigRef string `json:"log4j2ConfigRef"`

	// references a NodeSettings CR in the same namespace whose managed ConfigMap
	// is mounted as settings.txt. Changes take effect on the next pod restart.
	// +kubebuilder:validation:MinLength=1
	SettingsConfigRef string `json:"settingsConfigRef"`

	// references an ApplicationProperties CR in the same namespace whose managed ConfigMap
	// is subPath-mounted at data/config/application.properties.
	// +kubebuilder:validation:MinLength=1
	ApplicationPropertiesRef string `json:"applicationPropertiesRef"`

	// references an ApiPermissionProperties CR whose managed ConfigMap is subPath-mounted
	// at data/config/api-permission.properties. Optional — CN falls back to defaults.
	// +optional
	ApiPermissionPropertiesRef string `json:"apiPermissionPropertiesRef,omitempty"`

	// references a NodeProperties CR whose managed ConfigMap is subPath-mounted
	// at data/config/node.properties. Optional — CN falls back to defaults.
	// +optional
	NodePropertiesRef string `json:"nodePropertiesRef,omitempty"`

	// references a ThrottlesConfig CR whose managed ConfigMap is subPath-mounted
	// at data/config/throttles.json. Optional — CN falls back to defaults.
	// +optional
	ThrottlesConfigRef string `json:"throttlesConfigRef,omitempty"`

	// references a FeeSchedules CR whose managed ConfigMap is subPath-mounted
	// at data/config/feeSchedules.json. Optional — CN falls back to defaults.
	// +optional
	FeeSchedulesRef string `json:"feeSchedulesRef,omitempty"`

	// references a SimpleFeesSchedules CR whose managed ConfigMap is subPath-mounted
	// at data/config/simpleFeesSchedules.json. Optional — CN falls back to defaults.
	// +optional
	SimpleFeesSchedulesRef string `json:"simpleFeesSchedulesRef,omitempty"`

	// references an ApplicationOverrideProperties CR whose managed ConfigMap is subPath-mounted
	// at data/config/application-override.properties. Optional — CN falls back to defaults.
	// +optional
	ApplicationOverridePropertiesRef string `json:"applicationOverridePropertiesRef,omitempty"`

	// references a BootstrapProperties CR whose managed ConfigMap is subPath-mounted
	// at data/config/bootstrap.properties. Optional — CN falls back to defaults.
	// +optional
	BootstrapPropertiesRef string `json:"bootstrapPropertiesRef,omitempty"`

	// sets CONSENSUS_NODE_ALIAS env consumed by entrypoint.sh
	// +optional
	NodeAlias string `json:"nodeAlias,omitempty"`

	// references a BlockNodesConfig CR whose managed ConfigMap is subPath-mounted
	// at data/config/block-nodes.json. Optional — CN falls back to defaults.
	// The consensus-node deployment package delivers this per node under
	// block-nodes/config/block-nodes-<NODE_ID>.json; the UC selects the file for
	// this node and stages it verbatim into the CR (HIP-1494).
	// +optional
	BlockNodesConfigRef string `json:"blockNodesConfigRef,omitempty"`

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

	// Offline explicitly scales the consensus StatefulSet to 0 when true, meaning no pod exists, no resources are consumed, and no background processes run.
	// This is distinct from DesiredRunState=Stopped, where the pod remains up but the Java process is idle.
	// +optional
	// +kubebuilder:default=false
	Offline bool `json:"offline,omitempty"`

	// StartPolicy selects who starts the consensus process upon pod boot (HIP-1494/1495).
	// 'Auto' (default) starts the Java process automatically.
	// 'Manual' holds the Java process idle in the Stopped phase until you manually patch DesiredRunState to 'Running'.
	// +optional
	// +kubebuilder:default=Auto
	StartPolicy StartPolicy `json:"startPolicy,omitempty"`

	// DesiredRunState is the desired s6 process state (Running or Stopped).
	// When 'Stopped', the pod is running and sidecars are active, but the main Java process is idle.
	// +optional
	// +kubebuilder:validation:Enum=Running;Stopped
	DesiredRunState RunState `json:"desiredRunState,omitempty"`
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

	// +optional
	UC *UcSidecar `json:"uc,omitempty"`
}

// UcSidecar configures the Update Coordinator sidecar container.
type UcSidecar struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// SoftwareVersionSource declares multiple candidate registries (with layer hashes)
	// for this container's image; when set it takes precedence over SoftwareVersion and
	// the operator selects, verifies, and digest-pins the deployed image.
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`

	// Interval between marker-file polls during normal operation (default: 60s).
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="pollInterval must be a positive Go duration (e.g. '10s', '1m30s')"
	PollInterval string `json:"pollInterval,omitempty"`

	// Interval between marker-file polls during the freeze window (default: 1s).
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="freezePollInterval must be a positive Go duration (e.g. '100ms', '1s')"
	FreezePollInterval string `json:"freezePollInterval,omitempty"`

	// Interval between platform-status metrics scrapes (default: 5s).
	// Lower values detect ACTIVE faster after an upgrade but increase
	// metrics endpoint load.
	// +optional
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')",message="platformPollInterval must be a positive Go duration (e.g. '1s', '5s')"
	PlatformPollInterval string `json:"platformPollInterval,omitempty"`
}

// ConsensusNodeContainer configures the main consensus node container.
// +kubebuilder:validation:XValidation:rule="has(self.softwareVersion) || has(self.softwareVersionSource)",message="one of softwareVersion or softwareVersionSource is required"
type ConsensusNodeContainer struct {
	// +kubebuilder:default="consensus-node"
	// +optional
	Name string `json:"name,omitempty"`

	// container image spec; mutually optional with softwareVersionSource — at least one must be set.
	// When softwareVersionSource is also set it takes precedence and this field is ignored.
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// SoftwareVersionSource declares multiple candidate registries (with layer hashes)
	// for this container's image; when set it takes precedence over SoftwareVersion and
	// the operator selects, verifies, and digest-pins the deployed image.
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

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
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// SoftwareVersionSource declares multiple candidate registries (with layer hashes)
	// for this container's image; when set it takes precedence over SoftwareVersion and
	// the operator selects, verifies, and digest-pins the deployed image.
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

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

	// → /opt/solo/weaver/uc (UC sidecar runtime root: crs/, events/, backup/, uc.sock).
	// When nil an emptyDir is used — suitable for local dev and CI. Set this for
	// cluster-only deployments where UC state must survive pod restarts.
	// +optional
	UCData *corev1.PersistentVolumeClaimSpec `json:"ucData,omitempty"`

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

// Platform status names are the raw platform_PlatformStatus states reported by
// the consensus node's metrics endpoint (localhost:9999). The UC sidecar scrapes
// them and publishes the current one as the reason of the
// operator.solo.hiero.org/platform-status condition on its own pod (see
// PodConditionPlatformStatus); the operator proxies
// that into ConsensusCapsuleStatus.PlatformStatus (the operator is the sole writer
// of the capsule status) and rolls up the capsule phase. The names mirror the
// Hedera platform PlatformStatus enum; the numeric codes are carried in PlatformStatusCode.
const (
	PlatformStatusNoValue             = "NO_VALUE"
	PlatformStatusStartingUp          = "STARTING_UP"
	PlatformStatusActive              = "ACTIVE"
	PlatformStatusBehind              = "BEHIND"
	PlatformStatusFreezing            = "FREEZING"
	PlatformStatusFreezeComplete      = "FREEZE_COMPLETE"
	PlatformStatusReplayingEvents     = "REPLAYING_EVENTS"
	PlatformStatusObserving           = "OBSERVING"
	PlatformStatusChecking            = "CHECKING"
	PlatformStatusReconnectComplete   = "RECONNECT_COMPLETE"
	PlatformStatusCatastrophicFailure = "CATASTROPHIC_FAILURE"
	// PlatformStatusUnknown is written by the UC when the metrics endpoint is
	// unreachable or the reported code is unrecognised (fail-soft).
	PlatformStatusUnknown = "UNKNOWN"
)

// CondUCSidecarReady is the condition type for the UC sidecar health gate on ConsensusCapsule.
const CondUCSidecarReady = "UCSidecarReady"

// CondImageDigestPinned is the condition type for the orbit.spec.requireDigestOnDeploy gate:
// it is False when the Orbit requires digest-pinned images on deploy but a running image
// (the consensus node or an enabled sidecar/uploader) is not digest-pinned.
const CondImageDigestPinned = "ImageDigestPinned"

// CondImagesDeclared is the condition type for the explicit-image requirement: it is False
// when a running container (the consensus node or an enabled sidecar/uploader) has no
// declared image. The operator provides no built-in default images.
const CondImagesDeclared = "ImagesDeclared"

// ConsensusCapsuleStatus defines the observed state of ConsensusCapsule
type ConsensusCapsuleStatus struct {
	// Phase is the ConsensusCapsule lifecycle phase. Active (see PhaseActive) is a
	// consensus-only value: it is permitted here but excluded from every other CRD's
	// phase enum. The Stopped phase indicates the pod is up but the main consensus Java process is idle.
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Active;Succeeded;Failed;Unknown;Stopped
	Phase Phase `json:"phase,omitempty"`

	// PlatformStatus is the consensus node's live platform state (raw
	// platform_PlatformStatus name, e.g. STARTING_UP / REPLAYING_EVENTS / ACTIVE /
	// FREEZE_COMPLETE / CATASTROPHIC_FAILURE), or UNKNOWN when the current pod has
	// not reported yet. Written exclusively by the operator, which proxies it from
	// the operator.solo.hiero.org/platform-status condition the UC sidecar maintains
	// on its own pod (see PodConditionPlatformStatus). The operator is the sole writer of the
	// ConsensusCapsule status; the UC never patches this resource.
	// +optional
	PlatformStatus string `json:"platformStatus,omitempty"`

	// PlatformStatusCode is the numeric platform_PlatformStatus code corresponding
	// to PlatformStatus (PlatformStatusCodeUnknown when UNKNOWN). Written
	// exclusively by the operator, which derives it from the proxied name.
	// +optional
	PlatformStatusCode int32 `json:"platformStatusCode,omitempty"`

	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// RunState is the consensus node's live s6 process state.
	// +optional
	RunState RunState `json:"runState,omitempty"`

	// +optional
	AddressBookEntry *AddressBookEntry `json:"addressBookEntry,omitempty"`

	// ResolvedImages records the outcome of multi-registry image selection for each
	// container that declares a SoftwareVersionSource. The operator caches the resolved
	// pinned image here and reuses it while the source is unchanged (keyed by SourceHash),
	// re-inspecting registries only when the source spec changes. Written solely by the
	// operator. Containers using a single SoftwareVersion have no entry.
	// +optional
	// +listType=map
	// +listMapKey=container
	ResolvedImages []ResolvedImageStatus `json:"resolvedImages,omitempty"`
}

// ResolvedImageStatus is the cached result of resolving one container's
// SoftwareVersionSource to a verified, digest-pinned image.
type ResolvedImageStatus struct {
	// Container is the container this entry resolves (e.g. "consensusNode", "uc").
	Container string `json:"container"`

	// SourceHash fingerprints the SoftwareVersionSource that produced this selection;
	// a mismatch on reconcile triggers re-resolution.
	SourceHash string `json:"sourceHash"`

	// Image is the resolved reference pinned to its content digest
	// (repository/imageName:imageTag@sha256:...).
	Image string `json:"image"`

	// Digest is the verified content digest the image is pinned to.
	// +optional
	Digest Sha256Digest `json:"digest,omitempty"`

	// Repository is the winning candidate repository.
	Repository string `json:"repository"`

	// LayerHashesVerified is true when the candidate's layer hashes matched.
	LayerHashesVerified bool `json:"layerHashesVerified"`

	// Validity is the selection outcome for this container.
	Validity ImageValidity `json:"validity"`

	// LastProbeTime is when this entry was last (re)resolved against the registries.
	// It shows how stale the result is — most useful for an Invalid entry that is being
	// retried each reconcile.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cc
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="NodeId",type="integer",JSONPath=".spec.nodeId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Platform",type="string",JSONPath=".status.platformStatus"
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
