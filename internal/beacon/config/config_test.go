// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// loadNetwork loads the testdata config.yaml for the named network (one file per
// network, per docs/beacon/design.md §10).
func loadNetwork(t *testing.T, name string) *Config {
	t.Helper()
	c, err := Load(filepath.Join("testdata", name, "config.yaml"))
	if err != nil {
		t.Fatalf("Load %s: %v", name, err)
	}
	return c
}

func TestLoad_ParsesAndNormalizes(t *testing.T) {
	// name populated from the file, host/port/tls parse.
	sl := loadNetwork(t, "solo-local")
	if sl.Name != "solo-local" {
		t.Errorf("network Name not populated: %q", sl.Name)
	}
	n0 := sl.Nodes[0]
	if n0.Host != "network-node1-svc.solo.svc.cluster.local" || n0.Port != 50211 || n0.TLS {
		t.Errorf("node0 host/port/tls = %q/%d/%v", n0.Host, n0.Port, n0.TLS)
	}
	if got := n0.Endpoint(); got != "network-node1-svc.solo.svc.cluster.local:50211" {
		t.Errorf("Endpoint = %q", got)
	}

	// ipv4Addr alias → host, and default port applied.
	mn := loadNetwork(t, "mainnet").Nodes[0]
	if mn.Host != "35.237.200.180" {
		t.Errorf("ipv4Addr alias not applied to Host: %q", mn.Host)
	}
	if mn.Port != DefaultPort {
		t.Errorf("default port not applied: %d, want %d", mn.Port, DefaultPort)
	}

	// shard/realm propagate onto nodes, non-zero case.
	s5 := loadNetwork(t, "shard5")
	if s5.Nodes[0].Shard != 5 || s5.Nodes[0].Realm != 2 {
		t.Errorf("shard/realm not propagated: %d.%d", s5.Nodes[0].Shard, s5.Nodes[0].Realm)
	}
	if s5.Nodes[0].TLS != true || s5.Nodes[0].Port != 51211 {
		t.Errorf("shard5 node tls/port = %v/%d", s5.Nodes[0].TLS, s5.Nodes[0].Port)
	}
}

func TestResolve_DefaultsAndPayer(t *testing.T) {
	c := loadNetwork(t, "solo-local")

	// Empty payer → network defaultPayer; empty node selection → the
	// defaultNodeAccount node.
	tg, err := c.Resolve("", "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Network != "solo-local" {
		t.Errorf("Network = %q, want solo-local", tg.Network)
	}
	wantPayer := hiero.AccountID{Shard: 0, Realm: 0, Account: 2}
	if tg.Payer != wantPayer {
		t.Errorf("Payer = %v, want %v", tg.Payer, wantPayer)
	}
	if tg.Node.Account != 3 {
		t.Errorf("default node account = %d, want 3", tg.Node.Account)
	}
	wantNodeAcct := hiero.AccountID{Shard: 0, Realm: 0, Account: 3}
	if tg.NodeAccount != wantNodeAcct {
		t.Errorf("NodeAccount = %v, want %v", tg.NodeAccount, wantNodeAcct)
	}
}

func TestResolve_BareAndLiteralPayer(t *testing.T) {
	c := loadNetwork(t, "solo-local")

	// Bare number and full S.R.N literal resolve to the same AccountID under the
	// network's shard/realm.
	bare, err := c.Resolve("58", "", "")
	if err != nil {
		t.Fatalf("Resolve bare payer: %v", err)
	}
	literal, err := c.Resolve("0.0.58", "", "")
	if err != nil {
		t.Fatalf("Resolve literal payer: %v", err)
	}
	want := hiero.AccountID{Shard: 0, Realm: 0, Account: 58}
	if bare.Payer != want || literal.Payer != want {
		t.Errorf("payer bare=%v literal=%v, want %v", bare.Payer, literal.Payer, want)
	}
}

func TestResolve_NonZeroShardRealm(t *testing.T) {
	c := loadNetwork(t, "shard5")

	tg, err := c.Resolve("", "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Shard != 5 || tg.Realm != 2 {
		t.Errorf("Shard.Realm = %d.%d, want 5.2", tg.Shard, tg.Realm)
	}
	// Bare payer composes with the network's non-zero shard/realm.
	wantPayer := hiero.AccountID{Shard: 5, Realm: 2, Account: 58}
	if tg.Payer != wantPayer {
		t.Errorf("Payer = %v, want %v", tg.Payer, wantPayer)
	}
	// FileID for 0.0.150-equivalent is composed as 5.2.150.
	wantFile := hiero.FileID{Shard: 5, Realm: 2, File: 150}
	if got := c.FileID(150); got != wantFile {
		t.Errorf("FileID(150) = %v, want %v", got, wantFile)
	}
	// Default node account 7 selected.
	if tg.Node.Account != 7 {
		t.Errorf("node account = %d, want 7", tg.Node.Account)
	}
}

