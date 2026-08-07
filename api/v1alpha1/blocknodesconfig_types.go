// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BlockNodesConfigSpec struct {
	// Raw block-nodes.json content, staged verbatim into the managed ConfigMap.
	// The consensus-node deployment package delivers this per node under
	// block-nodes/config/block-nodes-<NODE_ID>.json (HIP-1494); the UC selects the
	// file matching this node's NODE_ID and stores it here unchanged, so the format
	// is exactly what the consensus node expects.
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`

	// Set so the Orbit can cascade-delete this resource.
	// +optional
	Orbit string `json:"orbit,omitempty"`
}

type BlockNodesConfigStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

const BlockNodesConfigConditionTypeValid = "Valid"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=blocknodescfg
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type BlockNodesConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlockNodesConfigSpec   `json:"spec,omitempty"`
	Status BlockNodesConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type BlockNodesConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockNodesConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlockNodesConfig{}, &BlockNodesConfigList{})
}
