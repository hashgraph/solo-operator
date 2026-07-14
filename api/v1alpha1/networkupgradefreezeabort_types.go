// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NetworkUpgradeFreezeAbortSpec is immutable after creation; the CEL rule below
// rejects any spec mutation at admission time.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable after creation"
type NetworkUpgradeFreezeAbortSpec struct {
	Orbit string `json:"orbit"`

	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	OperationID string `json:"operationId"`
}

type NetworkUpgradeFreezeAbortStatus struct {
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
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nua
// +kubebuilder:selectablefield:JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type NetworkUpgradeFreezeAbort struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeFreezeAbortSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeFreezeAbortStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NetworkUpgradeFreezeAbortList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeFreezeAbort `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeFreezeAbort{}, &NetworkUpgradeFreezeAbortList{})
}
