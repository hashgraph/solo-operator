// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "github.com/hashgraph/solo-operator/internal/errx"

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

// Component returns the named component's spec and whether it is present. The
// optional sidecar components report false when unset.
func (s UpgradeImagesSpec) Component(name ComponentName) (UpgradeComponentSpec, bool) {
	switch name {
	case ComponentConsensusNode:
		return s.ConsensusNode, true
	case ComponentRecordStreamUploader:
		return derefComponent(s.RecordStreamUploader)
	case ComponentEventStreamUploader:
		return derefComponent(s.EventStreamUploader)
	case ComponentBlockStreamUploader:
		return derefComponent(s.BlockStreamUploader)
	case ComponentBackupUploader:
		return derefComponent(s.BackupUploader)
	case ComponentUC:
		return derefComponent(s.UC)
	default:
		return UpgradeComponentSpec{}, false
	}
}

func derefComponent(c *UpgradeComponentSpec) (UpgradeComponentSpec, bool) {
	if c == nil {
		return UpgradeComponentSpec{}, false
	}
	return *c, true
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

func (s *SelectedSources) field(name ComponentName) **SelectedSource {
	switch name {
	case ComponentConsensusNode:
		return &s.ConsensusNode
	case ComponentRecordStreamUploader:
		return &s.RecordStreamUploader
	case ComponentEventStreamUploader:
		return &s.EventStreamUploader
	case ComponentBlockStreamUploader:
		return &s.BlockStreamUploader
	case ComponentBackupUploader:
		return &s.BackupUploader
	case ComponentUC:
		return &s.UC
	default:
		return nil
	}
}

func (s *SelectedSources) Get(name ComponentName) *SelectedSource {
	if f := s.field(name); f != nil {
		return *f
	}
	return nil
}

func (s *SelectedSources) Set(name ComponentName, src *SelectedSource) {
	if f := s.field(name); f != nil {
		*f = src
	}
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
	// LayerHashes stores the per-platform hashes used during initial selection so
	// re-validation doesn't need to re-scan comp.Registries. Keyed by platform.
	// +optional
	LayerHashes map[string][]Sha256Digest `json:"layerHashes,omitempty"`
}

// Labels linking all CRs of one upgrade operation.
const (
	LabelOperationID = "operator.solo.hiero.org/operation-id"
	LabelStep        = "operator.solo.hiero.org/step"
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
	CondCapsulesOnline          = "CapsulesOnline"
	CondNodesUpgraded           = "NodesUpgraded"
	CondNodeStarting            = "NodeStarting"

	// Abort
	CondAbortProcessed = "AbortProcessed"

	// Shared across all NetworkUpgrade* CRs.
	CondOperationIDLabelValid = "OperationIDLabelValid"
)

// Reason constants used in conditions and events. Typed as errx.Reason so one
// identifier flows from an error through logs to Condition.Reason/Event.reason.
// The string values are a public contract — never rename, only add.
const (
	ReasonSchemaValid       errx.Reason = "SchemaValid"
	ReasonSchemaInvalid     errx.Reason = "SchemaInvalid"
	ReasonDuplicateOpID     errx.Reason = "DuplicateOperationID"
	ReasonAllImagesValid    errx.Reason = "AllImagesValid"
	ReasonSelectionComplete errx.Reason = "SelectionComplete"
	ReasonRegistryInvalid   errx.Reason = "RegistryInvalid"
	ReasonRegistryFailover  errx.Reason = "RegistryFailover"

	ReasonHealthCheckPassed errx.Reason = "HealthCheckPassed"
	ReasonPreFreezePassed   errx.Reason = "PreFreezePassed"

	ReasonScaledDown       errx.Reason = "ScaledDown"
	ReasonReadyForDaemon   errx.Reason = "ReadyForProvisionerDaemon"
	ReasonPatchApplied     errx.Reason = "PatchApplied"
	ReasonCapsulesOnline   errx.Reason = "CapsulesOnline"
	ReasonPodReady         errx.Reason = "PodReady"
	ReasonUpgradeSucceeded errx.Reason = "UpgradeSucceeded"
	ReasonUpgradeFailed    errx.Reason = "UpgradeFailed"

	ReasonAbortSuccess     errx.Reason = "Success"
	ReasonAbortAfterFreeze errx.Reason = "AbortAfterFreeze"

	ReasonOperationIDLabelInvalid errx.Reason = "OperationIDLabelInvalid"
)

// DaemonResult condition reasons, written by the provisioner daemon.
const (
	ReasonDaemonSucceeded          errx.Reason = "Succeeded"
	ReasonDaemonFileDownloadFailed errx.Reason = "FileDownloadFailed"
	ReasonDaemonFileHashMismatch   errx.Reason = "FileHashMismatch"
	ReasonDaemonInfraUpgradeFailed errx.Reason = "InfraUpgradeFailed"
	ReasonDaemonSelfUpgradeFailed  errx.Reason = "SelfUpgradeFailed"
)

// Event reasons posted on NetworkUpgrade* CRs.
const (
	EventManifestInvalid       errx.Reason = "ManifestInvalid"
	EventRegistryAuditComplete errx.Reason = "RegistryAuditComplete"
	EventRegistryUnreachable   errx.Reason = "RegistryUnreachable"
	EventRegistrySelected      errx.Reason = "RegistrySelected"
	EventRegistryInvalid       errx.Reason = "RegistryInvalid"
	EventRegistryFailover      errx.Reason = "RegistryFailover"
	EventComponentSkipped      errx.Reason = "ComponentSkipped"
	EventSidecarDisabled       errx.Reason = "SidecarDisabled"

	EventPodScaledDown       errx.Reason = "PodScaledDown"
	EventReadyForDaemon      errx.Reason = "ReadyForProvisionerDaemon"
	EventPendingInfraUpgrade errx.Reason = "PendingInfraUpgrade"
	EventPendingNodeUpgrade  errx.Reason = "PendingNodeUpgrade"
	EventCapsulePatched      errx.Reason = "CapsulePatched"
	EventCapsulesOnline      errx.Reason = "CapsulesOnline"
	EventUpgradeSucceeded    errx.Reason = "UpgradeSucceeded"
	EventUpgradeFailed       errx.Reason = "UpgradeFailed"

	EventAbortProcessed   errx.Reason = "AbortProcessed"
	EventAbortAfterFreeze errx.Reason = "AbortAfterFreeze"

	EventConfigCRReconciled errx.Reason = "ConfigCRReconciled"
	EventConfigCRFailed     errx.Reason = "ConfigCRFailed"

	EventInvalidGenesisJSON errx.Reason = "InvalidGenesisJSON"

	EventOperationIDLabelInvalid errx.Reason = "OperationIDLabelInvalid"
)
