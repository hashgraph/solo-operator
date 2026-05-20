// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ComponentName identifies a component in a NetworkUpgrade* CR's spec.images section.
// The string values are the JSON/YAML keys used in spec.images and status.selectedSources.
type ComponentName string

const (
	ComponentConsensusNode        ComponentName = "consensusNode"
	ComponentRecordStreamUploader ComponentName = "recordStreamUploader"
	ComponentEventStreamUploader  ComponentName = "eventStreamUploader"
	ComponentBlockStreamUploader  ComponentName = "blockStreamUploader"
	ComponentBackupUploader       ComponentName = "backupUploader"
	ComponentUC                   ComponentName = "uc"
)

// AllUpgradeComponents is the canonical iteration order for component validation.
var AllUpgradeComponents = []ComponentName{
	ComponentConsensusNode,
	ComponentRecordStreamUploader,
	ComponentEventStreamUploader,
	ComponentBlockStreamUploader,
	ComponentBackupUploader,
	ComponentUC,
}

// UpgradeImagesSpec is the verbatim "images:" section of consensus-node-components.yaml.
// It is embedded into the Prepare, Freeze and Execute CR specs so each CR is
// self-contained and survives independently of its predecessors.
type UpgradeImagesSpec struct {
	// ConsensusNode is the required main container image set.
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

// UpgradeComponentSpec is the per-component block in UpgradeImagesSpec.
type UpgradeComponentSpec struct {
	// Version is the release version (e.g. "0.75.0") for audit/labelling.
	Version string `json:"version"`

	// Enabled defaults to true when omitted. false means the component is present
	// in the manifest but should be removed from the consensus pod on execute.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Deterministic carries the platform-keyed layer-hash expectations for
	// deterministic builds.
	Deterministic UpgradeDeterministicSpec `json:"deterministic"`

	// Registries is the list of OCI image references this component can be
	// pulled from. At least one entry is required.
	// +kubebuilder:validation:MinItems=1
	Registries []UpgradeRegistrySpec `json:"registries"`
}

// UpgradeDeterministicSpec describes layer-hash expectations when the build is
// deterministic. When Supported is false, hashes are supplied per-registry on
// UpgradeRegistrySpec instead.
type UpgradeDeterministicSpec struct {
	Supported bool `json:"supported"`

	// LayerHashes is keyed by platform string (e.g. "linux/amd64", "linux/arm64")
	// with an ordered list of layer SHA256 digests.
	// +optional
	LayerHashes map[string][]string `json:"layerHashes,omitempty"`
}

// UpgradeRegistrySpec is one OCI image source for the component.
type UpgradeRegistrySpec struct {
	// Image is the OCI image reference (with tag or digest).
	Image string `json:"image"`

	// LayerHashes is per-registry layer hashes for non-deterministic builds.
	// Keyed by platform string.
	// +optional
	LayerHashes map[string][]string `json:"layerHashes,omitempty"`
}

// SelectedSources is the per-component result of registry validation.
// It lives in NetworkUpgradePrepare.status; the UC copies it into Freeze.spec
// and Execute.spec verbatim. The Freeze and Execute reconcilers may write an
// updated copy to their own status when failover occurs.
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

// SelectedSource is the validation outcome for one component.
type SelectedSource struct {
	// Image is the OCI image reference that was selected.
	Image string `json:"image"`

	// Digest is the resolved manifest digest (e.g. "sha256:...").
	Digest string `json:"digest"`

	// LayerHashesVerified is true when expected layer hashes were configured
	// and matched what the registry served. False either when no hashes were
	// configured to compare against, or when a mismatch was detected.
	LayerHashesVerified bool `json:"layerHashesVerified"`
}

// Label keys used to link the four NetworkUpgrade* CRs for one operation.
const (
	LabelOperationID = "operator.solo.hedera.com/operation-id"
	LabelStep        = "operator.solo.hedera.com/step"
)

// Step label values.
const (
	StepPrepare = "prepare"
	StepFreeze  = "freeze"
	StepExecute = "execute"
	StepAbort   = "abort"
)

// Condition type constants for the NetworkUpgrade* CRs.
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
