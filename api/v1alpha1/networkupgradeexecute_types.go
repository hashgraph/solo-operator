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

	// +kubebuilder:validation:MinLength=1
	ConsensusNode string `json:"consensusNode"`

	Images UpgradeImagesSpec `json:"images"`

	SelectedSources SelectedSources `json:"selectedSources"`

	// RequiresInfraAccess indicates that the upgrade package declares
	// provisioner daemon or CLI versions (in infrastructure-versions.yaml)
	// that differ from the versions currently annotated on the Orbit CR.
	// When true the execute reconciler uses the scale-down-to-0 /
	// scale-up-to-1 sequence regardless of the Orbit-level rollingRestart
	// setting. The UC sets this automatically by comparing the manifest
	// versions against the Orbit annotations.
	// +kubebuilder:default=false
	// +optional
	RequiresInfraAccess bool `json:"requiresInfraAccess,omitempty"`
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
