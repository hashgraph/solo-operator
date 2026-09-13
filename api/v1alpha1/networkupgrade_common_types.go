// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "github.com/automa-saga/errx"

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
	ConsensusNode ImageComponentSpec `json:"consensusNode"`
	// +optional
	RecordStreamUploader *ImageComponentSpec `json:"recordStreamUploader,omitempty"`
	// +optional
	EventStreamUploader *ImageComponentSpec `json:"eventStreamUploader,omitempty"`
	// +optional
	BlockStreamUploader *ImageComponentSpec `json:"blockStreamUploader,omitempty"`
	// +optional
	BackupUploader *ImageComponentSpec `json:"backupUploader,omitempty"`
	// +optional
	UC *ImageComponentSpec `json:"uc,omitempty"`
}

// Component returns the named component's spec and whether it is present. The
// optional sidecar components report false when unset.
func (s UpgradeImagesSpec) Component(name ComponentName) (ImageComponentSpec, bool) {
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
		return ImageComponentSpec{}, false
	}
}

func derefComponent(c *ImageComponentSpec) (ImageComponentSpec, bool) {
	if c == nil {
		return ImageComponentSpec{}, false
	}
	return *c, true
}

// ImageComponentSpec is the registry-selection input for one component's image:
// the candidate registries, their expected layer hashes, and the iteration
// strategy. It is the shared model used by both first-time capsule deployment
// (converted from SoftwareVersionSource) and network upgrades, so the same
// selection + verification + digest-pinning engine serves both paths.
type ImageComponentSpec struct {
	Version string `json:"version"`

	// Enabled=false means present-but-remove the sidecar from the consensus pod on execute.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	Deterministic ImageDeterministicSpec `json:"deterministic"`

	// +kubebuilder:validation:MinItems=1
	Registries []ImageRegistrySpec `json:"registries"`

	// SelectionStrategy chooses the order this component's registries are probed
	// in: "Sequential" honors the listed order (primary first, the rest as
	// fallbacks); "Random" spreads pull traffic across registries. When unset it
	// falls back to the operator's --registry-order default.
	// +kubebuilder:validation:Enum=Random;Sequential
	// +optional
	SelectionStrategy string `json:"selectionStrategy,omitempty"`
}

// ImageDeterministicSpec carries layer hashes when Supported=true.
// When Supported=false, hashes live per-registry on ImageRegistrySpec instead.
type ImageDeterministicSpec struct {
	Supported bool `json:"supported"`
	// Keyed by platform ("linux/amd64", "linux/arm64").
	// +optional
	LayerHashes map[string][]Sha256Digest `json:"layerHashes,omitempty"`
}

