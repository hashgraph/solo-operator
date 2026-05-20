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

// Phase represents the current phase of a resource
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Unknown
type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
	PhaseUnknown   Phase = "Unknown"
)

// ImagePullPolicy represents the image pull policy
// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
type ImagePullPolicy string

const (
	ImagePullPolicyAlways       ImagePullPolicy = "Always"
	ImagePullPolicyNever        ImagePullPolicy = "Never"
	ImagePullPolicyIfNotPresent ImagePullPolicy = "IfNotPresent"
)

// SoftwareVersion defines the container image version
type SoftwareVersion struct {
	// Repository is the container registry (e.g., docker.io/library)
	Repository string `json:"repository"`

	// ImageName is the name of the image (e.g., ubuntu)
	ImageName string `json:"imageName"`

	// ImageTag is the tag of the image (e.g., focal)
	ImageTag string `json:"imageTag"`

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
	LayerHashes []string `json:"layerHashes"`
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
type CheetahUploader struct {
	// Enabled enables the uploader
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Name is the container name
	// +optional
	Name string `json:"name,omitempty"`

	// Image is the container image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// SoftwareVersion specifies the container image repository and tag
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

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
type StreamUploader struct {
	// Name is the container name
	// +optional
	Name string `json:"name,omitempty"`

	// Image is the container image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// SoftwareVersion specifies the container image repository and tag
	// +optional
	SoftwareVersion *SoftwareVersion `json:"softwareVersion,omitempty"`

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

	// AdditionalVolumeMounts for the uploader
	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
}

// Condition is an alias for metav1.Condition
type Condition = metav1.Condition
