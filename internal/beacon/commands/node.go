// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"
	"os"

	"github.com/automa-saga/errx"
	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

// NodeCommands returns the `node` command group: HIP-869 dynamic address book
// operations.
func NodeCommands(resolve SessionFunc) *cobra.Command {
	c := defaultConfirmer()
	node := &cobra.Command{
		Use:   "node",
		Short: "Operate on the HIP-869 dynamic address book (Node entities)",
	}
	node.AddCommand(createFromGenesisCmd(resolve, c))
	return node
}

func createFromGenesisCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var (
		path         string
		adminKeysDir string
		yes          bool
	)
	cmd := &cobra.Command{
		Use:   "create-from-genesis",
		Short: "Register the genesis address book into the dynamic address book (HIP-869)",
		Long: "Submits one NodeCreateTransaction per node in a genesis-network.json (the file solo-operator's " +
			"NetworkGenesis controller writes into the genesis-config ConfigMap), in ascending node-id order, " +
			"so genesis-bootstrapped nodes exist in the HIP-869 dynamic address book (the \"Node\" entity) as " +
			"well as the classic roster. With --admin-keys-dir it also sets and co-signs each node's AdminKey " +
			"from hashgen's node-node<N>-admin.key files (as bundled into the system-account-keys Secret); " +
			"without it, AdminKey is left unset — apply one later with NodeUpdateTransaction once provisioned. " +
			"Each submission's assigned node id is verified against the classic id before continuing; a " +
			"mismatch stops the run rather than silently drifting from the classic roster. Assumes the payer " +
			"(--payer/-p) can sign for each node's account, which holds for genesis system accounts — they all " +
			"share bootstrap.genesisPublicKey.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreateFromGenesis(cmd, resolve, c, yes, path, adminKeysDir)
		},
	}
	cmd.Flags().StringVar(&path, "genesis-network", "", "Path to the genesis-network.json extracted from the genesis-config ConfigMap (required)")
	cmd.Flags().StringVar(&adminKeysDir, "admin-keys-dir", "",
		"Directory of hashgen's node-node<N>-admin.key files (the system-account-keys Secret, mounted). "+
			"When set, each node's AdminKey is set and co-signed at creation instead of left unset.")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	_ = cmd.MarkFlagRequired("genesis-network")
	return cmd
}

func runCreateFromGenesis(cmd *cobra.Command, resolve SessionFunc, c confirmer, yes bool, path, adminKeysDir string) error {
	if err := c.confirm(yes, "register the genesis address book into the dynamic address book"); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errx.Decorate(
			errInput.Wrap(err, "read genesis-network.json %q", path),
			reasonGenesisNetworkReadFailed,
			fmt.Sprintf("Confirm %q exists — extract it with: kubectl get configmap genesis-config "+
				"-o jsonpath='{.data.genesis-network\\.json}' > %s", path, path),
		)
	}

	nodes, err := ops.ParseGenesisNetwork(data)
	if err != nil {
		return err
	}

	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	out := cmd.OutOrStdout()
	for _, node := range nodes {
		if adminKeysDir != "" {
			adminKey, err := ops.LoadAdminKey(adminKeysDir, node.Expected)
			if err != nil {
				return err
			}
			node.Transaction.SetAdminKey(adminKey.PublicKey())
			// A newly-set AdminKey must co-sign to prove control; Submit adds the
			// payer's signature separately on execute.
			if _, err := node.Transaction.FreezeWith(session.Client.Hiero()); err != nil {
				return fmt.Errorf("node %d: freeze for admin-key co-sign: %w", node.Expected, err)
			}
			node.Transaction.Sign(adminKey)
		}

		result, err := session.Client.Submit(node.Transaction, nil)
		if err != nil {
			return fmt.Errorf("node %d: %w", node.Expected, err)
		}
		if err := ops.CheckNodeID(node.Expected, result.Receipt); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "node %d: created as node %d (transaction %s)\n",
			node.Expected, result.Receipt.NodeID, result.TransactionID)
	}
	return nil
}
