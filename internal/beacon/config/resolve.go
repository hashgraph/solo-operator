// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// Target is the fully resolved result of applying the global flags against a
// config.yaml: the chosen network, the node to submit to, and the payer.
type Target struct {
	// Network is the resolved network name.
	Network string
	// Shard and Realm are the network's shard/realm, composed into every ID.
	Shard uint64
	Realm uint64
	// Node is the resolved target node (endpoint + fee-collection account). It
	// may be a config node or an ad-hoc node built from --node-ip/--node-account.
	Node *Node
	// NodeAccount is Node.Account composed into a full AccountID.
	NodeAccount hiero.AccountID
	// Payer is the resolved payer account.
	Payer hiero.AccountID
}

// Resolve applies the global flags against the loaded config and returns the
// concrete network/node/payer to act on. All string inputs may be empty (the
// flag was not set); each is defaulted per docs/beacon/design.md §6.1/§10:
//
//   - network:      networkFlag, else Config.DefaultNetwork
//   - payer:        payerFlag, else Network.DefaultPayer
//   - node:         by --node-ip and/or --node-account; a --node-ip absent from
//     config must be paired with --node-account; when neither is
//     set, the network's DefaultNodeAccount node is used.
func (c *Config) Resolve(networkFlag, payerFlag, nodeAccountFlag, nodeIPFlag string) (*Target, error) {
	net, err := c.resolveNetwork(networkFlag)
	if err != nil {
		return nil, err
	}

	payer, err := net.resolvePayer(payerFlag)
	if err != nil {
		return nil, err
	}

	node, err := net.resolveNode(nodeAccountFlag, nodeIPFlag)
	if err != nil {
		return nil, err
	}

	return &Target{
		Network:     net.Name,
		Shard:       net.Shard,
		Realm:       net.Realm,
		Node:        node,
		NodeAccount: net.accountID(node.Account),
		Payer:       payer,
	}, nil
}

// resolveNetwork picks the network named by networkFlag, falling back to
// DefaultNetwork.
func (c *Config) resolveNetwork(networkFlag string) (*Network, error) {
	name := strings.TrimSpace(networkFlag)
	if name == "" {
		name = c.DefaultNetwork
	}
	if name == "" {
		return nil, errx.Decorate(
			fmt.Errorf("no network selected and config sets no defaultNetwork"),
			reasonNoNetworkSelected,
			"Pass --network/-n or set defaultNetwork in config.yaml",
		)
	}
	net := c.Networks[name]
	if net == nil {
		return nil, errx.Decorate(
			fmt.Errorf("unknown network %q", name),
			reasonUnknownNetwork,
			fmt.Sprintf("Choose one of the networks defined in config.yaml: %s", strings.Join(c.networkNames(), ", ")),
		)
	}
	return net, nil
}

// resolvePayer resolves the payer AccountID from payerFlag, falling back to the
// network's DefaultPayer.
func (n *Network) resolvePayer(payerFlag string) (hiero.AccountID, error) {
	s := strings.TrimSpace(payerFlag)
	if s == "" {
		if n.DefaultPayer == 0 {
			return hiero.AccountID{}, errx.Decorate(
				fmt.Errorf("no payer selected and network %q sets no defaultPayer", n.Name),
				reasonUnknownPayer,
				"Pass --payer/-p or set defaultPayer for the network in config.yaml",
			)
		}
		return n.accountID(n.DefaultPayer), nil
	}
	id, err := n.parseAccountID(s)
	if err != nil {
		return hiero.AccountID{}, err
	}
	return id, nil
}

// resolveNode resolves the target node from --node-account/--node-ip per the
// rules in Resolve's doc comment. The returned Node is never a live pointer into
// the config when an override is applied (a copy is taken), so the loaded config
// is not mutated.
func (n *Network) resolveNode(nodeAccountFlag, nodeIPFlag string) (*Node, error) {
	ip := strings.TrimSpace(nodeIPFlag)
	acct := strings.TrimSpace(nodeAccountFlag)

	// --node-ip: prefer a config node at that address; otherwise it must be
	// paired with --node-account to form an ad-hoc endpoint.
	if ip != "" {
		if node := n.findByHost(ip); node != nil {
			if acct == "" {
				return node, nil
			}
			num, err := n.parseAccountNum(acct)
			if err != nil {
				return nil, err
			}
			override := *node
			override.Account = num
			return &override, nil
		}
		if acct == "" {
			return nil, errx.Decorate(
				fmt.Errorf("node-ip %q is not in config for network %q and no --node-account was given", ip, n.Name),
				reasonNodeIPWithoutAccount,
				"Pass --node-account/-a to target an off-config node by IP, or add the node to config.yaml",
			)
		}
		num, err := n.parseAccountNum(acct)
		if err != nil {
			return nil, err
		}
		return &Node{Account: num, Host: ip, Port: DefaultPort, Shard: n.Shard, Realm: n.Realm}, nil
	}

	// --node-account alone: select the config node with that account.
	if acct != "" {
		num, err := n.parseAccountNum(acct)
		if err != nil {
			return nil, err
		}
		if node := n.findByAccount(num); node != nil {
			return node, nil
		}
		return nil, errx.Decorate(
			fmt.Errorf("no node with account %d in network %q", num, n.Name),
			reasonUnknownNode,
			"Pass --node-ip/-i to target the node by address, or add it to config.yaml",
		)
	}

	// Neither given: use the network's default node account.
	if node := n.findByAccount(n.DefaultNodeAccount); node != nil {
		return node, nil
	}
	if len(n.Nodes) == 0 {
		return nil, errx.Decorate(
			fmt.Errorf("network %q defines no nodes", n.Name),
			reasonNoNodes,
			"Add at least one node under the network in config.yaml, or pass --node-ip with --node-account",
		)
	}
	return n.Nodes[0], nil
}

