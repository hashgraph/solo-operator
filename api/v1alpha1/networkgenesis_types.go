// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkGenesisSpec defines the desired state of a NetworkGenesis. When
// Spec.Roster is set the reconciler uses it directly; otherwise it discovers
// nodes from ConsensusCapsule CRs and Orbit external addresses.
type NetworkGenesisSpec struct {
	// +kubebuilder:validation:Required
	Orbit string `json:"orbit"`

	// +kubebuilder:validation:Required
	OperationID string `json:"operationId"`

	// +optional
	Roster []NetworkGenesisRosterEntry `json:"roster,omitempty"`

	// GenesisNetworkJSON is an optional pre-built genesis-network.json (complete Network proto,
	// JSON-encoded). When set, its content is written verbatim to the genesis-config ConfigMap,
	// skipping roster assembly and discovery. Intended for fresh cluster-only genesis; the
	// mainnet-daemon provisioning path does not populate this field.
	// +optional
	GenesisNetworkJSON string `json:"genesisNetworkJSON,omitempty"`
}

type NetworkGenesisRosterEntry struct {
	// +kubebuilder:validation:Minimum=0
	NodeID int64 `json:"nodeId"`

	// AccountID is "shard.realm.account" (e.g. "0.0.3").
	// +kubebuilder:validation:Required
	AccountID string `json:"accountId"`

	// +optional
	Description string `json:"description,omitempty"`

	// +kubebuilder:validation:Required
	GossipHost string `json:"gossipHost"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	GossipPort int32 `json:"gossipPort"`

	// +kubebuilder:validation:Required
	ServiceHost string `json:"serviceHost"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ServicePort int32 `json:"servicePort"`

	// PEM-encoded gossip signing certificate.
	// +kubebuilder:validation:Required
	GossipCaCertificate string `json:"gossipCaCertificate"`

	// PEM-encoded gRPC TLS certificate.
	// +kubebuilder:validation:Required
	GrpcCertificate string `json:"grpcCertificate"`

	// Weight defaults to 1 when zero.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Weight int64 `json:"weight,omitempty"`

	// Hex-encoded ED25519 admin public key.
	// +optional
	AdminKey string `json:"adminKey,omitempty"`
}

type NetworkGenesisStatus struct {
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// +optional
	GeneratedConfigMaps []string `json:"generatedConfigMaps,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ng
// +kubebuilder:printcolumn:name="Orbit",type="string",JSONPath=".spec.orbit"
// +kubebuilder:printcolumn:name="OperationID",type="string",JSONPath=".spec.operationId"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type NetworkGenesis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkGenesisSpec   `json:"spec,omitempty"`
	Status NetworkGenesisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type NetworkGenesisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkGenesis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkGenesis{}, &NetworkGenesisList{})
}
