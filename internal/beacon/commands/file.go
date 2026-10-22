// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
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

const defaultMaxPackageSize int64 = 1 << 30

// FileCommands returns the `file` command group for Hedera File Service
// operations: `file upload` and `file info`, each targeting an HFS file by
// its bare file number via --file-id (default 150, the upgrade special file).
func FileCommands(resolve SessionFunc) *cobra.Command {
	fileCmd := &cobra.Command{
		Use:   "file",
		Short: "Operate on Hedera File Service files",
		Long: "Commands that read and write Hedera File Service files by id. " +
			"Use --file-id to target a specific file (default: 150, the upgrade special file).",
	}
	fileCmd.AddCommand(uploadCmd(resolve))
	fileCmd.AddCommand(infoCmd(resolve))
	return fileCmd
}

func infoCmd(resolve SessionFunc) *cobra.Command {
	var fileNum uint64
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Print the SHA-384 the network holds for an HFS file",
		Long: "Reads an HFS file via FileInfoQuery and prints the running SHA-384 stored in its " +
			"memo — the canonical hash H the network validated on upload and that `freeze prepare`/" +
			"`freeze upgrade` must carry (design §7). Defaults to special file 0.0.150; use " +
			"--file-id 159 for the telemetry file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFileInfo(cmd, resolve, fileNum)
		},
	}
	cmd.Flags().Uint64Var(&fileNum, "file-id", updateFileNum, "HFS file number to inspect (e.g. 150, 159)")
	return cmd
}

func runFileInfo(cmd *cobra.Command, resolve SessionFunc, fileNum uint64) error {
	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	fileID := session.Target.FileID(fileNum)
	info, err := ops.FileInfoOf(session.Client.Hiero(), fileID)
	if err != nil {
		return err
	}

	memo := strings.TrimSpace(info.FileMemo)
	if memo == "" {
		memo = "(empty — no hash staged)"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"File %s\n  SHA-384: %s\n  size:    %d bytes\n",
		fileID, memo, info.Size)
	return nil
}

func uploadCmd(resolve SessionFunc) *cobra.Command {
	var (
		zipPath         string
		fileNum         uint64
		expectHash      string
		expectHashFile  string
		fromScratch     bool
		checkpointEvery int
		maxSize         int64
	)
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload the deployment package zip to an HFS special file",
		Long: "Stages the hip-1494 deployment package into the target HFS file (default 0.0.150) by an " +
			"initial FileUpdate followed by chunked FileAppend. Use --file-id to target another special " +
			"file. With --expect-hash/--expect-hash-file it verifies the local zip's SHA-384 before " +
			"writing anything, and on success prints the confirmed hash for `freeze prepare`. By default " +
			"it resumes an interrupted upload by matching the on-chain running hash to a chunk boundary " +
			"and re-sending only the missing chunks (a fresh file simply uploads from the start); pass " +
			"--from-scratch to force a full re-upload of every chunk. Appends are fired without waiting " +
			"for a receipt; --checkpoint-every controls how often the upload awaits a receipt and verifies " +
			"the on-chain running hash.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFileUpload(cmd, resolve, zipPath, fileNum, expectHash, expectHashFile, fromScratch, checkpointEvery, maxSize)
		},
	}
	cmd.Flags().StringVar(&zipPath, "zip", "", "Path to the package zip (default: <config-dir>/sysfiles/"+defaultSoftwareZipName+")")
	cmd.Flags().Uint64Var(&fileNum, "file-id", updateFileNum, "HFS file number for the target file (e.g. 150, 159)")
	cmd.Flags().StringVar(&expectHash, "expect-hash", "", "Expected SHA-384 hex; abort before upload if the zip does not match")
	cmd.Flags().StringVar(&expectHashFile, "expect-hash-file", "", "File with the expected SHA-384 (bare hex or sha384sum format)")
	cmd.Flags().BoolVar(&fromScratch, "from-scratch", false, "Ignore any partially-staged file and re-upload every chunk from the beginning")
	cmd.Flags().IntVar(&checkpointEvery, "checkpoint-every", ops.DefaultCheckpointEvery,
		"Await a receipt and verify the on-chain running hash every N appended chunks. Higher is faster on "+
			"high-latency networks (fewer receipt waits) but re-sends more on a failure; must be >= 1")
	cmd.Flags().Int64Var(&maxSize, "max-size", defaultMaxPackageSize, "Refuse a package zip larger than this many bytes")
	cmd.MarkFlagsMutuallyExclusive("expect-hash", "expect-hash-file")
	return cmd
}

