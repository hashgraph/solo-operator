// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProxyServiceType represents how a proxy service is exposed
// +kubebuilder:validation:Enum=ClusterIP;Ingress;LoadBalancer
type ProxyServiceType string

const (
	ProxyServiceTypeClusterIP    ProxyServiceType = "ClusterIP"
	ProxyServiceTypeIngress      ProxyServiceType = "Ingress"
	ProxyServiceTypeLoadBalancer ProxyServiceType = "LoadBalancer"
)

// Phase represents the lifecycle phase of a resource.
type Phase string

// Phase carries no type-level enum on purpose: each CRD's status field declares its
// own +kubebuilder:validation:Enum marker, so Active (a consensus-only phase) is
// permitted on ConsensusCapsule and excluded everywhere else. Keep those field
// markers in sync when adding or removing a phase value.
const (
	PhasePending Phase = "Pending"
	PhaseRunning Phase = "Running"
	// PhaseActive is reached only when the pod is Ready AND the consensus node has
	// reported platform status ACTIVE for the current pod revision. It is a sticky
	// milestone: transient non-ACTIVE dips (BEHIND, RECONNECT, …) keep the phase at
	// Active — the live truth stays in status.platformStatus. Only a
	// CATASTROPHIC_FAILURE or a pod restart leaves Active.
	PhaseActive    Phase = "Active"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
	PhaseUnknown   Phase = "Unknown"
	// PhaseStopped is a non-failure resting state for a Manual-start capsule whose
	// consensus process is observed not running (not-yet-started, stopped by a
	// Start/Stop CR, or crashed). It is distinct from Failed (never a failure) and
	// from Offline (which scales the pod to zero); the pod stays up. A fresh
	// ConsensusNodeStart is required to leave Stopped.
	PhaseStopped Phase = "Stopped"
)

// StartPolicy selects who starts the consensus process: Auto on pod boot, or
// Manual (held down, started out-of-band via the Start/Stop control interface).
// +kubebuilder:validation:Enum=Auto;Manual
type StartPolicy string

const (
	StartPolicyAuto   StartPolicy = "Auto"
	StartPolicyManual StartPolicy = "Manual"
)

// RunState represents the desired s6-managed CN process state.
// +kubebuilder:validation:Enum=Running;Stopped
type RunState string

const (
	RunStateRunning RunState = "Running"
	RunStateStopped RunState = "Stopped"
)

// ImagePullPolicy represents the image pull policy
// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
type ImagePullPolicy string

const (
	ImagePullPolicyAlways       ImagePullPolicy = "Always"
	ImagePullPolicyNever        ImagePullPolicy = "Never"
	ImagePullPolicyIfNotPresent ImagePullPolicy = "IfNotPresent"
)

// SoftwareVersion defines the container image version.
// The image must be pinned: either a non-empty imageTag or a digest. An empty tag with
// no digest would resolve to :latest — a mutable reference that could silently upgrade or
// downgrade the running image — so it is rejected.
// +kubebuilder:validation:XValidation:rule="size(self.imageTag) > 0 || has(self.digest)",message="set a non-empty imageTag or a digest so the image is not resolved to :latest"
type SoftwareVersion struct {
	// Repository is the container registry (e.g., docker.io/library)
	Repository string `json:"repository"`

	// ImageName is the name of the image (e.g., ubuntu)
	ImageName string `json:"imageName"`

	// ImageTag is the image tag (e.g. "0.65.1"). Must be at most 128 chars and not "latest"
	// (rejected even with a digest) so the image is pinned to a specific version.
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:XValidation:rule="self.lowerAscii() != 'latest'",message="imageTag must not be 'latest': pin a specific version so the image cannot silently change"
	ImageTag string `json:"imageTag"`

	// Digest, when set, pins the image to an immutable content digest ("sha256:...").
	// On the upgrade path it is populated from the verified SelectedSource.Digest so the
	// kubelet runs exactly the image that was verified during the Prepare phase, closing
	// the tag-mutation (TOCTOU) window. When empty, the image is referenced by tag only.
	// +optional
	Digest Sha256Digest `json:"digest,omitempty"`

	// ManifestHash is the optional manifest hash for verification
	// +optional
	ManifestHash string `json:"manifestHash,omitempty"`

	// ImageVerificationSpec contains hash verification details
	// +optional
	ImageVerificationSpec *ImageVerificationSpec `json:"imageVerificationSpec,omitempty"`

	// ImagePullSecrets is a list of references to secrets for pulling images
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy ImagePullPolicy `json:"imagePullPolicy,omitempty"`
}

