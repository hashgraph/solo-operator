// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashgraph/solo-operator/internal/uc"
)

func ensureUpgradePath(upgradePath string) error {
	if err := os.MkdirAll(upgradePath, 0o755); err != nil {
		return fmt.Errorf("create upgrade path %s: %w", upgradePath, err)
	}
	return nil
}

// touchMarker mirrors `touch`: always bumps mtime, since UC's staleness guards compare it.
func touchMarker(upgradePath, name string) error {
	if err := ensureUpgradePath(upgradePath); err != nil {
		return err
	}

	path := filepath.Join(upgradePath, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("touch marker %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("touch marker %s: %w", name, err)
	}

	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		return fmt.Errorf("touch marker %s: %w", name, err)
	}
	return nil
}

// Content is decimal Unix epoch seconds (internal/uc/freeze.go:parseFreezeScheduledTime).
func writeFreezeScheduled(upgradePath string, seconds int64) error {
	if err := ensureUpgradePath(upgradePath); err != nil {
		return err
	}

	path := filepath.Join(upgradePath, uc.MarkerFreezeScheduled)
	if err := os.WriteFile(path, []byte(strconv.FormatInt(seconds, 10)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", uc.MarkerFreezeScheduled, err)
	}
	return nil
}
