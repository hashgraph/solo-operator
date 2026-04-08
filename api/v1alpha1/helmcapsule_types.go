// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HelmComponent identifies a known Hedera ecosystem component managed by a HelmCapsule.
//
// When Component is set together with OrbitRef, the controller automatically derives
// and injects Helm values from the surrounding network topology (Orbit CR and
// ConsensusCapsules in the same namespace).
//
// # Value merge order (lowest → highest priority)
//
//  1. Auto-injected values  — derived from OrbitRef + ConsensusCapsules (see table below)
//  2. ValuesFrom            — values loaded from referenced ConfigMaps or Secrets
//  3. Values                — inline overrides specified directly in the spec (always wins)
//
// # Auto-injected values per component
//
// mirror (chart: hedera-mirror):
//   - hedera.mirror.importer.consensusNodeEndpoints  list of "host:port" built from
//     ConsensusCapsule.status.addressBookEntry.{internalHostname,internalPort}
//   - hedera.mirror.importer.network                 set to the Orbit name
//
// relay (chart: hedera-json-rpc-relay) and relay-websocket (chart: hedera-json-rpc-relay-websocket):
//   - config.HEDERA_NETWORK  JSON map {"host:port": "accountId"} built from
//     ConsensusCapsule status.addressBookEntry entries
//   - config.MIRROR_NODE_URL derived as
//     "http://mirror.<namespace>.svc.cluster.local"
//   - config.CHAIN_ID        from Orbit
//     spec.consensus.networkSettings["ledger.id"] when present
//
// explorer (chart: hiero-explorer-chart):
//   - config  JSON array with a single network entry:
//     [{"name":"<orbit-name>","displayName":"<orbit-name>","url":"/","ledgerID":"<ledger.id>"}]
//     Injected only when OrbitRef is set; otherwise the user must supply config via Values.
//
// block-node (chart: block-node-server):
//   - blockNode.config.BLOCK_NODE_ENDPOINTS  comma-separated "host:port" list derived
//     from ConsensusCapsule status.addressBookEntry entries
//   - blockNode.config.CONSENSUS_NODE_COUNT  number of ConsensusCapsules found in the
//     namespace that belong to the referenced Orbit
//
// +kubebuilder:validation:Enum=mirror;explorer;relay;relay-websocket;block-node
type HelmComponent string

const (
	HelmComponentMirror         HelmComponent = "mirror"
	HelmComponentExplorer       HelmComponent = "explorer"
	HelmComponentRelay          HelmComponent = "relay"
	HelmComponentRelayWebSocket HelmComponent = "relay-websocket"
	HelmComponentBlockNode      HelmComponent = "block-node"
)

// HelmCapsuleSpec defines the desired state of a HelmCapsule.
// +kubebuilder:validation:XValidation:rule="!has(self.component) || self.component == 'explorer' || has(self.orbitRef)",message="orbitRef is required when component is mirror, relay, relay-websocket, or block-node"
type HelmCapsuleSpec struct {
	// Chart defines the Helm chart source to install.
	Chart HelmChartSpec `json:"chart"`

	// ReleaseName overrides the Helm release name.
	// Defaults to the HelmCapsule's metadata.name when omitted.
	// +kubebuilder:validation:MinLength=1
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// Component identifies a known Hedera ecosystem component.
	// When set together with OrbitRef the operator auto-injects network-topology
	// values before any ValuesFrom or Values entries are merged.
	// See the HelmComponent type documentation for the full list of injected
	// values per component.
	// +optional
	Component *HelmComponent `json:"component,omitempty"`

	// OrbitRef references the cluster-scoped Orbit CR by name.
	// The Orbit and its ConsensusCapsules are the source of auto-injected values
	// when Component is set. Required for mirror, relay, and block-node components.
	// +optional
	OrbitRef *corev1.LocalObjectReference `json:"orbitRef,omitempty"`

	// ValuesFrom loads Helm values from ConfigMaps or Secrets in the same namespace.
	// Multiple sources are merged in the order listed. Values from later entries
	// override earlier ones. All ValuesFrom values are overridden by the inline Values.
	// +optional
	ValuesFrom []HelmValuesReference `json:"valuesFrom,omitempty"`

	// Values are inline, user-supplied Helm value overrides serialised as JSON/YAML.
	// These are merged last and take the highest priority, overriding both
	// auto-injected values and anything from ValuesFrom.
	// +optional
	Values *apiextv1.JSON `json:"values,omitempty"`

	// Timeout for Helm install/upgrade operations.
	// Defaults to 5m (same as the Helm CLI default).
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Wait waits until all resources are ready before marking the release successful.
	// Helm default: false.
	// +optional
	Wait bool `json:"wait,omitempty"`

	// Atomic rolls back the release on failure (implies Wait=true).
	// Helm default: false.
	// +optional
	Atomic bool `json:"atomic,omitempty"`

	// CleanupOnFail deletes newly-created resources when an upgrade fails.
	// Helm default: false.
	// +optional
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`

	// MaxHistory limits the number of revisions Helm keeps for this release.
	// 0 means unlimited. Helm CLI default: 10.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxHistory int `json:"maxHistory,omitempty"`

	// DisableHooks skips running pre/post Helm hooks during install and upgrade.
	// Helm default: false.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// KeepHistory preserves Helm release history when the HelmCapsule is deleted.
	// Equivalent to helm uninstall --keep-history.
	// Helm default: false.
	// +optional
	KeepHistory bool `json:"keepHistory,omitempty"`
}

// HelmChartSpec identifies the Helm chart to install.
type HelmChartSpec struct {
	// Repo is the chart repository URL.
	// Use the oci:// scheme for OCI registries (recommended).
	// For https:// repos the operator constructs a direct .tgz URL in the form
	// "<repo>/<chart>-<version>.tgz" — this works for repos that serve tarballs
	// at a predictable path but does NOT support index-based Helm repos
	// (those require `helm repo add` which is not performed by the operator).
	// Example: "oci://ghcr.io/hashgraph/solo-charts"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^(oci|https)://`
	Repo string `json:"repo"`

	// Chart is the name of the chart within the repository.
	// Example: "hedera-mirror"
	// +kubebuilder:validation:MinLength=1
	Chart string `json:"chart"`

	// Version is the chart version to install (semver).
	// Example: "0.129.1"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^v?[0-9]+\.[0-9]+\.[0-9]`
	Version string `json:"version"`

	// CredentialsRef names a Secret in the same namespace as the HelmCapsule
	// that contains chart repository credentials.
	// The Secret must have "username" and "password" string keys.
	// Required for private chart repositories; omit for public repos.
	// +optional
	CredentialsRef *corev1.LocalObjectReference `json:"credentialsRef,omitempty"`
}

