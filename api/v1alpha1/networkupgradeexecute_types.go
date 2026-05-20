// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ExecutePhase is the phase enum for NetworkUpgradeExecute. Unlike Prepare and
// Freeze it has intermediate states owned by the daemon and the operator.
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

// NetworkUpgradeExecuteSpec defines the desired state of a NetworkUpgradeExecute.
type NetworkUpgradeExecuteSpec struct {
	// Orbit is the deployment scope (typically the namespace name).
	Orbit string `json:"orbit"`

	// OperationID links this CR to the associated Prepare and Freeze CRs
	// through the shared operation-id label.
	OperationID string `json:"operationId"`

	// Images is copied verbatim from the associated Prepare CR's spec.
	Images UpgradeImagesSpec `json:"images"`

	// SelectedSources is copied verbatim from the associated Prepare CR's
	// status. The reconciler may write an updated copy to status on failover.
	SelectedSources SelectedSources `json:"selectedSources"`
}

// NetworkUpgradeExecuteStatus is the observed state of a NetworkUpgradeExecute.
type NetworkUpgradeExecuteStatus struct {
	// +optional
	Phase ExecutePhase `json:"phase,omitempty"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SelectedSources reflects last-known-good selection after any execute-time
	// failover. Empty until the reconciler revalidates.
	// +optional
	SelectedSources *SelectedSources `json:"selectedSources,omitempty"`

	// UpgradedNodes is the list of node IDs whose pods have come back Ready
	// after the image patch.
	// +optional
	UpgradedNodes []string `json:"upgradedNodes,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nux
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkUpgradeExecute is the Schema for the networkupgradeexecutes API.
type NetworkUpgradeExecute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeExecuteSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeExecuteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkUpgradeExecuteList contains a list of NetworkUpgradeExecute.
type NetworkUpgradeExecuteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeExecute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeExecute{}, &NetworkUpgradeExecuteList{})
}