type ImageRegistrySpec struct {
	// Image is a "registry[:port]/repository[:tag][@sha256:digest]" reference,
	// e.g. "ghcr.io/hashgraph/consensus-node:0.75.0" or
	// "registry.example.com:5000/consensus-node@sha256:<64 hex chars>".
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?(:[0-9]+)?(/[a-zA-Z0-9._-]+)+(:[a-zA-Z0-9._-]+)?(@sha256:[a-f0-9]{64})?$`
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

// FirstUnpinned returns the name of the first present (non-nil) selected source that
// lacks a verified content digest, scanning in AllUpgradeComponents order, and true when
// one is found. It returns ("", false) when every present source is digest-pinned — which
// includes the empty and nil cases, since there is then nothing unpinned. This is the
// single predicate over the component set shared by the operator's Freeze prefetch-skip
// gate (allSourcesPinned) and the UC's digest-pin refusal (ensureDigestsPinned), so adding
// a component to AllUpgradeComponents extends both checks automatically.
func (s *SelectedSources) FirstUnpinned() (ComponentName, bool) {
	if s == nil {
		return "", false
	}
	for _, name := range AllUpgradeComponents {
		if src := s.Get(name); src != nil && src.Digest == "" {
			return name, true
		}
	}
	return "", false
}

// HasResolved reports whether at least one selected source is present (non-nil).
func (s *SelectedSources) HasResolved() bool {
	if s == nil {
		return false
	}
	for _, name := range AllUpgradeComponents {
		if s.Get(name) != nil {
			return true
		}
	}
	return false
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

// Annotations written by the provisioner daemon and CLI on the Orbit CR to
// advertise their running versions. The execute reconciler compares these
// against the declared versions on the Execute CR spec to determine whether
// infra-level work requires a full pod scale-down.
const (
	AnnotationProvisionerDaemonVersion = "operator.solo.hiero.org/provisioner-daemon-version"
	AnnotationProvisionerCLIVersion    = "operator.solo.hiero.org/provisioner-cli-version"
)

const (
	LabelPrefetchRole = "operator.solo.hiero.org/role"
	RolePrefetch      = "prefetch"
	LabelCapsule      = "operator.solo.hiero.org/capsule"
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
	CondConfigCRsApplied           = "ConfigCRsApplied"
	CondDaemonResult               = "DaemonResult"
	CondConsensusCapsulePatched    = "ConsensusCapsulePatched"
	CondCapsulesOnline             = "CapsulesOnline"
	CondNodesUpgraded              = "NodesUpgraded"
	CondNodeActiveDeadlineExceeded = "NodeActiveDeadlineExceeded"

	// Prepare — prefetch
	CondImagePrefetchCreated   = "ImagePrefetchCreated"
	CondImagePrefetchCompleted = "ImagePrefetchCompleted"

	// Freeze — prefetch
	CondImagePrefetched = "ImagePrefetched"

	// Abort
	CondAbortProcessed = "AbortProcessed"

	// Shared across all NetworkUpgrade* CRs.
	CondOperationIDLabelValid = "OperationIDLabelValid"
	CondStepLabelValid        = "StepLabelValid"
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

	ReasonScaledDown                 errx.Reason = "ScaledDown"
	ReasonReadyForDaemon             errx.Reason = "ReadyForProvisionerDaemon"
	ReasonPatchApplied               errx.Reason = "PatchApplied"
	ReasonCapsulesOnline             errx.Reason = "CapsulesOnline"
	ReasonPodReady                   errx.Reason = "PodReady"
	ReasonUpgradeSucceeded           errx.Reason = "UpgradeSucceeded"
	ReasonUpgradeFailed              errx.Reason = "UpgradeFailed"
	ReasonNodeActiveDeadlineExceeded errx.Reason = "NodeActiveDeadlineExceeded"

	ReasonPrefetchCreated      errx.Reason = "PrefetchCreated"
	ReasonPrefetchCompleted    errx.Reason = "PrefetchCompleted"
	ReasonPrefetchFailed       errx.Reason = "PrefetchFailed"
	ReasonPrefetchPending      errx.Reason = "PrefetchPending"
	ReasonPrefetchSkipped      errx.Reason = "PrefetchSkipped"
	ReasonPrefetchInconclusive errx.Reason = "PrefetchInconclusive"
	ReasonPrefetchDisabled     errx.Reason = "PrefetchDisabled"

	ReasonAbortSuccess     errx.Reason = "Success"
	ReasonAbortAfterFreeze errx.Reason = "AbortAfterFreeze"

	ReasonOperationIDLabelInvalid errx.Reason = "OperationIDLabelInvalid"
	ReasonStepLabelInvalid        errx.Reason = "StepLabelInvalid"
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
	EventRegistryDenied        errx.Reason = "RegistryDenied"
	EventRegistryFailover      errx.Reason = "RegistryFailover"
	EventComponentSkipped      errx.Reason = "ComponentSkipped"
	EventSidecarDisabled       errx.Reason = "SidecarDisabled"

	EventPodScaledDown              errx.Reason = "PodScaledDown"
	EventReadyForDaemon             errx.Reason = "ReadyForProvisionerDaemon"
	EventPendingInfraUpgrade        errx.Reason = "PendingInfraUpgrade"
	EventPendingNodeUpgrade         errx.Reason = "PendingNodeUpgrade"
	EventCapsulePatched             errx.Reason = "CapsulePatched"
	EventCapsulesOnline             errx.Reason = "CapsulesOnline"
	EventUpgradeSucceeded           errx.Reason = "UpgradeSucceeded"
	EventUpgradeFailed              errx.Reason = "UpgradeFailed"
	EventNodeActiveDeadlineExceeded errx.Reason = "NodeActiveDeadlineExceeded"

	EventPrefetchCreated   errx.Reason = "PrefetchCreated"
	EventPrefetchCompleted errx.Reason = "PrefetchCompleted"
	EventPrefetchFailed    errx.Reason = "PrefetchFailed"

	EventAbortProcessed   errx.Reason = "AbortProcessed"
	EventAbortAfterFreeze errx.Reason = "AbortAfterFreeze"

	EventConfigCRReconciled errx.Reason = "ConfigCRReconciled"
	EventConfigCRFailed     errx.Reason = "ConfigCRFailed"

	EventInvalidGenesisJSON errx.Reason = "InvalidGenesisJSON"

	EventOperationIDLabelInvalid errx.Reason = "OperationIDLabelInvalid"
	EventStepLabelInvalid        errx.Reason = "StepLabelInvalid"

	// ConsensusCapsule phase-transition events (posted by the capsule controller).
	EventConsensusNodeActive  errx.Reason = "ConsensusNodeActive"
	EventConsensusNodeRunning errx.Reason = "ConsensusNodeRunning"
	EventConsensusNodeFailed  errx.Reason = "ConsensusNodeFailed"
	EventConsensusNodeStarted errx.Reason = "ConsensusNodeStarted"
	EventConsensusNodeStopped errx.Reason = "ConsensusNodeStopped"
	// EventUCSidecarRequired is posted when a ConsensusCapsule is missing the
	// mandatory UC sidecar (terminal configuration error).
	EventUCSidecarRequired errx.Reason = "UCSidecarRequired"
	// EventImageDigestRequired is posted when the Orbit sets requireDigestOnDeploy but one or
	// more of the ConsensusCapsule's running images (the consensus node or an enabled
	// sidecar/uploader) is not digest-pinned (terminal config error).
	EventImageDigestRequired errx.Reason = "ImageDigestRequired"
	// EventImageRequired is posted when a ConsensusCapsule has a running container (the
	// consensus node or an enabled sidecar/uploader) with no declared image; the operator
	// provides no built-in defaults (terminal config error).
	EventImageRequired errx.Reason = "ImageRequired"
	// EventImageSourceInvalid is posted when a container's SoftwareVersionSource is malformed
	// — no candidate repositories or no usable per-platform layer hashes (terminal config error).
	EventImageSourceInvalid errx.Reason = "ImageSourceInvalid"
)
