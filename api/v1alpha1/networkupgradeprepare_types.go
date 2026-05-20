// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NetworkUpgradePrepareSpec defines the desired state of a NetworkUpgradePrepare.
type NetworkUpgradePrepareSpec struct {
	// Orbit is the deployment scope (typically the namespace name).
	Orbit string `json:"orbit"`

	// OperationID is the opaque identifier shared by all CRs of one upgrade
	// operation (typically "upgrade-<timestamp>-<version>").
	OperationID string `json:"operationId"`

	// Images is the verbatim images section of consensus-node-components.yaml.
	Images UpgradeImagesSpec `json:"images"`
}

// NetworkUpgradePrepareStatus is the observed state of a NetworkUpgradePrepare.
type NetworkUpgradePrepareStatus struct {
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

	// SelectedSources records the chosen registry per present-and-enabled
	// component on successful reconciliation. Populated only when Phase=Succeeded.
	// +optional
	SelectedSources *SelectedSources `json:"selectedSources,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nup
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NetworkUpgradePrepare is the Schema for the networkupgradeprepares API.
type NetworkUpgradePrepare struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkUpgradePrepareSpec   `json:"spec,omitempty"`
	Status NetworkUpgradePrepareStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkUpgradePrepareList contains a list of NetworkUpgradePrepare.
type NetworkUpgradePrepareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkUpgradePrepare `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkUpgradePrepare{}, &NetworkUpgradePrepareList{})
}
