// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"os"
	"path/filepath"

	"github.com/automa-saga/logx"
)

// rotateUpgradePackage moves the contents of currentPath into previousPath after
// a successful upgrade, mirroring what the consensus node itself does on install:
//
//  1. If previousPath exists: clear it (rm -rf previous/*), then move current
//     contents into it (mv -f current/* previous/). current is left empty.
//  2. Fallback: if previousPath does not exist or clearing it fails, delete
//     current contents outright (rm -rf current/*). current is left empty.
//
// In both cases the directory itself is preserved so incron can continue
// watching it for the next upgrade cycle.
func rotateUpgradePackage(currentPath, previousPath, operationID string) {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("current_path", currentPath).
			Msg("rotate: cannot read current dir — skipping rotation")
		return
	}
	if len(entries) == 0 {
		// Nothing to rotate.
		return
	}

	if rotateIntoPrevious(currentPath, previousPath, operationID) {
		return
	}

	// Fallback: could not move contents to previous — clear current instead.
	logx.As().Warn().
		Str("reason", ReasonUpgradePackageRotateFailed.String()).
		Str("operation_id", operationID).
		Str("current_path", currentPath).
		Str("previous_path", previousPath).
		Msg("rotate: could not archive upgrade package to previous — deleting current contents")
	clearDir(currentPath, operationID)
}

// rotateIntoPrevious clears previousPath then moves all entries from currentPath
// into it. Returns true on success.
func rotateIntoPrevious(currentPath, previousPath, operationID string) bool {
	if _, err := os.Stat(previousPath); err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("previous_path", previousPath).
			Msg("rotate: previous dir not accessible — falling back to delete current")
		return false
	}

	// Clear previous/* so stale data from the last upgrade is gone.
	prevEntries, err := os.ReadDir(previousPath)
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("previous_path", previousPath).
			Msg("rotate: cannot read previous dir — falling back to delete current")
		return false
	}
	for _, e := range prevEntries {
		target := filepath.Join(previousPath, e.Name())
		if rmErr := os.RemoveAll(target); rmErr != nil {
			logx.As().Warn().Err(rmErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).
				Str("path", target).
				Msg("rotate: failed to clear previous entry — falling back to delete current")
			return false
		}
	}

	// Move current/* into previous/.
	curEntries, err := os.ReadDir(currentPath)
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("current_path", currentPath).
			Msg("rotate: cannot read current dir after clearing previous — falling back to delete current")
		return false
	}
	for _, e := range curEntries {
		src := filepath.Join(currentPath, e.Name())
		dst := filepath.Join(previousPath, e.Name())
		if mvErr := os.Rename(src, dst); mvErr != nil {
			logx.As().Warn().Err(mvErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).
				Str("src", src).Str("dst", dst).
				Msg("rotate: move failed — falling back to delete remaining current entries")
			// Partial move: clean up whatever is left in current.
			clearDir(currentPath, operationID)
			return true // current is now empty either way
		}
	}

	logx.As().Info().
		Str("reason", ReasonUpgradePackageRotated.String()).
		Str("operation_id", operationID).
		Str("current_path", currentPath).
		Str("previous_path", previousPath).
		Msg("upgrade package rotated: current → previous")
	return true
}

// clearDir removes all entries inside dir, leaving the directory itself intact.
func clearDir(dir, operationID string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("path", dir).
			Msg("rotate: cannot read dir for clearDir")
		return
	}
	for _, e := range entries {
		target := filepath.Join(dir, e.Name())
		if rmErr := os.RemoveAll(target); rmErr != nil {
			logx.As().Warn().Err(rmErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).
				Str("path", target).
				Msg("rotate: failed to remove entry from current")
		}
	}
	logx.As().Info().
		Str("reason", ReasonUpgradePackageRotated.String()).
		Str("operation_id", operationID).
		Str("path", dir).
		Msg("upgrade package cleared from current (no previous dir)")
}
