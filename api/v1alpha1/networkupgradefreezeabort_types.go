// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NetworkUpgradeFreezeAbortSpec defines the desired state of a NetworkUpgradeFreezeAbort.
type NetworkUpgradeFreezeAbortSpec struct {
	// Orbit is the deployment scope (typically the namespace name).
	Orbit string `json:"orbit"`

	// OperationID links this CR to the associated Prepare/Freeze CR(s) being
	// aborted, through the shared operation-id label.
	OperationID string `json:"operationId"`
}

// NetworkUpgradeFreezeAbortStatus is the observed state of a NetworkUpgradeFreezeAbort.
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
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkUpgradeFreezeAbort is the Schema for the networkupgradefreezeaborts API.
type NetworkUpgradeFreezeAbort struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradeFreezeAbortSpec   `json:"spec,omitempty"`
	Status NetworkUpgradeFreezeAbortStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkUpgradeFreezeAbortList contains a list of NetworkUpgradeFreezeAbort.
type NetworkUpgradeFreezeAbortList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradeFreezeAbort `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradeFreezeAbort{}, &NetworkUpgradeFreezeAbortList{})
}
