// SPDX-License-Identifier: Apache-2.0

// Package config loads and resolves beacon's runtime input: the yahcli-compatible
// config.yaml describing the networks beacon can target and their node topology.
//
// config.yaml is beacon's ONLY runtime input — it carries no dependency on
// client-go/kubeconfig, so beacon stays a portable, HAPI-only tool runnable
// against any network (see docs/beacon/design.md §10, §10.1). For Solo networks
// the file is generated from Orbit.AddressBook by an external helper; that
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
// a network omits defaultNodeAccount.
const defaultNodeAccount uint64 = 3

// Config is the parsed config.yaml. Field names mirror yahcli's so existing
// operator config files and key material migrate as-is, plus the Solo additions
// (per-node host/port/tls) beacon needs.
type Config struct {
	// DefaultNetwork is the network used when --network is not supplied.
	DefaultNetwork string `json:"defaultNetwork"`
	// Networks is the set of targetable networks keyed by name.
	Networks map[string]*Network `json:"networks"`
}

// Network is a single targetable network. Shard/Realm default to 0/0 and are
// propagated to every node during normalization so all IDs beacon builds are
// composed with them (never hardcoded 0.0).
type Network struct {
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

	// Name is the map key this network was loaded under. Populated during Load,
	// not read from the file.
	Name string `json:"-"`
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
// (node account, port), resolves the ipv4Addr→host alias, and propagates each
// network's shard/realm onto its nodes. The returned Config is ready for Resolve.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errx.Decorate(
			fmt.Errorf("read config %q: %w", path, err),
			reasonConfigRead,
			fmt.Sprintf("Confirm %q exists and is readable, or pass --config/-c", path),
		)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, errx.Decorate(
			fmt.Errorf("parse config %q: %w", path, err),
			reasonConfigParse,
			"Check that config.yaml is valid YAML matching the beacon schema (see docs/beacon/design.md §10)",
		)
	}

	if len(c.Networks) == 0 {
		return nil, errx.Decorate(
			fmt.Errorf("config %q defines no networks", path),
			reasonConfigParse,
			"Add at least one entry under networks: in config.yaml",
		)
	}

	c.normalize()
	return &c, nil
}

// normalize fills in defaults and propagates network-level shard/realm onto
// nodes, so downstream ID construction never hardcodes 0.0 and every node has a
// concrete host/port.
func (c *Config) normalize() {
	for name, n := range c.Networks {
		if n == nil {
			continue
		}
		n.Name = name
		if n.DefaultNodeAccount == 0 {
			n.DefaultNodeAccount = defaultNodeAccount
		}
		for _, node := range n.Nodes {
			if node == nil {
				continue
			}
			if node.Host == "" {
				node.Host = node.IPv4Addr
			}
			if node.Port == 0 {
				node.Port = DefaultPort
			}
			node.Shard = n.Shard
			node.Realm = n.Realm
		}
	}
}

// Endpoint returns the node's "host:port" HAPI address.
func (n *Node) Endpoint() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}
