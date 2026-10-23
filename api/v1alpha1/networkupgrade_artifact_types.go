// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// ArtifactCandidateRoster is the name of the candidate-roster artifact: the
	// roster the consensus node exports at PREPARE_UPGRADE (a JSON-encoded
	// Network message). The UC forwards it verbatim; the operator interprets it.
	ArtifactCandidateRoster = "candidate-roster.json"

	// MaxArtifactContentBytes caps one artifact's content. A CR is stored in etcd
	// (about 1.5 MiB per object) and also carries the image specs and status, so
	// the cap leaves headroom; a roster of dozens of nodes is far below it.
	MaxArtifactContentBytes = 512 * 1024
)

// UpgradeArtifact is an opaque file the UC read from the node's upgrade state and
// forwarded verbatim in the Prepare CR. The UC never interprets Content; the
// operator decides what each named artifact means and what to do with it, so a new
// artifact is a new Name, not a new CR kind or a new UC permission.
type UpgradeArtifact struct {
	// Name identifies the artifact (for example "candidate-roster.json").
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`
	Name string `json:"name"`

	// Sha256 is the digest of Content's bytes, computed by the UC when it read the
	// file. The operator verifies it before using the content.
	Sha256 Sha256Digest `json:"sha256"`

	// Content is the file's text, unmodified (UTF-8; the UC rejects binary files).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=524288
	Content string `json:"content"`
}
