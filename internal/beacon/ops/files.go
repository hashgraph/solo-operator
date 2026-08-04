// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// bytesPerOp is the per-operation payload size for special-file upload, matching
// yahcli's default: the initial FileUpdate and every FileAppend carry at most
// this many bytes (design §8.1).
const bytesPerOp = 4096

// SubmitFunc submits one transaction and verifies its outcome (SUCCESS). It is
// the seam between the network-free upload driver here and the client package's
// verified-submit helper, and lets UploadRawFile be unit-tested without a node.
type SubmitFunc func(tx hiero.TransactionInterface) error

// HashBytes returns the SHA-384 digest of data — the hash contract the network
// tracks for special files (§7) and the value passed to prepare-upgrade.
func HashBytes(data []byte) []byte {
	sum := sha512.Sum384(data)
	return sum[:]
}

// UploadRawFile writes data verbatim into fileID: an initial FileUpdate carrying
// the first chunk, then sequential FileAppend chunks of up to bytesPerOp until
// the file is fully written (design §8.1, §9 gaps 1–2 — the SDK's append
// defaults are bypassed by driving the loop ourselves for progress control). It
// is generic over the target file, so the telemetry file (0.0.159) reuses it.
func UploadRawFile(submit SubmitFunc, fileID hiero.FileID, data []byte) error {
	if len(data) == 0 {
		return errx.Decorate(
			fmt.Errorf("nothing to upload to %s", fileID),
			reasonEmptyUpload,
			"Confirm the package file exists and is non-empty",
		)
	}

	chunks := chunkBytes(data, bytesPerOp)
	total := len(chunks)
	logx.As().Info().
		Str("reason", reasonUploadStarted.String()).
		Uint64("file", fileID.File).
		Int("bytes", len(data)).
		Int("chunks", total).
		Msg("uploading special file")

	// Initial op: FileUpdate with chunk 0 (replaces the file's contents).
	if err := submit(hiero.NewFileUpdateTransaction().SetFileID(fileID).SetContents(chunks[0])); err != nil {
		// The submit error is already decorated at the client boundary; wrap for
		// context with %w so its reason survives.
		return fmt.Errorf("initial file update (chunk 1/%d): %w", total, err)
	}
	logUploadProgress(fileID, 1, total)

	// Remaining ops: one FileAppend per chunk. SetMaxChunkSize keeps each a
	// single append (the payload never exceeds bytesPerOp).
	for i := 1; i < total; i++ {
		tx := hiero.NewFileAppendTransaction().
			SetFileID(fileID).
			SetContents(chunks[i]).
			SetMaxChunkSize(bytesPerOp).
			SetMaxChunks(1)
		if err := submit(tx); err != nil {
			return fmt.Errorf("append chunk %d/%d: %w", i+1, total, err)
		}
		logUploadProgress(fileID, i+1, total)
	}
	return nil
}

// VerifyUploadedHash reads fileID's info and asserts its memo — where the network
// records the running SHA-384 of a special file (§7) — equals wantRaw. This is
// the post-upload read-back that makes "success" mean the staged file is
// observably correct (§5), not merely that the writes were accepted.
func VerifyUploadedHash(hc *hiero.Client, fileID hiero.FileID, wantRaw []byte) error {
	info, err := hiero.NewFileInfoQuery().SetFileID(fileID).Execute(hc)
	if err != nil {
		return errx.Decorate(
			fmt.Errorf("query info for %s: %w", fileID, err),
			reasonFileInfoFailed,
			"Confirm the node is reachable and the payer can pay the query fee",
		)
	}

	// The special-file memo is the hex SHA-384 of the staged contents.
	memo := strings.TrimSpace(info.FileMemo)
	got, err := hex.DecodeString(memo)
	if err != nil {
		return errx.Decorate(
			fmt.Errorf("on-chain memo for %s is not a hex hash: %q", fileID, memo),
			reasonUploadedHashMismatch,
			"The staged file's memo is not the expected SHA-384; re-upload the package",
		)
	}
	if !bytes.Equal(got, wantRaw) {
		return errx.Decorate(
			fmt.Errorf("staged hash %s does not match uploaded %s", memo, hex.EncodeToString(wantRaw)),
			reasonUploadedHashMismatch,
			"The staged file differs from the local package; re-upload before prepare-upgrade",
		)
	}
	return nil
}

// chunkBytes splits data into consecutive slices of at most size bytes. The
// slices alias data (no copy); the caller must not mutate them. Empty data
// yields nil.
func chunkBytes(data []byte, size int) [][]byte {
	if len(data) == 0 {
		return nil
	}
	chunks := make([][]byte, 0, (len(data)+size-1)/size)
	for i := 0; i < len(data); i += size {
		end := i + size
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}

// logUploadProgress emits an info line at each 10% boundary and at completion, so
// a large upload reports progress without a line per chunk.
func logUploadProgress(fileID hiero.FileID, done, total int) {
	if total == 0 {
		return
	}
	pct := done * 100 / total
	prev := (done - 1) * 100 / total
	if done == total || pct/10 != prev/10 {
		logx.As().Info().
			Str("reason", reasonUploadProgress.String()).
			Uint64("file", fileID.File).
			Int("chunk", done).
			Int("chunks", total).
			Int("percent", pct).
			Msg("upload progress")
	}
}
