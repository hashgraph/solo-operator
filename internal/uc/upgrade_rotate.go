// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/automa-saga/logx"
)

// rotateUpgradePackage atomically replaces previousPath with currentPath after a
// successful upgrade (Execute.Succeeded). previous always holds the last
// successfully installed package — this is the invariant that makes rollback
// meaningful. Only Execute.Succeeded triggers rotation; failed and aborted
// upgrades must never write to previous.
//
// Atomicity model:
//
//	The move is implemented as os.Rename(currentPath, previousPath), which maps
//	to a single rename(2) syscall and is atomic on any POSIX filesystem. This
//	eliminates partial-move: either the whole package moves or nothing does.
//
//	IMPORTANT: currentPath and previousPath MUST reside on the same filesystem
//	(same mount point). rename(2) fails with EXDEV across devices. In production
//	both paths share the same parent (data/upgrade/), so this is guaranteed; if
//	the operator mounts them separately they must still be on the same block
//	device. This constraint is checked at startup by ensureDirs.
//
// Crash recovery:
//
//	The function detects mid-rotation crashes purely from filesystem state —
//	no external state file is required:
//
//	  currentPath absent, previousPath present → crashed after rename but before
//	  mkdir; previous holds the complete package. Recover by recreating current/.
//
//	  currentPath present, previousPath absent → crashed after rmdir but before
//	  rename; rename directly (no rmdir needed).
//
//	  Both present → normal first run or crash before any destructive step;
//	  clear previous/, rmdir it, then rename.
//
// After rotation currentPath is recreated as an empty directory so the
// MarkerMonitor can continue polling it for the next upgrade cycle.
func rotateUpgradePackage(currentPath, previousPath, operationID string) {
	// Crash recovery: current/ is absent means we crashed after rename(current→previous)
	// but before mkdir(current). previous already holds the complete package —
	// just recreate current/ and we are done.
	if _, err := os.Stat(currentPath); errors.Is(err, os.ErrNotExist) {
		logx.As().Info().
			Str("reason", ReasonUpgradePackageRotated.String()).
			Str("operation_id", operationID).
			Str("current_path", currentPath).
			Msg("rotate: current/ absent — recreating after post-rename crash")
		recreateDir(currentPath, previousPath, operationID)
		return
	}

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
		return
	}

	if rotateIntoPrevious(currentPath, previousPath, operationID) {
		return
	}

	// Fallback: rename failed (e.g. EXDEV — cross-device, which should never
	// happen in production but could occur in tests or unusual mounts).
	logx.As().Warn().
		Str("reason", ReasonUpgradePackageRotateFailed.String()).
		Str("operation_id", operationID).
		Str("current_path", currentPath).
		Str("previous_path", previousPath).
		Msg("rotate: atomic rename failed — deleting current contents (package not archived)")
	clearDir(currentPath, operationID)
}

