// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"

	"github.com/automa-saga/errx"
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
	infraVersionsFile      = "infrastructure-versions.yaml"

	// componentsSchemaVersion is the only consensus-node-components.yaml schema
	// version this UC understands. schemaVersion is an integer (HIP-1494); an
	// unknown value is a ManifestInvalid condition and halts processing. Within a
	// known version, unknown fields are ignored (lenient parsing) so additive
	// changes do not require a UC upgrade.
	componentsSchemaVersion = 1
)

// infraVersionsManifest is the thin wrapper around infrastructure-versions.yaml
// (HIP-1494). Only the provisioner.daemon.version and provisioner.cli.version
// fields are needed — the UC compares them against the Orbit CR annotations to
// decide whether infra-level work is required.
type infraVersionsManifest struct {
	Provisioner struct {
		CLI    infraVersionEntry `json:"cli"`
		Daemon infraVersionEntry `json:"daemon"`
	} `json:"provisioner"`
}

type infraVersionEntry struct {
	Version string `json:"version"`
}

// parseInfraVersionsManifest reads infrastructure-versions.yaml from the
// upgrade package. Returns nil (not an error) when the file does not exist —
// absence means no infra upgrade is declared.
func parseInfraVersionsManifest(upgradePath string) (*infraVersionsManifest, error) {
	path := filepath.Join(upgradePath, manifestsSubdir, infraVersionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "read infra-versions manifest %s", path), ReasonManifestReadFailed),
			"Check that manifests/infrastructure-versions.yaml is readable",
		)
	}
	var m infraVersionsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errManifestInvalid.Wrap(err, "parse %s", path), ReasonManifestInvalid),
			"Regenerate the deployment package — infrastructure-versions.yaml is not valid YAML",
		)
	}
	return &m, nil
}

// declaredProvisionerVersions reads the provisioner daemon and CLI versions
// from the upgrade package's infrastructure-versions.yaml. Returns nil when
// the manifest is absent or declares no versions.
func declaredProvisionerVersions(upgradePath string) (*operatorv1alpha1.DeclaredProvisionerVersions, error) {
	infra, err := parseInfraVersionsManifest(upgradePath)
	if err != nil {
		return nil, err
	}
	if infra == nil {
		return nil, nil
	}
	daemon := infra.Provisioner.Daemon.Version
	cli := infra.Provisioner.CLI.Version
	if daemon == "" && cli == "" {
		return nil, nil
	}
	return &operatorv1alpha1.DeclaredProvisionerVersions{
		DaemonVersion: daemon,
		CLIVersion:    cli,
	}, nil
}

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
		// A read failure is transient — the file may not be fully written yet, or
		// the volume may be briefly unavailable; the caller may retry next tick.
		return nil, errx.WithHints(
			errx.WithReason(errPersist.Wrap(err, "read components manifest %s", path), ReasonManifestReadFailed),
			"Confirm the consensus node extracted the package into --upgrade-path",
			"Check that manifests/consensus-node-components.yaml exists and is readable",
		)
	}

	var m componentsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		// A malformed YAML body cannot be parsed regardless of schema; treat it as
		// invalid (halt), not transient.
		return nil, errx.WithHints(
			errx.WithReason(errManifestInvalid.Wrap(err, "parse %s", path), ReasonManifestInvalid),
			"Regenerate the deployment package — consensus-node-components.yaml is not valid YAML",
		)
	}

	if m.SchemaVersion != componentsSchemaVersion {
		return nil, errx.WithHints(
			errx.WithReason(errManifestInvalid.New("unknown schemaVersion %d in %s (supported: %d)",
				m.SchemaVersion, path, componentsSchemaVersion), ReasonManifestInvalid),
			"This UC build understands schemaVersion 1 only",
			"Upgrade the UC sidecar image to one that supports this package schemaVersion",
		)
	}
	if m.Images.ConsensusNode.Version == "" {
		return nil, errx.WithHints(
			errx.WithReason(errManifestInvalid.New("images.consensusNode.version is required in %s", path), ReasonManifestInvalid),
			"Set images.consensusNode.version in consensus-node-components.yaml",
		)
	}
	return &m, nil
}
