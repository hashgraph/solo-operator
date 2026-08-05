// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/beacon/config"
	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

const validHash = "abababababababababababababababababababababababababababababababababababababababababababababababababab" // 48 bytes

// ttyConfirmer is a confirmer that behaves like an interactive terminal auto-
// answering "yes", so command tests exercise the post-gate flow.
func ttyConfirmer(answer string) confirmer {
	return confirmer{in: strings.NewReader(answer), out: &bytes.Buffer{}, isTTY: true}
}

// nonTTYConfirmer refuses without --yes, like CI.
func nonTTYConfirmer() confirmer {
	return confirmer{in: strings.NewReader(""), out: &bytes.Buffer{}, isTTY: false}
}

// runCmd executes a cobra command with args, capturing stdout.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestFreezeCommands_AllRegistered(t *testing.T) {
	group := FreezeCommands(func() (*Session, error) { return nil, nil })
	if group.Name() != "freeze" {
		t.Fatalf("group name = %q, want %q", group.Name(), "freeze")
	}
	got := make(map[string]*cobra.Command)
	for _, c := range group.Commands() {
		got[c.Name()] = c
	}
	for _, name := range []string{"prepare", "upgrade", "telemetry", "only", "abort"} {
		if got[name] == nil {
			t.Errorf("freeze subcommand %q not registered", name)
		}
	}
	// Every variant carries the --yes gate flag.
	for name, c := range got {
		if c.Flags().Lookup("yes") == nil {
			t.Errorf("%q missing --yes flag", name)
		}
	}
}

func TestPrepareUpgrade_NonInteractiveWithoutYesFails(t *testing.T) {
	resolved := false
	resolve := func() (*Session, error) {
		resolved = true
		return nil, nil
	}
	cmd := prepareUpgradeCmd(resolve, nonTTYConfirmer())

	_, err := runCmd(t, cmd, "--upgrade-zip-hash", validHash)
	wantReason(t, err, reasonConfirmationRequired)
	if resolved {
		t.Error("session resolved despite the confirmation gate refusing")
	}
}

func TestPrepareUpgrade_YesBypassesGateAndResolves(t *testing.T) {
	// --yes must skip the (non-TTY) gate and proceed to resolve; we surface a
	// sentinel from resolve to prove the flow reached it.
	sentinel := errx.Decorate(fmt.Errorf("boom"), errx.Reason("ResolveStub"))
	resolve := func() (*Session, error) { return nil, sentinel }
	cmd := prepareUpgradeCmd(resolve, nonTTYConfirmer())

	_, err := runCmd(t, cmd, "--upgrade-zip-hash", validHash, "--yes")
	wantReason(t, err, errx.Reason("ResolveStub"))
}

func TestPrepareUpgrade_RequiresHash(t *testing.T) {
	cmd := prepareUpgradeCmd(func() (*Session, error) { return nil, nil }, ttyConfirmer("yes\n"))
	_, err := runCmd(t, cmd, "--yes")
	if err == nil || !strings.Contains(err.Error(), "upgrade-zip-hash") {
		t.Errorf("expected required-flag error for --upgrade-zip-hash, got %v", err)
	}
}

func TestFreezeUpgrade_InvalidHashRejectedAfterResolve(t *testing.T) {
	// Resolve succeeds (offline session, no client needed); the bad hash is
	// caught in the build step.
	resolve := func() (*Session, error) {
		return &Session{Target: &config.Target{}}, nil // Client nil: build fails first
	}
	cmd := freezeUpgradeCmd(resolve, ttyConfirmer("yes\n"))

	_, err := runCmd(t, cmd, "--start-time", "2026-08-01.14:30:00", "--upgrade-zip-hash", "nothex")
	wantReason(t, err, errx.Reason("InvalidUpgradeHash"))
}

func TestFreezeOnly_InvalidStartTimeRejected(t *testing.T) {
	resolve := func() (*Session, error) {
		return &Session{Target: &config.Target{}}, nil
	}
	cmd := freezeOnlyCmd(resolve, ttyConfirmer("yes\n"))

	_, err := runCmd(t, cmd, "--start-time", "not-a-time")
	wantReason(t, err, errx.Reason("InvalidStartTime"))
}

// TestBuildersComposeFileIDUnderShardRealm guards that the command's update-file
// id is composed under the target's shard/realm (via config.Target.FileID), not
// hardcoded 0.0.150.
func TestBuildersComposeFileIDUnderShardRealm(t *testing.T) {
	target := &config.Target{Shard: 5, Realm: 2}
	if got := target.FileID(updateFileNum); got.Shard != 5 || got.Realm != 2 || got.File != 150 {
		t.Errorf("FileID = %v, want 5.2.150", got)
	}
	// Sanity: the ops builder carries exactly that id.
	tx := ops.PrepareUpgrade(target.FileID(updateFileNum), bytes.Repeat([]byte{1}, 48))
	if id := tx.GetFileID(); id == nil || id.Shard != 5 || id.Realm != 2 || id.File != 150 {
		t.Errorf("prepare-upgrade file id = %v, want 5.2.150", tx.GetFileID())
	}
}