func runFileUpload(cmd *cobra.Command, resolve SessionFunc, zipPath string, fileNum uint64, expectHash, expectHashFile string, fromScratch bool, checkpointEvery int, maxSize int64) error {
	session, err := resolve()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	if err := assertPrivilegedPayer(session.Target.Payer); err != nil {
		return err
	}

	if checkpointEvery < 1 {
		return errx.Decorate(
			errInput.New("--checkpoint-every must be >= 1, got %d", checkpointEvery),
			reasonInvalidCheckpoint,
			"Pass a positive value; higher values checkpoint less often (faster, but re-send more on failure)",
		)
	}

	if zipPath == "" {
		zipPath = filepath.Join(session.ConfigDir, "sysfiles", defaultSoftwareZipName)
	}
	data, err := readPackage(zipPath, maxSize)
	if err != nil {
		return err
	}

	localRaw := ops.HashBytes(data)
	localHex := hex.EncodeToString(localRaw)

	if want, provided, err := expectedHash(expectHash, expectHashFile); err != nil {
		return err
	} else if provided && !bytes.Equal(want, localRaw) {
		return errx.Decorate(
			errInput.New("local package hash %s does not match expected %s", localHex, hex.EncodeToString(want)),
			reasonExpectedHashMismatch,
			"Nothing was uploaded. The package on disk is not the one you expected — re-check --expect-hash or the zip",
		)
	}

	fileID := session.Target.FileID(fileNum)
	readMemo := func(id hiero.FileID) (string, error) {
		return ops.FileMemo(session.Client.Hiero(), id)
	}
	uploader := ops.Uploader{
		Fire: func(tx hiero.TransactionInterface) error {
			return session.Client.Fire(tx)
		},
		Verify: func(tx hiero.TransactionInterface) error {
			_, e := session.Client.Submit(tx, nil)
			return e
		},
		ReadMemo:        readMemo,
		CheckpointEvery: checkpointEvery,
	}
	if fromScratch {
		if err := ops.UploadRawFile(uploader, fileID, data); err != nil {
			return err
		}
	} else {
		if err := ops.ResumeUpload(uploader, readMemo, fileID, data); err != nil {
			return err
		}
	}

	if err := ops.VerifyUploadedHash(session.Client.Hiero(), fileID, localRaw); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Uploaded %s to %s\nConfirmed SHA-384: %s\nNext: beacon freeze prepare --upgrade-zip-hash %s\n",
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
			errInput.New("payer account %d is not privileged for special-file upload", payer.Account),
			reasonNotPrivilegedPayer,
			"Use a privileged payer with --payer/-p: 2, 50, or 58",
		)
	}
}

func readPackage(path string, maxSize int64) ([]byte, error) {
	readErr := func(err error) error {
		return errx.Decorate(
			errInput.Wrap(err, "read package %q", path),
			reasonZipReadFailed,
			fmt.Sprintf("Confirm the package exists at %q, or pass --zip", path),
		)
	}
	tooLarge := func(size int64) error {
		return errx.Decorate(
			errInput.New("package %q is at least %d bytes, over the --max-size limit of %d", path, size, maxSize),
			reasonZipTooLarge,
			"Confirm --zip points at the deployment package, or raise --max-size if it is legitimately larger",
		)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, readErr(err)
	}
	if !info.Mode().IsRegular() {
		return nil, errx.Decorate(
			errInput.New("package %q is not a regular file (mode %s)", path, info.Mode()),
			reasonZipNotRegular,
			"Pass --zip with the path of the package zip file itself",
		)
	}
	if info.Size() > maxSize {
		return nil, tooLarge(info.Size())
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, readErr(err)
	}
	defer func() { _ = f.Close() }()
	buf := bytes.NewBuffer(make([]byte, 0, info.Size()+1))
	if _, err := buf.ReadFrom(io.LimitReader(f, maxSize+1)); err != nil {
		return nil, readErr(err)
	}
	if int64(buf.Len()) > maxSize {
		return nil, tooLarge(int64(buf.Len()))
	}
	return buf.Bytes(), nil
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
				errInput.Wrap(err, "read expect-hash file %q", fileFlag),
				reasonExpectHashReadFailed,
				fmt.Sprintf("Confirm %q exists and is readable", fileFlag),
			)
		}
		fields := strings.Fields(string(content))
		if len(fields) == 0 {
			return nil, false, errx.Decorate(
				errInput.New("expect-hash file %q is empty", fileFlag),
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
