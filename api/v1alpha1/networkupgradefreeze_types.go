// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NetworkUpgradeFreezeSpec defines the desired state of a NetworkUpgradeFreeze.
type NetworkUpgradeFreezeSpec struct {
	// Orbit is the deployment scope (typically the namespace name).
	Orbit string `json:"orbit"`

	// OperationID links this CR to the associated Prepare and (later) Execute
	// CRs through the shared operation-id label.
	OperationID string `json:"operationId"`

	// ScheduledTime is the planned consensus-protocol freeze time. Recorded for
	// audit; the reconciler does not gate readiness on it.
	ScheduledTime metav1.Time `json:"scheduledTime"`

	// Images is copied verbatim from the associated Prepare CR's spec at the
	// moment of Freeze CR creation. Kept here so this CR is self-contained.
	Images UpgradeImagesSpec `json:"images"`

	// SelectedSources is copied verbatim from the associated Prepare CR's
	// status. The reconciler may write an updated copy to status on failover.
	SelectedSources SelectedSources `json:"selectedSources"`
}

// NetworkUpgradeFreezeStatus is the observed state of a NetworkUpgradeFreeze.
type NetworkUpgradeFreezeStatus struct {
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SelectedSources reflects the post-revalidation selection. Empty when no
	// failover was required; otherwise the entries that changed are recorded.
	// +optional
	SelectedSources *SelectedSources `json:"selectedSources,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nuf
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Scheduled",type="string",JSONPath=".spec.scheduledTime"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkUpgradeFreeze is the Schema for the networkupgradefreezes API.
type NetworkUpgradeFreeze struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeFreezeSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeFreezeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkUpgradeFreezeList contains a list of NetworkUpgradeFreeze.
type NetworkUpgradeFreezeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeFreeze `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeFreeze{}, &NetworkUpgradeFreezeList{})
}
