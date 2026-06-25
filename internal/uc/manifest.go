// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"errors"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// Deployment-package layout the UC reads. The marker directory is the root of the
// extracted package (`.../data/upgrade/current`), so the machine-readable
// manifests live under its `manifests/` subdirectory (HIP-1494 §"Package
// Directory Structure").
const (
	manifestsSubdir        = "manifests"
	componentsManifestFile = "consensus-node-components.yaml"

	// componentsSchemaVersion is the only consensus-node-components.yaml schema
	// version this UC understands. schemaVersion is an integer (HIP-1494); an
	// unknown value is a ManifestInvalid condition and halts processing. Within a
	// known version, unknown fields are ignored (lenient parsing) so additive
	// changes do not require a UC upgrade.
	componentsSchemaVersion = 1
)

// errManifestInvalid marks a manifest the UC refuses to process (unknown
// schemaVersion or a missing required field). It maps to the ManifestInvalid
// condition: the handler halts without creating a CR rather than retrying, since
// re-reading the same bytes will not fix it.
var errManifestInvalid = errors.New("manifest invalid")

// componentsManifest is the thin wrapper around consensus-node-components.yaml.
// The `images:` block maps 1:1 onto UpgradeImagesSpec (HIP-1494) — reused
// verbatim so the parsed value drops straight into NetworkUpgradePrepare.spec.
// Only schemaVersion + the wrapper are new here.
type componentsManifest struct {
	SchemaVersion int                                `json:"schemaVersion"`
	Images        operatorv1alpha1.UpgradeImagesSpec `json:"images"`
}

// parseComponentsManifest reads and validates consensus-node-components.yaml.
// A read error is returned as-is (transient — the caller may retry on the next
// poll). A semantic problem (unknown schemaVersion, missing consensus-node
// version) is wrapped with errManifestInvalid so the caller can map it to a
// terminal ManifestInvalid halt. Parsing is lenient: unknown fields within a
// known schemaVersion are silently ignored (sigs.k8s.io/yaml does not reject
// them), per HIP-1494's backwards-compatibility rule.
func parseComponentsManifest(path string) (*componentsManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read components manifest %s: %w", path, err)
	}

	var m componentsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		// A malformed YAML body cannot be parsed regardless of schema; treat it as
		// invalid (halt), not transient.
		return nil, fmt.Errorf("%w: parse %s: %v", errManifestInvalid, path, err)
	}

	if m.SchemaVersion != componentsSchemaVersion {
		return nil, fmt.Errorf("%w: unknown schemaVersion %d in %s (supported: %d)",
			errManifestInvalid, m.SchemaVersion, path, componentsSchemaVersion)
	}
	if m.Images.ConsensusNode.Version == "" {
		return nil, fmt.Errorf("%w: images.consensusNode.version is required in %s",
			errManifestInvalid, path)
	}
	return &m, nil
}
