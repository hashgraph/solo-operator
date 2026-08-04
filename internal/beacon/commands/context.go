// SPDX-License-Identifier: Apache-2.0

// Package commands wires beacon's cobra subcommands to the operations in
// internal/beacon/ops, resolving each command's target/payer/client from the
// global flags and submitting through the verified-submit helper.
package commands

import (
	"path/filepath"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"

	"github.com/hashgraph/solo-operator/internal/beacon/client"
	"github.com/hashgraph/solo-operator/internal/beacon/config"
	"github.com/hashgraph/solo-operator/internal/beacon/keystore"
)

// Global carries the resolved values of beacon's persistent flags into the
// commands package (main.go owns the flags themselves).
type Global struct {
	// ConfigPath is -c/--config: the path to the network's config.yaml. Its
	// directory holds keys/ and sysfiles/ (design §10).
	ConfigPath string
	// Payer, NodeAccount, NodeIP mirror -p/-a/-i and feed config.Resolve.
	Payer       string
	NodeAccount string
	NodeIP      string
}

// Session is a resolved command context: the target the command acts on plus a
// verified-submit client signed by the target's payer. Callers must Close it.
type Session struct {
	Target *config.Target
	Client *client.Client
}

// Close releases the session's client connections.
func (s *Session) Close() error {
	if s.Client == nil {
		return nil
	}
	return s.Client.Close()
}

// SessionFunc resolves a Session from the global flags at command-execution time.
// main.go supplies one closed over its parsed globalOptions; tests supply a stub.
type SessionFunc func() (*Session, error)

// Resolve loads the config, resolves the target node/payer, loads the payer's
// signing key from <dir(config)>/keys, and builds a verified-submit client. It is
// the shared entry point every network-touching command uses.
func (g Global) Resolve() (*Session, error) {
	cfg, err := config.Load(g.ConfigPath)
	if err != nil {
		return nil, err
	}

	target, err := cfg.Resolve(g.Payer, g.NodeAccount, g.NodeIP)
	if err != nil {
		return nil, err
	}

	keysDir := filepath.Join(filepath.Dir(g.ConfigPath), "keys")
	// The keystore decorates its own error at the source; return it as-is.
	key, err := keystore.New(keysDir).Load(target.Payer.Account)
	if err != nil {
		return nil, err
	}

	network := map[string]hiero.AccountID{target.Node.Endpoint(): target.NodeAccount}
	c, err := client.New(network, target.Payer, key, client.Options{TLS: target.Node.TLS})
	if err != nil {
		return nil, err
	}

	return &Session{Target: target, Client: c}, nil
}