func TestResolve_NodeByAccount(t *testing.T) {
	c := loadNetwork(t, "solo-local")

	tg, err := c.Resolve("", "4", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Node.ID != 1 || tg.Node.Account != 4 {
		t.Errorf("node id/account = %d/%d, want 1/4", tg.Node.ID, tg.Node.Account)
	}
}

func TestResolve_NodeByIPInConfig(t *testing.T) {
	c := loadNetwork(t, "mainnet")

	// IP present in config (via ipv4Addr alias) selects that node.
	tg, err := c.Resolve("", "", "35.237.200.180")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Node.ID != 0 || tg.Node.Account != 3 {
		t.Errorf("node id/account = %d/%d, want 0/3", tg.Node.ID, tg.Node.Account)
	}
}

func TestResolve_NodeByIPOffConfigWithAccount(t *testing.T) {
	c := loadNetwork(t, "solo-local")

	// IP not in config, paired with --node-account → ad-hoc endpoint, does not
	// mutate config.
	tg, err := c.Resolve("", "9", "10.9.9.9")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Node.Host != "10.9.9.9" || tg.Node.Port != DefaultPort || tg.Node.Account != 9 {
		t.Errorf("ad-hoc node = %q:%d acct %d", tg.Node.Host, tg.Node.Port, tg.Node.Account)
	}
	if len(c.Nodes) != 2 {
		t.Error("ad-hoc resolution must not append to config nodes")
	}
}

func TestResolve_NodeIPOverrideDoesNotMutateConfig(t *testing.T) {
	c := loadNetwork(t, "mainnet")

	// IP in config + explicit account overrides the fee-collection account on a
	// copy, leaving the config node untouched.
	tg, err := c.Resolve("", "99", "35.237.200.180")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tg.Node.Account != 99 {
		t.Errorf("override account = %d, want 99", tg.Node.Account)
	}
	if c.Nodes[0].Account != 3 {
		t.Errorf("config node mutated: account = %d, want 3", c.Nodes[0].Account)
	}
}

// wantReason asserts err carries the given errx reason code.
func wantReason(t *testing.T, err error, reason errx.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	got, ok := errx.ReasonOf(err)
	if !ok || got != reason {
		t.Errorf("reason = %q (ok=%v), want %q", got, ok, reason)
	}
}

func TestResolve_Errors(t *testing.T) {
	c := loadNetwork(t, "solo-local")

	t.Run("unknown node account", func(t *testing.T) {
		_, err := c.Resolve("", "404", "")
		wantReason(t, err, reasonUnknownNode)
	})

	t.Run("ip off-config without account", func(t *testing.T) {
		_, err := c.Resolve("", "", "1.2.3.4")
		wantReason(t, err, reasonNodeIPWithoutAccount)
	})

	t.Run("no payer and no defaultPayer", func(t *testing.T) {
		// Build a network with no defaultPayer.
		cfg := &Config{Name: "bare", Nodes: []*Node{{ID: 0, Account: 3}}}
		cfg.normalize()
		_, err := cfg.Resolve("", "", "")
		wantReason(t, err, reasonUnknownPayer)
	})

	t.Run("malformed account literal", func(t *testing.T) {
		_, err := c.Resolve("0.0", "", "")
		wantReason(t, err, reasonInvalidAccountID)
	})

	t.Run("account literal in wrong shard/realm", func(t *testing.T) {
		_, err := c.Resolve("", "1.0.4", "")
		wantReason(t, err, reasonInvalidAccountID)
	})
}

func TestLoad_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Load(filepath.Join("testdata", "does-not-exist.yaml"))
		wantReason(t, err, reasonConfigRead)
	})

	for name, body := range map[string]string{
		"null node":           "name: n\nnodes: [null]\n",
		"null among valid":    "name: n\nnodes:\n  - null\n  - {account: 3, host: 10.0.0.1}\n",
		"node without host":   "name: n\nnodes:\n  - {account: 3}\n",
		"node with null host": "name: n\nnodes:\n  - {account: 3, host: null}\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			wantReason(t, err, reasonInvalidNode)
		})
	}
}
