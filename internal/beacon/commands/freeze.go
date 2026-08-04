// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"fmt"
	"io"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/beacon/config"
	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

// updateFileNum is the default HFS special file for the software upgrade package.
const updateFileNum uint64 = 150

// telemetryFileNum is the default HFS special file for the telemetry upgrade.
const telemetryFileNum uint64 = 159

// FreezeCommands returns the freeze-family subcommands (design §6.2): all five
// FreezeType variants, each a single verified freeze submission behind the
// irreversible-action gate. resolve provides the per-invocation session.
func FreezeCommands(resolve SessionFunc) []*cobra.Command {
	c := defaultConfirmer()
	return []*cobra.Command{
		prepareUpgradeCmd(resolve, c),
		freezeUpgradeCmd(resolve, c),
		upgradeTelemetryCmd(resolve, c),
		freezeOnlyCmd(resolve, c),
		freezeAbortCmd(resolve, c),
	}
}

// buildFunc turns the resolved target into the concrete transaction to submit.
type buildFunc func(target *config.Target) (hiero.TransactionInterface, error)

// runFreeze is the shared flow: confirm the irreversible action, resolve the
// session, build the variant's transaction, submit it verified, and report the
// outcome. Confirmation happens before any key loading or network work.
func runFreeze(out io.Writer, resolve SessionFunc, c confirmer, yes bool, action string, build buildFunc) error {
	if err := c.confirm(yes, action); err != nil {
		return err
	}

	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	tx, err := build(session.Target)
	if err != nil {
		return err
	}

	result, err := session.Client.Submit(tx, nil)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%s: transaction %s reached %s\n", action, result.TransactionID, result.Receipt.Status)
	return nil
}

func prepareUpgradeCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var (
		hashHex string
		fileNum uint64
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "prepare-upgrade",
		Short: "Stage the upgrade software on all nodes (PREPARE_UPGRADE)",
		Long: "Submits a PREPARE_UPGRADE freeze that stages the software package (special file " +
			"0.0.150 by default) on every node, pinned to its SHA-384 hash. It does not halt the " +
			"network; follow it with freeze-upgrade to apply.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFreeze(cmd.OutOrStdout(), resolve, c, yes,
				"prepare the network for upgrade (stage software on all nodes)",
				func(target *config.Target) (hiero.TransactionInterface, error) {
					hash, err := ops.ParseUpgradeHash(hashHex)
					if err != nil {
						return nil, err
					}
					return ops.PrepareUpgrade(target.FileID(fileNum), hash), nil
				})
		},
	}
	cmd.Flags().StringVar(&hashHex, "upgrade-zip-hash", "", "Hex SHA-384 of the staged upgrade package (required)")
	cmd.Flags().Uint64Var(&fileNum, "upgrade-file-num", updateFileNum, "HFS special file number for the upgrade package")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	_ = cmd.MarkFlagRequired("upgrade-zip-hash")
	return cmd
}

func freezeUpgradeCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var (
		startTime string
		hashHex   string
		fileNum   uint64
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "freeze-upgrade",
		Short: "Freeze the network and apply the staged upgrade (FREEZE_UPGRADE)",
		Long: "Submits a FREEZE_UPGRADE freeze that, at --start-time, halts the network and applies the " +
			"software previously staged by prepare-upgrade. The --upgrade-zip-hash must match the one " +
			"passed to prepare-upgrade.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFreeze(cmd.OutOrStdout(), resolve, c, yes,
				"freeze the network and apply the staged upgrade",
				func(target *config.Target) (hiero.TransactionInterface, error) {
					start, err := ops.ParseStartTime(startTime)
					if err != nil {
						return nil, err
					}
					hash, err := ops.ParseUpgradeHash(hashHex)
					if err != nil {
						return nil, err
					}
					return ops.FreezeUpgrade(start, target.FileID(fileNum), hash), nil
				})
		},
	}
	cmd.Flags().StringVar(&startTime, "start-time", "", "UTC start time yyyy-MM-dd.HH:mm:ss (required)")
	cmd.Flags().StringVar(&hashHex, "upgrade-zip-hash", "", "Hex SHA-384 of the staged upgrade package; must match prepare-upgrade (required)")
	cmd.Flags().Uint64Var(&fileNum, "upgrade-file-num", updateFileNum, "HFS special file number for the upgrade package")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	_ = cmd.MarkFlagRequired("start-time")
	_ = cmd.MarkFlagRequired("upgrade-zip-hash")
	return cmd
}

func upgradeTelemetryCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var (
		startTime string
		hashHex   string
		fileNum   uint64
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade-telemetry",
		Short: "Apply the telemetry upgrade at a scheduled time (TELEMETRY_UPGRADE)",
		Long: "Submits a TELEMETRY_UPGRADE freeze that, at --start-time, applies the telemetry/observability " +
			"bundle staged in special file 0.0.159 by default, pinned to its SHA-384 hash.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFreeze(cmd.OutOrStdout(), resolve, c, yes,
				"apply the telemetry upgrade",
				func(target *config.Target) (hiero.TransactionInterface, error) {
					start, err := ops.ParseStartTime(startTime)
					if err != nil {
						return nil, err
					}
					hash, err := ops.ParseUpgradeHash(hashHex)
					if err != nil {
						return nil, err
					}
					return ops.UpgradeTelemetry(start, target.FileID(fileNum), hash), nil
				})
		},
	}
	cmd.Flags().StringVar(&startTime, "start-time", "", "UTC start time yyyy-MM-dd.HH:mm:ss (required)")
	cmd.Flags().StringVar(&hashHex, "upgrade-zip-hash", "", "Hex SHA-384 of the staged telemetry package (required)")
	cmd.Flags().Uint64Var(&fileNum, "upgrade-file-num", telemetryFileNum, "HFS special file number for the telemetry package")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	_ = cmd.MarkFlagRequired("start-time")
	_ = cmd.MarkFlagRequired("upgrade-zip-hash")
	return cmd
}

func freezeOnlyCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var (
		startTime string
		yes       bool
	)
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "Halt the network at a scheduled time without upgrading (FREEZE_ONLY)",
		Long: "Submits a FREEZE_ONLY freeze that halts the network at --start-time without changing any " +
			"software. Use freeze-abort to cancel it before it fires.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFreeze(cmd.OutOrStdout(), resolve, c, yes,
				"freeze the network at the given time",
				func(*config.Target) (hiero.TransactionInterface, error) {
					start, err := ops.ParseStartTime(startTime)
					if err != nil {
						return nil, err
					}
					return ops.FreezeOnly(start), nil
				})
		},
	}
	cmd.Flags().StringVar(&startTime, "start-time", "", "UTC start time yyyy-MM-dd.HH:mm:ss (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	_ = cmd.MarkFlagRequired("start-time")
	return cmd
}

func freezeAbortCmd(resolve SessionFunc, c confirmer) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "freeze-abort",
		Short: "Cancel a scheduled freeze or upgrade (FREEZE_ABORT)",
		Long:  "Submits a FREEZE_ABORT freeze that cancels a pending freeze/upgrade before its start time.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFreeze(cmd.OutOrStdout(), resolve, c, yes,
				"abort the scheduled freeze/upgrade",
				func(*config.Target) (hiero.TransactionInterface, error) {
					return ops.FreezeAbort(), nil
				})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the interactive confirmation (required for non-interactive use)")
	return cmd
}
