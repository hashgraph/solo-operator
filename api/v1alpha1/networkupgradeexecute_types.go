// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ExecutePhase is the phase enum for NetworkUpgradeExecute.
// +kubebuilder:validation:Enum=Pending;ReadyForProvisionerDaemon;PendingInfraUpgrade;PendingNodeUpgrade;Succeeded;Failed
type ExecutePhase string

const (
	ExecutePhasePending                   ExecutePhase = "Pending"
	ExecutePhaseReadyForProvisionerDaemon ExecutePhase = "ReadyForProvisionerDaemon"
	ExecutePhasePendingInfraUpgrade       ExecutePhase = "PendingInfraUpgrade"
	ExecutePhasePendingNodeUpgrade        ExecutePhase = "PendingNodeUpgrade"
	ExecutePhaseSucceeded                 ExecutePhase = "Succeeded"
	ExecutePhaseFailed                    ExecutePhase = "Failed"
)

// NetworkUpgradeExecuteSpec is immutable after creation; the CEL rule below
// rejects any spec mutation at admission time.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable after creation"
type NetworkUpgradeExecuteSpec struct {
	Orbit string `json:"orbit"`

	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	OperationID string `json:"operationId"`

	// ConsensusCapsule is the name of the ConsensusCapsule (this node) this CR targets.
	// The operator scopes the image-verification pull-secret keychain to this capsule so
	// verification uses the same credentials the node's pod pulls with.
	// +kubebuilder:validation:MinLength=1
	ConsensusCapsule string `json:"consensusCapsule"`

	Images UpgradeImagesSpec `json:"images"`

	SelectedSources SelectedSources `json:"selectedSources"`

	// Provisioner carries the provisioner daemon and CLI versions declared in
	// the upgrade package's infrastructure-versions.yaml. The execute
	// reconciler compares these against the Orbit CR's annotations to decide
	// whether infra-level work requires a full pod scale-down.
	// +optional
	Provisioner *DeclaredProvisionerVersions `json:"provisioner,omitempty"`
}

// DeclaredProvisionerVersions holds the provisioner versions declared in the
// upgrade package's infrastructure-versions.yaml (HIP-1494).
type DeclaredProvisionerVersions struct {
	// DaemonVersion is the provisioner daemon version from the manifest.
	// +optional
	DaemonVersion string `json:"daemonVersion,omitempty"`
	// CLIVersion is the provisioner CLI version from the manifest.
	// +optional
	CLIVersion string `json:"cliVersion,omitempty"`
}

type NetworkUpgradeExecuteStatus struct {
	// +optional
	Phase ExecutePhase `json:"phase,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// +optional
	NodeUpgradeStartTime *metav1.Time `json:"nodeUpgradeStartTime,omitempty"`
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// +optional
	SelectedSources *SelectedSources `json:"selectedSources,omitempty"`
	// FirewallPlan is copied verbatim from the Prepare CR's status when this CR is
	// created. The provisioner daemon only reads it; it reports progress through the
	// FirewallApplied condition.
	// +optional
	FirewallPlan *FirewallPlan `json:"firewallPlan,omitempty"`
	// +optional
	UpgradedNodes []string `json:"upgradedNodes,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nux
// +kubebuilder:selectablefield:JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="ConsensusNode",type="string",JSONPath=".spec.consensusNode"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type NetworkUpgradeExecute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeExecuteSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeExecuteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NetworkUpgradeExecuteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeExecute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeExecute{}, &NetworkUpgradeExecuteList{})
}
