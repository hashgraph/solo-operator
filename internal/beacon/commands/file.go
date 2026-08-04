// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
	"github.com/spf13/cobra"

	"github.com/hashgraph/solo-operator/internal/beacon/ops"
)

// defaultSoftwareZipName is the conventional package filename under a network's
// sysfiles/ directory (design §10).
const defaultSoftwareZipName = "softwareUpgrade.zip"

// FileCommands returns the `file` command group for Hedera File Service
// operations. The MVP wires only `file upload software-zip` (the special-file
// upload path, design §8.1); download/info are later stories.
func FileCommands(resolve SessionFunc) *cobra.Command {
	fileCmd := &cobra.Command{
		Use:   "file",
		Short: "Operate on Hedera File Service files",
		Long: "Commands that read and write Hedera File Service files by id. The MVP covers " +
			"uploading the deployment package to the upgrade special file (0.0.150).",
	}
	upload := &cobra.Command{
		Use:   "upload",
		Short: "Upload contents to an HFS file",
	}
	upload.AddCommand(softwareZipUploadCmd(resolve))
	fileCmd.AddCommand(upload)
	return fileCmd
}

func softwareZipUploadCmd(resolve SessionFunc) *cobra.Command {
	var (
		zipPath        string
		fileNum        uint64
		expectHash     string
		expectHashFile string
		restart        bool
	)
	cmd := &cobra.Command{
		Use:   "software-zip",
		Short: "Upload the deployment package zip to the upgrade special file (0.0.150)",
		Long: "Stages the hip-1494 deployment package into special file 0.0.150 by an initial FileUpdate " +
			"followed by chunked FileAppend. It is a convenience alias for `file upload 150`. With " +
			"--expect-hash/--expect-hash-file it verifies the local zip's SHA-384 before writing anything, " +
			"and on success prints the confirmed hash for `prepare-upgrade`. --restart-from-failure resumes " +
			"an interrupted upload by matching the on-chain running hash to a chunk boundary and re-sending " +
			"only the missing chunks.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSoftwareZipUpload(cmd, resolve, zipPath, fileNum, expectHash, expectHashFile, restart)
		},
	}
	cmd.Flags().StringVar(&zipPath, "zip", "", "Path to the package zip (default: <config-dir>/sysfiles/"+defaultSoftwareZipName+")")
	cmd.Flags().Uint64Var(&fileNum, "upgrade-file-num", updateFileNum, "HFS special file number for the upgrade package")
	cmd.Flags().StringVar(&expectHash, "expect-hash", "", "Expected SHA-384 hex; abort before upload if the zip does not match")
	cmd.Flags().StringVar(&expectHashFile, "expect-hash-file", "", "File with the expected SHA-384 (bare hex or sha384sum format)")
	cmd.Flags().BoolVar(&restart, "restart-from-failure", false, "Resume an interrupted upload, re-sending only the chunks the network is missing")
	cmd.MarkFlagsMutuallyExclusive("expect-hash", "expect-hash-file")
	return cmd
}

func runSoftwareZipUpload(cmd *cobra.Command, resolve SessionFunc, zipPath string, fileNum uint64, expectHash, expectHashFile string, restart bool) error {
	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	// Special-file writes require a privileged payer.
	if err := assertPrivilegedPayer(session.Target.Payer); err != nil {
		return err
	}

	if zipPath == "" {
		zipPath = filepath.Join(session.ConfigDir, "sysfiles", defaultSoftwareZipName)
	}
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return errx.Decorate(
			fmt.Errorf("read package %q: %w", zipPath, err),
			reasonZipReadFailed,
			fmt.Sprintf("Confirm the package exists at %q, or pass --zip", zipPath),
		)
	}

	localRaw := ops.HashBytes(data)
	localHex := hex.EncodeToString(localRaw)

	// Verify-before-upload (§15 Q1): abort before any on-chain write on mismatch.
	if want, provided, err := expectedHash(expectHash, expectHashFile); err != nil {
		return err
	} else if provided && !bytes.Equal(want, localRaw) {
		return errx.Decorate(
			fmt.Errorf("local package hash %s does not match expected %s", localHex, hex.EncodeToString(want)),
			reasonExpectedHashMismatch,
			"Nothing was uploaded. The package on disk is not the one you expected — re-check --expect-hash or the zip",
		)
	}

	fileID := session.Target.FileID(fileNum)
	submit := func(tx hiero.TransactionInterface) error {
		_, e := session.Client.Submit(tx, nil)
		return e
	}
	if restart {
		readMemo := func(id hiero.FileID) (string, error) {
			return ops.FileMemo(session.Client.Hiero(), id)
		}
		if err := ops.ResumeUpload(submit, readMemo, fileID, data); err != nil {
			return err
		}
	} else if err := ops.UploadRawFile(submit, fileID, data); err != nil {
		return err
	}

	// Read back the staged hash and confirm it equals the local package.
	if err := ops.VerifyUploadedHash(session.Client.Hiero(), fileID, localRaw); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Uploaded %s to %s\nConfirmed SHA-384: %s\nNext: beacon prepare-upgrade --upgrade-zip-hash %s\n",
		zipPath, fileID, localHex, localHex)
	return nil
}

// assertPrivilegedPayer rejects a payer that is not one of the privileged upgrade
// accounts (0.0.2, 0.0.50, 0.0.58), which special-file writes require (design §10).
func assertPrivilegedPayer(payer hiero.AccountID) error {
	switch payer.Account {
	case 2, 50, 58:
		return nil
	default:
		return errx.Decorate(
			fmt.Errorf("payer account %d is not privileged for special-file upload", payer.Account),
			reasonNotPrivilegedPayer,
			"Use a privileged payer with --payer/-p: 2, 50, or 58",
		)
	}
}

// expectedHash resolves the optional expected SHA-384 from the mutually exclusive
// --expect-hash / --expect-hash-file flags. The file may be bare hex, or in
// sha384sum layout where the hash is the first whitespace-delimited token
// followed by the filename. It returns (hash, provided, error); provided is
// false when neither flag is set.
func expectedHash(hashFlag, fileFlag string) (hash []byte, provided bool, err error) {
	switch {
	case hashFlag != "":
		raw, err := ops.ParseUpgradeHash(hashFlag)
		return raw, true, err
	case fileFlag != "":
		content, err := os.ReadFile(fileFlag)
		if err != nil {
			return nil, false, errx.Decorate(
				fmt.Errorf("read expect-hash file %q: %w", fileFlag, err),
				reasonExpectHashReadFailed,
				fmt.Sprintf("Confirm %q exists and is readable", fileFlag),
			)
		}
		fields := strings.Fields(string(content))
		if len(fields) == 0 {
			return nil, false, errx.Decorate(
				fmt.Errorf("expect-hash file %q is empty", fileFlag),
				reasonExpectHashInvalid,
				"The file must contain the SHA-384 hex (optionally sha384sum format)",
			)
		}
		raw, err := ops.ParseUpgradeHash(fields[0])
		return raw, true, err
	default:
		return nil, false, nil
	}
}