// rotateIntoPrevious atomically replaces previousPath with currentPath via
// rename(2), then recreates an empty currentPath.
//
// Steps (and their crash-recovery behaviour):
//  1. If previousPath exists: clear its contents and rmdir it.
//     Crash here → both dirs still exist; retry restarts from step 1.
//  2. os.Rename(currentPath, previousPath) — single atomic syscall.
//     Crash here → currentPath absent, previousPath complete; rotateUpgradePackage
//     detects this and calls recreateDir on the next run.
//  3. Recreate currentPath as an empty directory (inherit permissions from
//     previousPath, which was currentPath moments ago).
//     Crash here → same as step 2 crash; handled identically.
//
// Returns true on success (or when current is empty after a fallback clear).
func rotateIntoPrevious(currentPath, previousPath, operationID string) bool {
	// Step 1: remove previous/ if it exists so rename can replace it.
	// Crash during this step leaves previous/ partially cleared but current/
	// intact — safe to retry from scratch.
	if _, statErr := os.Stat(previousPath); statErr == nil {
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
		if rmErr := os.Remove(previousPath); rmErr != nil {
			logx.As().Warn().Err(rmErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).
				Str("previous_path", previousPath).
				Msg("rotate: failed to rmdir previous — falling back to delete current")
			return false
		}
	}

	// Step 2: atomic rename — the entire package moves in one syscall.
	// currentPath and previousPath must be on the same filesystem; rename(2)
	// returns EXDEV otherwise. In production both are siblings under data/upgrade/
	// on the same volume, so this is always satisfied.
	if mvErr := os.Rename(currentPath, previousPath); mvErr != nil {
		logx.As().Warn().Err(mvErr).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("src", currentPath).Str("dst", previousPath).
			Msg("rotate: atomic rename failed — falling back to delete current")
		return false
	}

	// Step 3: recreate currentPath so MarkerMonitor can poll it next cycle.
	// previousPath (the just-renamed directory) carries the original permissions;
	// reuse them so current/ is created with the same mode.
	recreateDir(currentPath, previousPath, operationID)

	logx.As().Info().
		Str("reason", ReasonUpgradePackageRotated.String()).
		Str("operation_id", operationID).
		Str("current_path", currentPath).
		Str("previous_path", previousPath).
		Msg("upgrade package rotated: current → previous")
	return true
}

// recreateDir creates an empty directory at path, inheriting permissions from
// permSource (typically previousPath immediately after the rename). Called after
// rename(current→previous) to restore current/ for the next upgrade cycle.
func recreateDir(path, permSource, operationID string) {
	perm := os.FileMode(0o750) // safe default
	if info, err := os.Stat(permSource); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.Mkdir(path, perm); err != nil && !errors.Is(err, os.ErrExist) {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).
			Str("path", path).
			Msg("rotate: failed to recreate current dir — MarkerMonitor will retry on next poll")
	}
}

// discardUpgradePackage wipes current/* without touching previous. Called on
// FreezeAbort.Succeeded — the staged package was never applied (the freeze was
// cancelled before the node installed it), so there is nothing to rotate.
// previous always holds the last successfully installed package; discarding
// current preserves that invariant by never overwriting previous with a
// staged-but-never-used package. The current directory itself is preserved so
// the MarkerMonitor can continue watching it for the next upgrade cycle.
func discardUpgradePackage(currentPath, operationID string) {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		logx.As().Warn().Err(err).
			Str("reason", ReasonUpgradePackageRotateFailed.String()).
			Str("operation_id", operationID).Str("path", currentPath).
			Msg("abort: cannot read current dir — staged package may remain")
		return
	}
	var discardErr bool
	for _, e := range entries {
		target := filepath.Join(currentPath, e.Name())
		if rmErr := os.RemoveAll(target); rmErr != nil {
			discardErr = true
			logx.As().Warn().Err(rmErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).Str("path", target).
				Msg("abort: failed to remove entry from current — manual cleanup may be required")
		}
	}
	if !discardErr {
		logx.As().Info().
			Str("reason", ReasonUpgradePackageDiscarded.String()).
			Str("operation_id", operationID).Str("path", currentPath).
			Msg("discarded staged upgrade package on abort (current wiped, previous untouched)")
	}
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
	var clearErr bool
	for _, e := range entries {
		target := filepath.Join(dir, e.Name())
		if rmErr := os.RemoveAll(target); rmErr != nil {
			clearErr = true
			logx.As().Warn().Err(rmErr).
				Str("reason", ReasonUpgradePackageRotateFailed.String()).
				Str("operation_id", operationID).
				Str("path", target).
				Msg("rotate: failed to remove entry from current")
		}
	}
	if !clearErr {
		logx.As().Info().
			Str("reason", ReasonUpgradePackageDiscarded.String()).
			Str("operation_id", operationID).
			Str("path", dir).
			Msg("upgrade package cleared from current (no previous dir — contents discarded, not archived)")
	}
}
