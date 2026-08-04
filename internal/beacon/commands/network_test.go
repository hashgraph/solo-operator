// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"errors"
	"testing"
)

func TestNetworkCommand_VersionRegistered(t *testing.T) {
	root := NetworkCommand(func() (*Session, error) { return nil, nil })
	if root.Name() != "network" {
		t.Errorf("group name = %q, want %q", root.Name(), "network")
	}
	ver, _, err := root.Find([]string{"version"})
	if err != nil || ver.Name() != "version" {
		t.Fatalf("`network version` not registered: cmd=%v err=%v", ver, err)
	}
}

func TestNetworkVersion_ResolveErrorPropagates(t *testing.T) {
	sentinel := errors.New("no config")
	cmd := networkVersionCmd(func() (*Session, error) { return nil, sentinel })
	// resolve fails before any network query, so this stays offline.
	_, err := runCmd(t, cmd)
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the resolve error", err)
	}
}
