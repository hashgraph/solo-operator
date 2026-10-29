// SPDX-License-Identifier: Apache-2.0

// Package config loads and resolves beacon's runtime input: the yahcli-compatible
// config.yaml describing a single target network and its node topology.
//
// One file = one network (see docs/beacon/design.md §10): the file is a flat
// network object selected in whole by -c/--config, so there is no in-file
// network selector and no cross-environment map to pick the wrong entry from.
// The file's directory is the self-contained network context — keys/ and
// sysfiles/ resolve relative to it.
//
// config.yaml is beacon's ONLY runtime input — it carries no dependency on
// client-go/kubeconfig, so beacon stays a portable, HAPI-only tool runnable
// against any network. For Solo networks the file is generated from the Orbit
// (spec.network + spec.consensus.addressBook) by an external helper; that
// generator is out of scope here.
package config

import (
	"fmt"
	"os"

	"github.com/automa-saga/errx"
	"sigs.k8s.io/yaml"
)

// DefaultPort is the standard plaintext HAPI gRPC port; used when a node omits
// port. TLS HAPI (port 50212) is opt-in via the node's tls flag.
const DefaultPort uint32 = 50211

// defaultNodeAccount is the fallback fee-collection account number (0.0.3) when
// the config omits defaultNodeAccount.
const defaultNodeAccount uint64 = 3

// Config is the parsed config.yaml: a single flat network. Field names mirror
// yahcli's so existing operator config files and key material migrate as-is,
// plus the Solo additions (per-node host/port/tls) beacon needs.
type Config struct {
	// Name is the network's human-readable name, read from the file. It is used
	// for diagnostics only — selecting a network means selecting this file.
	Name  string `json:"name"`
	Shard uint64 `json:"shard"`
	Realm uint64 `json:"realm"`
	// DefaultPayer is the privileged account num used when --payer is absent
	// (typically 2, 50, or 58).
	DefaultPayer uint64 `json:"defaultPayer"`
	// DefaultNodeAccount is the fee-collection account num of the node targeted
	// when neither --node-account nor --node-ip is given (defaults to 3).
	DefaultNodeAccount uint64 `json:"defaultNodeAccount"`
	// AllowedReceiverAccountIds is a Phase-2 guardrail used only by `accounts
	// send`; parsed but otherwise unused in the MVP.
	AllowedReceiverAccountIds []uint64 `json:"allowedReceiverAccountIds,omitempty"`
	// Nodes is the network's node topology.
	Nodes []*Node `json:"nodes"`
}

// Node is one consensus node's HAPI endpoint and fee-collection account.
type Node struct {
	// ID is the node's identity (a non-negative integer, e.g. 0, 1, 2).
	ID uint64 `json:"id"`
	// Account is the node's fee-collection account num (composed with the
	// network's shard/realm to form the full AccountID).
	Account uint64 `json:"account"`
	// Host is the node's HAPI endpoint IP or FQDN (Solo service DNS).
	Host string `json:"host"`
	// IPv4Addr is the legacy yahcli field name for Host, accepted as an alias.
	// When Host is empty it is copied from IPv4Addr during normalization.
	IPv4Addr string `json:"ipv4Addr,omitempty"`
	// Port is the HAPI gRPC port (defaults to DefaultPort).
	Port uint32 `json:"port"`
	// TLS selects TLS HAPI when the endpoint requires it (defaults to false).
	TLS bool `json:"tls"`

	// Shard/Realm are propagated from the owning network during normalization,
	// not read from the file.
	Shard uint64 `json:"-"`
	Realm uint64 `json:"-"`
}

// Load reads and parses config.yaml at path, then normalizes it: fills defaults
// (node account, port), resolves the ipv4Addr→host alias, and propagates the
// network's shard/realm onto its nodes. The returned Config is ready for Resolve.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errx.Decorate(
			errConfigIO.Wrap(err, "read config %q", path),
			reasonConfigRead,
			fmt.Sprintf("Confirm %q exists and is readable, or pass --config/-c", path),
		)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, errx.Decorate(
			errConfigInvalid.Wrap(err, "parse config %q", path),
			reasonConfigParse,
			"Check that config.yaml is valid YAML matching the beacon schema (see docs/beacon/design.md §10)",
		)
	}

	if len(c.Nodes) == 0 {
		return nil, errx.Decorate(
			errConfigInvalid.New("config %q defines no nodes", path),
			reasonConfigParse,
			"Add at least one entry under nodes: in config.yaml",
		)
	}

	c.normalize()
	if err := c.validateNodes(path); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validateNodes(path string) error {
	for i, node := range c.Nodes {
		if node == nil {
			return errx.Decorate(
				errConfigInvalid.New("config %q: nodes[%d] is empty", path, i),
				reasonInvalidNode,
				"Remove the empty entry under nodes: in config.yaml, or fill in its account and host",
			)
		}
		if node.Host == "" {
			return errx.Decorate(
				errConfigInvalid.New("config %q: nodes[%d] (account %d) has no host", path, i, node.Account),
				reasonInvalidNode,
				"Set host (or the legacy ipv4Addr) on every node in config.yaml",
			)
		}
	}
	return nil
}

// normalize fills in defaults and propagates the network's shard/realm onto
// nodes, so downstream ID construction never hardcodes 0.0 and every node has a
// concrete host/port.
func (c *Config) normalize() {
	if c.DefaultNodeAccount == 0 {
		c.DefaultNodeAccount = defaultNodeAccount
	}
	for _, node := range c.Nodes {
		if node == nil {
			continue
		}
		if node.Host == "" {
			node.Host = node.IPv4Addr
		}
		if node.Port == 0 {
			node.Port = DefaultPort
		}
		node.Shard = c.Shard
		node.Realm = c.Realm
	}
}

// Endpoint returns the node's "host:port" HAPI address.
func (n *Node) Endpoint() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}
