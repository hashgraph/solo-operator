// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

// NetworkCommand returns the `network` command group for read-only queries about
// the connected Hedera network. The MVP wires `network version`; balances and
// other probes are later stories.
func NetworkCommand(resolve SessionFunc) *cobra.Command {
	net := &cobra.Command{
		Use:   "network",
		Short: "Query the connected Hedera network",
	}
	net.AddCommand(networkVersionCmd(resolve))
	return net
}

// networkVersionCmd queries the connected network for its active build (services
// + HAPI version) via NetworkVersionInfoQuery. It is the post-restart
// confirmation step of the upgrade flow (design §7 step 5). beacon's own build
// metadata is a separate command (`beacon version`).
func networkVersionCmd(resolve SessionFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the network's active build (services + HAPI version)",
		Long: "Queries the connected network via NetworkVersionInfoQuery and prints the active Hedera " +
			"services version and HAPI protobuf version. Run it after freeze-upgrade completes to confirm " +
			"the new build is live (design §7 step 5). For beacon's own build, use `beacon version`.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runNetworkVersion(cmd, resolve)
		},
	}
}

func runNetworkVersion(cmd *cobra.Command, resolve SessionFunc) error {
	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	info, err := ops.NetworkVersion(session.Client.Hiero())
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Services version: %s\nHAPI version:     %s\n",
		ops.SemverString(info.ServicesVersion),
		ops.SemverString(info.ProtobufVersion))
	return nil
}
