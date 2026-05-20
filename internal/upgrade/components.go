// SPDX-License-Identifier: Apache-2.0

// Package upgrade contains shared helpers used by the four NetworkUpgrade*
// reconcilers (Prepare, Freeze, Execute, FreezeAbort).
package upgrade

import operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"

// ComponentSpec returns the per-component spec from an UpgradeImagesSpec by
// name. Returns nil when the component is omitted from the manifest.
func ComponentSpec(images *operatorv1alpha1.UpgradeImagesSpec, name operatorv1alpha1.ComponentName) *operatorv1alpha1.UpgradeComponentSpec {
	if images == nil {
		return nil
	}
	switch name {
	case operatorv1alpha1.ComponentConsensusNode:
		return &images.ConsensusNode
	case operatorv1alpha1.ComponentRecordStreamUploader:
		return images.RecordStreamUploader
	case operatorv1alpha1.ComponentEventStreamUploader:
		return images.EventStreamUploader
	case operatorv1alpha1.ComponentBlockStreamUploader:
		return images.BlockStreamUploader
	case operatorv1alpha1.ComponentBackupUploader:
		return images.BackupUploader
	case operatorv1alpha1.ComponentUC:
		return images.UC
	}
	return nil
}

// ComponentExpectsLayerHashes reports whether the component, or any of its
// registries, has at least one platform's layer-hash list configured. When
// this is true the validator's verified=false result must be treated as
// "registry rejected" rather than "no hashes to compare".
func ComponentExpectsLayerHashes(c *operatorv1alpha1.UpgradeComponentSpec) bool {
	if c == nil {
		return false
	}
	if c.Deterministic.Supported {
		for _, hs := range c.Deterministic.LayerHashes {
			if len(hs) > 0 {
				return true
			}
		}
	}
	for i := range c.Registries {
		for _, hs := range c.Registries[i].LayerHashes {
			if len(hs) > 0 {
				return true
			}
		}
	}
	return false
}

// ComponentEnabled reports whether the component is present and enabled.
// Returns (present, enabled). A nil component is present=false.
// Enabled defaults to true when the pointer is omitted.
func ComponentEnabled(c *operatorv1alpha1.UpgradeComponentSpec) (present, enabled bool) {
	if c == nil {
		return false, false
	}
	if c.Enabled == nil {
		return true, true
	}
	return true, *c.Enabled
}

// SelectedFor returns the per-component selection from a SelectedSources block.
func SelectedFor(sel *operatorv1alpha1.SelectedSources, name operatorv1alpha1.ComponentName) *operatorv1alpha1.SelectedSource {
	if sel == nil {
		return nil
	}
	switch name {
	case operatorv1alpha1.ComponentConsensusNode:
		return sel.ConsensusNode
	case operatorv1alpha1.ComponentRecordStreamUploader:
		return sel.RecordStreamUploader
	case operatorv1alpha1.ComponentEventStreamUploader:
		return sel.EventStreamUploader
	case operatorv1alpha1.ComponentBlockStreamUploader:
		return sel.BlockStreamUploader
	case operatorv1alpha1.ComponentBackupUploader:
		return sel.BackupUploader
	case operatorv1alpha1.ComponentUC:
		return sel.UC
	}
	return nil
}

// SetSelected writes a per-component selection into a SelectedSources block.
// Allocates the block if it was nil and returns the same pointer for chaining.
func SetSelected(sel *operatorv1alpha1.SelectedSources, name operatorv1alpha1.ComponentName, src *operatorv1alpha1.SelectedSource) *operatorv1alpha1.SelectedSources {
	if sel == nil {
		sel = &operatorv1alpha1.SelectedSources{}
	}
	switch name {
	case operatorv1alpha1.ComponentConsensusNode:
		sel.ConsensusNode = src
	case operatorv1alpha1.ComponentRecordStreamUploader:
		sel.RecordStreamUploader = src
	case operatorv1alpha1.ComponentEventStreamUploader:
		sel.EventStreamUploader = src
	case operatorv1alpha1.ComponentBlockStreamUploader:
		sel.BlockStreamUploader = src
	case operatorv1alpha1.ComponentBackupUploader:
		sel.BackupUploader = src
	case operatorv1alpha1.ComponentUC:
		sel.UC = src
	}
	return sel
}
