// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type ComponentName string

const (
	ComponentConsensusNode        ComponentName = "consensusNode"
	ComponentRecordStreamUploader ComponentName = "recordStreamUploader"
	ComponentEventStreamUploader  ComponentName = "eventStreamUploader"
	ComponentBlockStreamUploader  ComponentName = "blockStreamUploader"
	ComponentBackupUploader       ComponentName = "backupUploader"
	ComponentUC                   ComponentName = "uc"
)

var AllUpgradeComponents = []ComponentName{
	ComponentConsensusNode,
	ComponentRecordStreamUploader,
	ComponentEventStreamUploader,
	ComponentBlockStreamUploader,
	ComponentBackupUploader,
	ComponentUC,
}

// UpgradeImagesSpec is the verbatim "images:" section of consensus-node-components.yaml.
// Embedded in Prepare, Freeze and Execute specs so each CR is self-contained.
type UpgradeImagesSpec struct {
	ConsensusNode UpgradeComponentSpec `json:"consensusNode"`
	// +optional
	RecordStreamUploader *UpgradeComponentSpec `json:"recordStreamUploader,omitempty"`
	// +optional
	EventStreamUploader *UpgradeComponentSpec `json:"eventStreamUploader,omitempty"`
	// +optional
	BlockStreamUploader *UpgradeComponentSpec `json:"blockStreamUploader,omitempty"`
	// +optional
	BackupUploader *UpgradeComponentSpec `json:"backupUploader,omitempty"`
	// +optional
	UC *UpgradeComponentSpec `json:"uc,omitempty"`
}

type UpgradeComponentSpec struct {
	Version string `json:"version"`

	// Enabled=false means present-but-remove the sidecar from the consensus pod on execute.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	Deterministic UpgradeDeterministicSpec `json:"deterministic"`

	// +kubebuilder:validation:MinItems=1
	Registries []UpgradeRegistrySpec `json:"registries"`
}

// UpgradeDeterministicSpec carries layer hashes when Supported=true.
// When Supported=false, hashes live per-registry on UpgradeRegistrySpec instead.
type UpgradeDeterministicSpec struct {
	Supported bool `json:"supported"`
	// Keyed by platform ("linux/amd64", "linux/arm64").
	// +optional
	LayerHashes map[string][]Sha256Digest `json:"layerHashes,omitempty"`
}

type UpgradeRegistrySpec struct {
	Image string `json:"image"`
	// Per-registry layer hashes for non-deterministic builds.
	// +optional
	LayerHashes map[string][]Sha256Digest `json:"layerHashes,omitempty"`
}

// SelectedSources is the per-component validation outcome. Lives in
// Prepare.status; UC copies it into Freeze.spec and Execute.spec verbatim.
type SelectedSources struct {
	// +optional
	ConsensusNode *SelectedSource `json:"consensusNode,omitempty"`
	// +optional
	RecordStreamUploader *SelectedSource `json:"recordStreamUploader,omitempty"`
	// +optional
	EventStreamUploader *SelectedSource `json:"eventStreamUploader,omitempty"`
	// +optional
	BlockStreamUploader *SelectedSource `json:"blockStreamUploader,omitempty"`
	// +optional
	BackupUploader *SelectedSource `json:"backupUploader,omitempty"`
	// +optional
	UC *SelectedSource `json:"uc,omitempty"`
}

// Sha256Digest is an OCI sha256 digest ("sha256:" + 64 lowercase hex chars).
// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
type Sha256Digest string

type SelectedSource struct {
	Image  string       `json:"image"`
	Digest Sha256Digest `json:"digest"`
	// LayerHashesVerified is true when expected hashes were configured and matched
	// what the registry served; false when no hashes were configured or on mismatch.
	LayerHashesVerified bool `json:"layerHashesVerified"`
}

// Labels linking all CRs of one upgrade operation.
const (
	LabelOperationID = "operator.solo.hedera.com/operation-id"
	LabelStep        = "operator.solo.hedera.com/step"
)

const (
	StepPrepare = "prepare"
	StepFreeze  = "freeze"
	StepExecute = "execute"
	StepAbort   = "abort"
)

// Condition types per CR.
const (
	// Prepare
	CondManifestValidated = "ManifestValidated"
	CondImageValidation   = "ImageValidation"
	CondRegistrySelected  = "RegistrySelected"

	// Freeze
	CondRegistryHealthCheck = "RegistryHealthCheck"
	CondReadyForFreeze      = "ReadyForFreeze"

	// Execute
	CondConfigCRsApplied        = "ConfigCRsApplied"
	CondDaemonResult            = "DaemonResult"
	CondConsensusCapsulePatched = "ConsensusCapsulePatched"
	CondNodesUpgraded           = "NodesUpgraded"
	CondNodeStarting            = "NodeStarting"

	// Abort
	CondAbortProcessed = "AbortProcessed"
)

// Reason constants used in conditions and events.
const (
	ReasonSchemaValid       = "SchemaValid"
	ReasonAllImagesValid    = "AllImagesValid"
	ReasonSelectionComplete = "SelectionComplete"
	ReasonRegistryInvalid   = "RegistryInvalid"
	ReasonRegistryFailover  = "RegistryFailover"

	ReasonHealthCheckPassed = "HealthCheckPassed"
	ReasonPreFreezePassed   = "PreFreezePassed"

	ReasonScaledDown       = "ScaledDown"
	ReasonReadyForDaemon   = "ReadyForProvisionerDaemon"
	ReasonPatchApplied     = "PatchApplied"
	ReasonPodReady         = "PodReady"
	ReasonUpgradeSucceeded = "UpgradeSucceeded"
	ReasonUpgradeFailed    = "UpgradeFailed"

	ReasonAbortSuccess     = "Success"
	ReasonAbortAfterFreeze = "AbortAfterFreeze"
)

// DaemonResult condition reasons, written by the provisioner daemon.
const (
	ReasonDaemonSucceeded          = "Succeeded"
	ReasonDaemonFileDownloadFailed = "FileDownloadFailed"
	ReasonDaemonFileHashMismatch   = "FileHashMismatch"
	ReasonDaemonInfraUpgradeFailed = "InfraUpgradeFailed"
	ReasonDaemonSelfUpgradeFailed  = "SelfUpgradeFailed"
)

// Event reasons posted on NetworkUpgrade* CRs.
const (
	EventRegistryAuditComplete = "RegistryAuditComplete"
	EventRegistryUnreachable   = "RegistryUnreachable"
	EventRegistrySelected      = "RegistrySelected"
	EventRegistryInvalid       = "RegistryInvalid"
	EventRegistryFailover      = "RegistryFailover"

	EventPodScaledDown      = "PodScaledDown"
	EventReadyForDaemon     = "ReadyForProvisionerDaemon"
	EventPendingNodeUpgrade = "PendingNodeUpgrade"
	EventCapsulePatched     = "CapsulePatched"
	EventUpgradeSucceeded   = "UpgradeSucceeded"
	EventUpgradeFailed      = "UpgradeFailed"

	EventAbortProcessed   = "AbortProcessed"
	EventAbortAfterFreeze = "AbortAfterFreeze"
)
