// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NetworkUpgradeFreezeSpec is immutable after creation; the CEL rule below
// rejects any spec mutation at admission time.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable after creation"
type NetworkUpgradeFreezeSpec struct {
	Orbit string `json:"orbit"`

	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	OperationID string `json:"operationId"`

	ScheduledTime metav1.Time `json:"scheduledTime"`

	Images UpgradeImagesSpec `json:"images"`

	SelectedSources SelectedSources `json:"selectedSources"`
}

type NetworkUpgradeFreezeStatus struct {
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Unknown
	Phase Phase `json:"phase,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// +optional
	SelectedSources *SelectedSources `json:"selectedSources,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nuf
// +kubebuilder:selectablefield:JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Scheduled",type="string",JSONPath=".spec.scheduledTime"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type NetworkUpgradeFreeze struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeFreezeSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeFreezeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NetworkUpgradeFreezeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeFreeze `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeFreeze{}, &NetworkUpgradeFreezeList{})
}