// ImageVerificationSpec defines verification parameters for container images
type ImageVerificationSpec struct {
	// OS is the target operating system
	// +kubebuilder:validation:Enum=darwin;linux;windows
	// +optional
	OS string `json:"os,omitempty"`

	// Architecture is the target CPU architecture
	// +kubebuilder:validation:Enum=amd64;arm64
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// LayerHashes are the expected layer digest hashes
	// +kubebuilder:validation:MinItems=1
	LayerHashes []string `json:"layerHashes"`
}

// ImageRepository defines a container image repository
type ImageRepository struct {
	// Repository is the registry URL
	Repository string `json:"repository"`

	// ImageName is the image name
	ImageName string `json:"imageName"`

	// ImageTag is the image tag. A candidate registry in a SoftwareVersionSource must
	// carry an explicit, non-empty tag: an empty tag would resolve to :latest and differ
	// per registry. The mutable "latest" tag is also rejected, but in the reconciler
	// (latestTagInSource) rather than here — a per-candidate CEL rule over the unbounded
	// imageRepositories list would exceed the API server's cost budget.
	// +kubebuilder:validation:MinLength=1
	ImageTag string `json:"imageTag"`

	// ImagePullSecrets for accessing private registries
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// SoftwareVersionSource defines multiple candidate registries for a container image,
// together with the layer hashes used to verify them. It is mutually exclusive with the
// single SoftwareVersion: when set, the operator probes the registries, verifies
// each candidate's layer hashes, and deploys the resolved image pinned to its content
// digest. Layer hashes are mandatory (ImageVerificationSpec is required and non-empty)
// so a source is never deployed unverified — a container that needs no verification
// should use the single SoftwareVersion instead.
type SoftwareVersionSource struct {
	// ImageRepositories is the ordered list of candidate image repositories.
	// +kubebuilder:validation:MinItems=1
	ImageRepositories []ImageRepository `json:"imageRepositories"`

	// ImageVerificationSpec is the per-platform expected layer hashes every candidate
	// must match. Required and non-empty: a SoftwareVersionSource must carry layer hashes.
	// +kubebuilder:validation:MinItems=1
	ImageVerificationSpec []ImageVerificationSpec `json:"imageVerificationSpec"`

	// SelectionStrategy chooses the order candidate registries are probed in:
	// "Sequential" honors the listed order (primary first); "Random" spreads pull
	// traffic. Unset falls back to the operator's --registry-order default.
	// +kubebuilder:validation:Enum=Random;Sequential
	// +optional
	SelectionStrategy string `json:"selectionStrategy,omitempty"`

	// ImagePullPolicy specifies when to pull images
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy ImagePullPolicy `json:"imagePullPolicy,omitempty"`

	// Selected is the digest-pinned image chosen by a NetworkUpgradeExecute. When set, the
	// operator deploys this exact image and skips registry selection and verification — the
	// upgrade already verified it — while the candidate registries above remain for the next
	// upgrade's re-selection. Unset on a fresh install, where the operator selects and
	// verifies across the candidates. The selected image must be one of ImageRepositories.
	// +optional
	Selected *SelectedImage `json:"selected,omitempty"`
}

// SelectedImage is a digest-pinned image chosen by an upgrade: the winning registry's
// tag-only reference plus the content digest verified during the upgrade's prepare phase.
type SelectedImage struct {
	// Image is the winning registry's tag-only reference, e.g.
	// "ghcr.io/hashgraph/consensus-node:0.74.2". It must match one of the source's
	// ImageRepositories so the operator can reuse that candidate's pull secrets.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Digest is the verified content digest the image is pinned to.
	Digest Sha256Digest `json:"digest"`
}

// ImageValidity represents the validation status of an image
// +kubebuilder:validation:Enum=Unknown;Valid;Invalid;NotFound;PullFailed
type ImageValidity string

const (
	ImageValidityUnknown  ImageValidity = "Unknown"
	ImageValidityValid    ImageValidity = "Valid"
	ImageValidityInvalid  ImageValidity = "Invalid"
	ImageValidityNotFound ImageValidity = "NotFound"
	// ImageValidityPullFailed means the pinned image resolved fine but the kubelet could
	// not pull it (the pod is in ImagePullBackOff). It signals the next reconcile to
	// re-resolve the source and fail over to another candidate registry.
	ImageValidityPullFailed ImageValidity = "PullFailed"
)

// ImageRepositoryStatus represents the status of an image repository
type ImageRepositoryStatus struct {
	// Repository is the registry URL
	Repository string `json:"repository"`

	// ImageName is the image name
	ImageName string `json:"imageName"`

	// ImageTag is the image tag
	ImageTag string `json:"imageTag"`

	// Validity indicates the validation status
	Validity ImageValidity `json:"validity"`

	// SelectedForDeployment indicates if this repository was selected
	SelectedForDeployment bool `json:"selectedForDeployment"`

	// ManifestHash is the resolved manifest hash
	// +optional
	ManifestHash string `json:"manifestHash,omitempty"`

	// ImageVerificationSpec contains the verification spec used
	// +optional
	ImageVerificationSpec *ImageVerificationSpec `json:"imageVerificationSpec,omitempty"`

	// ImagePullSecrets for accessing the registry
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ImagePullPolicy for this image
	// +optional
	ImagePullPolicy ImagePullPolicy `json:"imagePullPolicy,omitempty"`
}

// AddressBookEntry represents a node in the address book
type AddressBookEntry struct {
	// NodeId is the unique identifier for the node
	// +kubebuilder:validation:Minimum=0
	NodeId int64 `json:"nodeId"`

	// NickName is a friendly name for the node
	NickName string `json:"nickName"`

	// SelfName is the self-identifying name
	SelfName string `json:"selfName"`

	// Weight is the consensus weight
	// +kubebuilder:validation:Minimum=0
	Weight int `json:"weight"`

	// InternalHostname is the internal FQDN or IP
	InternalHostname string `json:"internalHostname"`

	// InternalPort is the internal gossip port
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	InternalPort int32 `json:"internalPort"`

	// ExternalHostname is the external FQDN or IP
	ExternalHostname string `json:"externalHostname"`

	// ExternalPort is the external gossip port
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	ExternalPort int32 `json:"externalPort"`

	// NodeAccount is the account ID in the form "0.0.x"
	NodeAccount string `json:"nodeAccount"`

	// PublicSigningCertificate is the PEM-encoded signing certificate
	// +optional
	PublicSigningCertificate string `json:"publicSigningCertificate,omitempty"`

	// PublicLedgerKey is the base64-encoded public ledger key
	// +optional
	PublicLedgerKey string `json:"publicLedgerKey,omitempty"`
}

// KeyReference references a key in a secret
type KeyReference struct {
	// SecretName is the name of the secret
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// KeyName is the key within the secret
	// +optional
	KeyName string `json:"keyName,omitempty"`
}

// StreamUploaderConfig contains configuration for stream uploaders
type StreamUploaderConfig struct {
	// Debug enables debug logging
	// +kubebuilder:default=true
	// +optional
	Debug bool `json:"debug"`

	// EnableCompression enables stream compression
	// +kubebuilder:default=false
	// +optional
	EnableCompression bool `json:"enableCompression,omitempty"`

	// EnableSidecarFiles enables sidecar file processing
	// +kubebuilder:default=false
	// +optional
	EnableSidecarFiles bool `json:"enableSidecarFiles,omitempty"`

	// Reaper configuration
	// +optional
	Reaper *ReaperConfig `json:"reaper,omitempty"`

	// Signature configuration
	// +optional
	Signature *SignatureConfig `json:"signature,omitempty"`

	// StreamFileExtension is the extension for stream files
	// +optional
	StreamFileExtension string `json:"streamFileExtension,omitempty"`

	// StreamSigExtension is the extension for signature files
	// +optional
	StreamSigExtension string `json:"streamSigExtension,omitempty"`

	// StreamExtension is the general stream extension
	// +optional
	StreamExtension string `json:"streamExtension,omitempty"`

	// SigExtension is the signature extension
	// +optional
	SigExtension string `json:"sigExtension,omitempty"`

	// BucketName is the storage bucket name
	// +optional
	BucketName string `json:"bucketName,omitempty"`

	// BucketPath is the path within the bucket
	// +optional
	BucketPath string `json:"bucketPath,omitempty"`

	// S3 configuration
	// +optional
	S3 *S3Config `json:"s3,omitempty"`

	// GCS configuration
	// +optional
	GCS *GCSConfig `json:"gcs,omitempty"`

	// EnvFromSecret is the secret name for environment variables
	// +optional
	EnvFromSecret string `json:"envFromSecret,omitempty"`
}

// ReaperConfig contains reaper configuration
type ReaperConfig struct {
	// Enabled enables the reaper
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled"`

	// MinKeep is the minimum number of files to keep
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinKeep int `json:"minKeep,omitempty"`

	// Interval is the reaper interval
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Interval int `json:"interval,omitempty"`

	// DefaultBackoff is the default backoff time
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	DefaultBackoff int `json:"defaultBackoff,omitempty"`
}

// SignatureConfig contains signature configuration
type SignatureConfig struct {
	// Require requires signatures
	// +kubebuilder:default=true
	// +optional
	Require bool `json:"require"`

	// Prioritize prioritizes signature files
	// +kubebuilder:default=true
	// +optional
	Prioritize bool `json:"prioritize"`
}

// S3Config contains S3 storage configuration
type S3Config struct {
	// Enabled enables S3 storage
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AccessKey is the S3 access key
	// +kubebuilder:default="solo-operator"
	// +optional
	AccessKey string `json:"accessKey,omitempty"`

	// URL is the S3 endpoint URL
	// +optional
	URL string `json:"url,omitempty"`
}

// GCSConfig contains GCS storage configuration
type GCSConfig struct {
	// Enabled enables GCS storage
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AccessKey is the GCS access key
	// +kubebuilder:default="solo-operator"
	// +optional
	AccessKey string `json:"accessKey,omitempty"`

	// URL is the GCS endpoint URL
	// +optional
	URL string `json:"url,omitempty"`
}

// CheetahUploader is the Cheetah uploader configuration
// An optional sidecar may declare at most one image source — softwareVersion or
// softwareVersionSource, never both (a disabled uploader may declare neither).
// +kubebuilder:validation:XValidation:rule="!has(self.softwareVersion) || !has(self.softwareVersionSource)",message="set at most one of softwareVersion or softwareVersionSource"
type CheetahUploader struct {
	// Enabled enables the uploader
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Name is the container name
	// +optional
	Name string `json:"name,omitempty"`

	// SoftwareVersion specifies the container image repository and tag
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// SoftwareVersionSource declares multiple candidate registries (with layer hashes)
	// for this container's image; mutually exclusive with SoftwareVersion. When set, the
	// operator selects, verifies, and digest-pins the deployed image.
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// LogLevel is the log level
	// +kubebuilder:default="Info"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// S3 configuration (deprecated: use Blob instead)
	// +optional
	S3 *CheetahS3Config `json:"s3,omitempty"`

	// GCS configuration (deprecated: use Blob instead)
	// +optional
	GCS *CheetahGCSConfig `json:"gcs,omitempty"`

	// Blob configuration for multicloud storage via go-cloud (S3, GCS, Azure).
	// The URL scheme determines the cloud provider:
	//   - "s3://bucket?region=us-east-1" for AWS S3
	//   - "gs://bucket" for Google Cloud Storage
	//   - "azblob://container" for Azure Blob Storage
	// Credentials are resolved automatically via each provider's default chain,
	// including workload identity (EKS IRSA, GKE Workload Identity, Azure Managed Identity).
	// +optional
	Blob *CheetahBlobConfig `json:"blob,omitempty"`

	// EnvFromSecret is the secret name for credentials
	// +optional
	EnvFromSecret string `json:"envFromSecret,omitempty"`

	// MaxProcessors is the maximum number of concurrent file processors per pipeline
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxProcessors int `json:"maxProcessors"`

	// StopOnError stops the pipeline when an error occurs.
	// If false, the pipeline continues with the expectation that the error resolves on retry.
	// +kubebuilder:default=false
	// +optional
	StopOnError bool `json:"stopOnError,omitempty"`

	// BucketPrefix is prepended to all stream bucket object paths (e.g. "mynetwork/testnet")
	// +optional
	BucketPrefix string `json:"bucketPrefix,omitempty"`

	// ConfigCheckInterval is the interval for checking config changes and updating pipelines (e.g. "30s")
	// +kubebuilder:default="30s"
	// +optional
	ConfigCheckInterval string `json:"configCheckInterval,omitempty"`
}

// CheetahConfig contains Cheetah uploader configuration
type CheetahConfig struct {
	// Enabled enables Cheetah uploader
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SoftwareVersion specifies the Cheetah image
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// LogLevel is the log level
	// +kubebuilder:default="Info"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// StreamsConfig for record/events stream uploads
	// +optional
	StreamsConfig *CheetahStreamsConfig `json:"streamsConfig,omitempty"`

	// BackupConfig for state backup uploads
	// +optional
	BackupConfig *CheetahBackupConfig `json:"backupConfig,omitempty"`

	// EnvFromSecret is the secret name for S3/GCS credentials (S3_SECRET_KEY, GCS_SECRET_KEY)
	// +optional
	EnvFromSecret string `json:"envFromSecret,omitempty"`
}

// CheetahStreamsConfig contains stream upload configuration
type CheetahStreamsConfig struct {
	// Enabled enables stream uploads
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled"`

	// S3 configuration
	// +optional
	S3 *CheetahS3Config `json:"s3,omitempty"`

	// GCS configuration
	// +optional
	GCS *CheetahGCSConfig `json:"gcs,omitempty"`
}

// GetS3OrDefault returns S3 config or nil
func (c *CheetahStreamsConfig) GetS3OrDefault() *CheetahS3Config {
	if c == nil {
		return nil
	}
	return c.S3
}

// GetGCSOrDefault returns GCS config or nil
func (c *CheetahStreamsConfig) GetGCSOrDefault() *CheetahGCSConfig {
	if c == nil {
		return nil
	}
	return c.GCS
}

// CheetahBackupConfig contains backup upload configuration
type CheetahBackupConfig struct {
	// Enabled enables backup uploads
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// S3 configuration
	// +optional
	S3 *CheetahS3Config `json:"s3,omitempty"`

	// GCS configuration
	// +optional
	GCS *CheetahGCSConfig `json:"gcs,omitempty"`
}

// GetS3OrDefault returns S3 config or nil
func (c *CheetahBackupConfig) GetS3OrDefault() *CheetahS3Config {
	if c == nil {
		return nil
	}
	return c.S3
}

// GetGCSOrDefault returns GCS config or nil
func (c *CheetahBackupConfig) GetGCSOrDefault() *CheetahGCSConfig {
	if c == nil {
		return nil
	}
	return c.GCS
}

// CheetahS3Config contains S3 configuration for Cheetah
type CheetahS3Config struct {
	// Enabled enables S3 storage
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Bucket is the S3 bucket name
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Region is the S3 region
	// +kubebuilder:default="us-east-1"
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint is the S3 endpoint URL
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// UseSsl enables SSL
	// +kubebuilder:default=false
	// +optional
	UseSsl bool `json:"useSsl,omitempty"`
}

// CheetahGCSConfig contains GCS configuration for Cheetah
type CheetahGCSConfig struct {
	// Enabled enables GCS storage
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Bucket is the GCS bucket name
	// +optional
	Bucket string `json:"bucket,omitempty"`

	// Region is the GCS region
	// +kubebuilder:default="us-east-1"
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint is the GCS endpoint URL
	// +kubebuilder:default="storage.googleapis.com"
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// UseSsl enables SSL
	// +kubebuilder:default=true
	// +optional
	UseSsl bool `json:"useSsl"`
}

// CheetahBlobConfig contains multicloud blob storage configuration for Cheetah.
// Uses gocloud.dev/blob for unified access to S3, GCS, and Azure Blob Storage.
// +kubebuilder:validation:XValidation:rule="!self.enabled || size(self.url) > 0",message="url is required when blob storage is enabled"
type CheetahBlobConfig struct {
	// Enabled enables blob storage
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// URL is the gocloud.dev bucket URL.
	// The scheme determines the cloud provider:
	//   - "s3://bucket?region=us-east-1" for AWS S3
	//   - "gs://bucket" for Google Cloud Storage
	//   - "azblob://container" for Azure Blob Storage
	// +optional
	URL string `json:"url,omitempty"`
}

// StreamUploader defines a stream uploader sidecar
// An optional sidecar may declare at most one image source — softwareVersion or
// softwareVersionSource, never both (a disabled uploader may declare neither).
// +kubebuilder:validation:XValidation:rule="!has(self.softwareVersion) || !has(self.softwareVersionSource)",message="set at most one of softwareVersion or softwareVersionSource"
type StreamUploader struct {
	// Name is the container name
	// +optional
	Name string `json:"name,omitempty"`

	// SoftwareVersion specifies the container image repository and tag
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

	// SoftwareVersionSource declares multiple candidate registries (with layer hashes)
	// for this container's image; mutually exclusive with SoftwareVersion. When set, the
	// operator selects, verifies, and digest-pins the deployed image.
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Enabled enables the uploader
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Config contains uploader configuration
	// +optional
	Config *StreamUploaderConfig `json:"config,omitempty"`

	// AdditionalVolumeMounts for the uploader, appended to the default saved and stats
	// mounts; a mount with the same mountPath replaces the default.
	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// StreamUploaderChangeset defines changes to apply to a stream uploader
type StreamUploaderChangeset struct {
	// Name is the container name
	// +optional
	Name string `json:"name,omitempty"`

	// SoftwareVersionSource specifies the new image source
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// Resources specifies new resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Enabled changes the enabled state
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Config contains configuration changes
	// +optional
	Config *StreamUploaderConfig `json:"config,omitempty"`
}

// NodeProperties represents node-specific properties
type NodeProperties struct {
	// NodeId is the node identifier
	// +kubebuilder:validation:Minimum=0
	NodeId int64 `json:"nodeId"`

	// Configuration is the key-value configuration
	Configuration map[string]string `json:"configuration"`
}

// NodeLoggingConfiguration represents node-specific logging
type NodeLoggingConfiguration struct {
	// NodeId is the node identifier
	// +kubebuilder:validation:Minimum=0
	NodeId int64 `json:"nodeId"`

	// Configuration is the log4j2.xml content
	Configuration string `json:"configuration"`
}

// ConsensusNodeChangeset defines changes for consensus nodes
type ConsensusNodeChangeset struct {
	// SoftwareVersionSource specifies the new image source
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// NodeProperties are node-specific property changes
	// +optional
	NodeProperties []NodeProperties `json:"nodeProperties,omitempty"`

	// Logging contains logging configuration changes
	// +optional
	Logging []NodeLoggingConfiguration `json:"logging,omitempty"`
}

// HaproxyChangeset defines changes for HAProxy
type HaproxyChangeset struct {
	// Haproxy contains HAProxy-specific changes
	// +optional
	Haproxy *HaproxyConfig `json:"haproxy,omitempty"`

	// ServiceType is the Kubernetes service type
	// +kubebuilder:validation:Enum=ClusterIP;Ingress;LoadBalancer
	// +kubebuilder:default=LoadBalancer
	// +optional
	ServiceType string `json:"serviceType,omitempty"`

	// ServiceSpec is the full service specification
	// +optional
	ServiceSpec *corev1.ServiceSpec `json:"serviceSpec,omitempty"`
}

// HaproxyConfig contains HAProxy configuration
type HaproxyConfig struct {
	// SoftwareVersionSource specifies the image source
	// +optional
	SoftwareVersionSource *SoftwareVersionSource `json:"softwareVersionSource,omitempty"`

	// User is the stats username
	// +kubebuilder:default="user"
	// +optional
	User string `json:"user,omitempty"`

	// Auth is the stats authentication
	// +kubebuilder:default="auth"
	// +optional
	Auth string `json:"auth,omitempty"`
}

// Condition is an alias for metav1.Condition
type Condition = metav1.Condition