// HelmValuesReference points to a ConfigMap or Secret key that contains a Helm values YAML.
type HelmValuesReference struct {
	// Kind of the values source.
	// +kubebuilder:validation:Enum=ConfigMap;Secret
	Kind string `json:"kind"`

	// Name of the ConfigMap or Secret in the same namespace as the HelmCapsule.
	Name string `json:"name"`

	// ValuesKey is the key within the ConfigMap or Secret whose value is a Helm
	// values YAML document. Defaults to "values.yaml" when omitted.
	// +kubebuilder:validation:MinLength=1
	// +optional
	ValuesKey string `json:"valuesKey,omitempty"`

	// Optional silently skips this reference if the ConfigMap or Secret does not
	// exist. When false (the default) a missing source causes reconciliation to fail.
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// HelmCapsuleStatus defines the observed state of a HelmCapsule.
type HelmCapsuleStatus struct {
	// Phase is the current lifecycle phase of the Helm release.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// ObservedGeneration is the .metadata.generation last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReleaseVersion is the Helm revision number currently deployed (increments on each upgrade).
	// +optional
	ReleaseVersion int `json:"releaseVersion,omitempty"`

	// InstalledChart is the chart name + version string of the currently deployed release,
	// e.g. "hedera-mirror-0.129.1".
	// +optional
	InstalledChart string `json:"installedChart,omitempty"`

	// ReleaseStatus is the Helm release status as reported by Helm after the last
	// install or upgrade (e.g. "deployed", "failed", "pending-install").
	// +optional
	ReleaseStatus string `json:"releaseStatus,omitempty"`

	// ValuesHash is a SHA-256 digest of the chart spec and the fully-merged values
	// map at the time of the last successful install or upgrade.
	// The controller compares this against the current desired state on every
	// reconcile; a mismatch triggers helm upgrade. This covers both chart version
	// changes and values changes (inline Values, ValuesFrom sources, or
	// auto-injected topology values).
	// +optional
	ValuesHash string `json:"valuesHash,omitempty"`

	// Conditions holds the latest observations of the HelmCapsule state.
	// Ready=True means the release is deployed and up-to-date.
	// Ready=False carries a Reason: ChartFetchFailed, HelmFailed, or ValuesBuildFailed.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

const (
	HelmCapsuleReasonDeployed             = "Deployed"
	HelmCapsuleReasonChartFetchFailed     = "ChartFetchFailed"
	HelmCapsuleReasonHelmFailed           = "HelmFailed"
	HelmCapsuleReasonValuesBuildFailed    = "ValuesBuildFailed"
	HelmCapsuleReasonReleaseNameConflict  = "ReleaseNameConflict"
	HelmCapsuleReasonProgressing          = "Progressing"
	HelmCapsuleReasonReleaseNotReady      = "ReleaseNotReady"
	HelmCapsuleReasonUnknownReleaseStatus = "UnknownReleaseStatus"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=helmcap
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component"
// +kubebuilder:printcolumn:name="Chart",type="string",JSONPath=".spec.chart.chart"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.chart.version"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="HelmStatus",type="string",JSONPath=".status.releaseStatus"
// +kubebuilder:printcolumn:name="Release",type="integer",JSONPath=".status.releaseVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=helmcapsules/finalizers,verbs=update

// HelmCapsule manages a Helm release as part of a solo-operator network.
// It supports both generic chart deployments and first-class Hedera ecosystem
// components (mirror, relay, explorer, block-node) with automatic value
// injection from the network topology.
type HelmCapsule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmCapsuleSpec   `json:"spec,omitempty"`
	Status HelmCapsuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HelmCapsuleList contains a list of HelmCapsule resources.
type HelmCapsuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmCapsule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmCapsule{}, &HelmCapsuleList{})
}