// findByHost returns the node whose Host (or original ipv4Addr) equals addr.
func (n *Network) findByHost(addr string) *Node {
	for _, node := range n.Nodes {
		if node != nil && (node.Host == addr || node.IPv4Addr == addr) {
			return node
		}
	}
	return nil
}

// findByAccount returns the node with the given fee-collection account num.
func (n *Network) findByAccount(num uint64) *Node {
	for _, node := range n.Nodes {
		if node != nil && node.Account == num {
			return node
		}
	}
	return nil
}

// networkNames returns the configured network names for error hints.
func (c *Config) networkNames() []string {
	names := make([]string, 0, len(c.Networks))
	for name := range c.Networks {
		names = append(names, name)
	}
	return names
}

// accountID composes a bare account num with the network's shard/realm.
func (n *Network) accountID(num uint64) hiero.AccountID {
	return hiero.AccountID{Shard: n.Shard, Realm: n.Realm, Account: num}
}

// FileID composes a bare file num (e.g. 150, 159) with the network's
// shard/realm into a full FileID, so special-file targets are never hardcoded
// 0.0.150.
func (n *Network) FileID(num uint64) hiero.FileID {
	return hiero.FileID{Shard: n.Shard, Realm: n.Realm, File: num}
}

// parseAccountID resolves an account literal into a full AccountID, composing a
// bare number with the network's shard/realm and honoring an explicit S.R.N
// literal's own shard/realm.
func (n *Network) parseAccountID(literal string) (hiero.AccountID, error) {
	shard, realm, num, err := n.parseIDLiteral(literal)
	if err != nil {
		return hiero.AccountID{}, err
	}
	return hiero.AccountID{Shard: shard, Realm: realm, Account: num}, nil
}

// parseAccountNum resolves an account literal to just its number, rejecting a
// full S.R.N literal whose shard/realm disagree with the network (the node
// account must live in the network's shard/realm).
func (n *Network) parseAccountNum(literal string) (uint64, error) {
	shard, realm, num, err := n.parseIDLiteral(literal)
	if err != nil {
		return 0, err
	}
	if shard != n.Shard || realm != n.Realm {
		return 0, errx.Decorate(
			fmt.Errorf("account %q is not in network %q shard.realm %d.%d", literal, n.Name, n.Shard, n.Realm),
			reasonInvalidAccountID,
			fmt.Sprintf("Use a bare account number or an S.R.N literal in shard.realm %d.%d", n.Shard, n.Realm),
		)
	}
	return num, nil
}

// parseIDLiteral ports yahcli's normalizePossibleIdLiteral: it accepts BOTH a
// bare number ("58" → composed with the network's shard/realm) and a full
// "S.R.N" literal ("0.0.58" → those explicit parts). Any other shape is an
// error.
func (n *Network) parseIDLiteral(literal string) (shard, realm, num uint64, err error) {
	s := strings.TrimSpace(literal)
	if s == "" {
		return 0, 0, 0, errx.Decorate(
			fmt.Errorf("empty account/file id"),
			reasonInvalidAccountID,
			"Provide a bare number (e.g. 58) or an S.R.N literal (e.g. 0.0.58)",
		)
	}

	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return 0, 0, 0, n.invalidIDErr(literal)
		}
		shard, err = strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return 0, 0, 0, n.invalidIDErr(literal)
		}
		realm, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return 0, 0, 0, n.invalidIDErr(literal)
		}
		num, err = strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			return 0, 0, 0, n.invalidIDErr(literal)
		}
		return shard, realm, num, nil
	}

	num, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, 0, 0, n.invalidIDErr(literal)
	}
	return n.Shard, n.Realm, num, nil
}

// invalidIDErr builds the shared malformed-id error.
func (n *Network) invalidIDErr(literal string) error {
	return errx.Decorate(
		fmt.Errorf("invalid account/file id %q", literal),
		reasonInvalidAccountID,
		"Use a bare number (e.g. 58) or an S.R.N literal (e.g. 0.0.58)",
	)
}
