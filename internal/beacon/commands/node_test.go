// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
	"github.com/spf13/cobra"
)

func TestNodeCommands_Registered(t *testing.T) {
	group := NodeCommands(func() (*Session, error) { return nil, nil })
	if group.Name() != "node" {
		t.Fatalf("group name = %q, want %q", group.Name(), "node")
	}
	got := make(map[string]*cobra.Command)
	for _, c := range group.Commands() {
		got[c.Name()] = c
	}
	if got["create-from-genesis"] == nil {
		t.Error("node subcommand \"create-from-genesis\" not registered")
	}
}

func TestCreateFromGenesis_RequiresGenesisNetworkFlag(t *testing.T) {
	cmd := createFromGenesisCmd(func() (*Session, error) { return nil, nil }, ttyConfirmer("yes\n"))
	_, err := runCmd(t, cmd, "--yes")
	if err == nil || !strings.Contains(err.Error(), "genesis-network") {
		t.Errorf("expected required-flag error for --genesis-network, got %v", err)
	}
}

func TestCreateFromGenesis_NonInteractiveWithoutYesFails(t *testing.T) {
	resolved := false
	resolve := func() (*Session, error) {
		resolved = true
		return nil, nil
	}
	cmd := createFromGenesisCmd(resolve, nonTTYConfirmer())

	_, err := runCmd(t, cmd, "--genesis-network", "/does/not/matter")
	wantReason(t, err, reasonConfirmationRequired)
	if resolved {
		t.Error("session resolved despite the confirmation gate refusing")
	}
}

func TestCreateFromGenesis_MissingFileFailsBeforeResolve(t *testing.T) {
	resolved := false
	resolve := func() (*Session, error) {
		resolved = true
		return nil, nil
	}
	cmd := createFromGenesisCmd(resolve, ttyConfirmer("yes\n"))

	_, err := runCmd(t, cmd, "--genesis-network", filepath.Join(t.TempDir(), "missing.json"), "--yes")
	wantReason(t, err, reasonGenesisNetworkReadFailed)
	if resolved {
		t.Error("session resolved despite the genesis-network file being unreadable")
	}
}

func TestCreateFromGenesis_InvalidGenesisNetworkFailsBeforeResolve(t *testing.T) {
	resolved := false
	resolve := func() (*Session, error) {
		resolved = true
		return nil, nil
	}
	path := filepath.Join(t.TempDir(), "genesis-network.json")
	if err := os.WriteFile(path, []byte(`{"nodeMetadata": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := createFromGenesisCmd(resolve, ttyConfirmer("yes\n"))

	_, err := runCmd(t, cmd, "--genesis-network", path, "--yes")
	wantReason(t, err, errx.Reason("GenesisNetworkInvalid")) // ops package's reason, not exported from commands
	if resolved {
		t.Error("session resolved despite genesis-network.json having no node entries")
	}
}
