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

// MemoFunc returns a file's current memo — where the network records the running
// SHA-384 of a special file's staged contents (§7), as hex. It is the read seam
// for resume, mirroring SubmitFunc on the write side, and lets ResumeUpload be
// unit-tested without a node.
type MemoFunc func(fileID hiero.FileID) (string, error)

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
		return errEmptyUpload(fileID)
	}
	chunks := chunkBytes(data, bytesPerOp)
	logx.As().Info().
		Str("reason", reasonUploadStarted.String()).
		Uint64("file", fileID.File).
		Int("bytes", len(data)).
		Int("chunks", len(chunks)).
		Msg("uploading special file")
	return uploadChunks(submit, fileID, chunks, 0)
}

// ResumeUpload continues an interrupted UploadRawFile without re-sending chunks
// the network already has (design §8.2). It reads fileID's memo (the running
// SHA-384 of the staged contents), matches it to a chunk boundary — checking
// only boundaries, O(#chunks) — and resumes FileAppend from the first missing
// chunk. It never re-sends the initial FileUpdate past a match (that would
// truncate the file); if no boundary matches (empty/unreadable memo, or a
// mismatch), it falls back to a full upload from chunk 0, which FileUpdate
// safely resets.
func ResumeUpload(submit SubmitFunc, readMemo MemoFunc, fileID hiero.FileID, data []byte) error {
	if len(data) == 0 {
		return errEmptyUpload(fileID)
	}
	chunks := chunkBytes(data, bytesPerOp)
	total := len(chunks)

	memo, err := readMemo(fileID)
	if err != nil {
		return err
	}
	done := matchRunningHash(chunks, decodeMemo(fileID, memo))

	if done >= total {
		// Every chunk's cumulative hash is already staged — nothing to re-send.
		logx.As().Info().
			Str("reason", reasonUploadResumed.String()).
			Uint64("file", fileID.File).
			Int("chunks", total).
			Msg("upload already complete; nothing to resume")
		return nil
	}
	logx.As().Info().
		Str("reason", reasonUploadResumed.String()).
		Uint64("file", fileID.File).
		Int("bytes", len(data)).
		Int("chunks", total).
		Int("resume_from", done).
		Msg("resuming special-file upload")
	return uploadChunks(submit, fileID, chunks, done)
}

// uploadChunks writes chunks[start:] to fileID. When start is 0 the first op is
// a FileUpdate (replacing the file's contents with chunk 0) and appends follow;
// when start is greater than 0 it emits FileAppend only, resuming from that
// chunk without disturbing the chunks already on-chain. Each append is forced to
// a single chunk (the payload never exceeds bytesPerOp).
func uploadChunks(submit SubmitFunc, fileID hiero.FileID, chunks [][]byte, start int) error {
	total := len(chunks)
	i := start
	if start == 0 {
		// Initial op: FileUpdate with chunk 0 (replaces the file's contents).
		if err := submit(hiero.NewFileUpdateTransaction().SetFileID(fileID).SetContents(chunks[0])); err != nil {
			// The submit error is already decorated at the client boundary; wrap
			// for context with %w so its reason survives.
			return fmt.Errorf("initial file update (chunk 1/%d): %w", total, err)
		}
		logUploadProgress(fileID, 1, total)
		i = 1
	}
	for ; i < total; i++ {
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

// matchRunningHash returns how many leading chunks are already staged: the count
// k for which sha384(chunks[0] || … || chunks[k-1]) equals memoRaw. It advances
// an incremental hash one chunk at a time, comparing only at chunk boundaries
// (never at intermediate byte offsets), so it is O(#chunks) comparisons over
// O(len(data)) total hashing. It returns 0 when no boundary matches, including a
// nil/short memoRaw.
func matchRunningHash(chunks [][]byte, memoRaw []byte) int {
	if len(memoRaw) != sha384Len {
		return 0
	}
	h := sha512.New384()
	for k, c := range chunks {
		h.Write(c)
		// Sum snapshots the digest of everything written so far without
		// resetting the state, so each boundary reuses the prior work.
		if bytes.Equal(h.Sum(nil), memoRaw) {
			return k + 1
		}
	}
	return 0
}

// decodeMemo hex-decodes a file memo into raw hash bytes, returning nil (which
// matchRunningHash treats as "no progress" → full re-upload) when the memo is
// empty or not valid hex.
func decodeMemo(fileID hiero.FileID, memo string) []byte {
	memo = strings.TrimSpace(memo)
	if memo == "" {
		return nil
	}
	raw, err := hex.DecodeString(memo)
	if err != nil {
		logx.As().Warn().
			Str("reason", reasonUploadResumed.String()).
			Uint64("file", fileID.File).
			Str("memo", memo).
			Msg("file memo is not hex; restarting upload from the beginning")
		return nil
	}
	return raw
}

// errEmptyUpload is the shared guard for a zero-length payload.
func errEmptyUpload(fileID hiero.FileID) error {
	return errx.Decorate(
		errValidation.New("nothing to upload to %s", fileID),
		reasonEmptyUpload,
		"Confirm the package file exists and is non-empty",
	)
}

// VerifyUploadedHash reads fileID's info and asserts its memo — where the network
// records the running SHA-384 of a special file (§7) — equals wantRaw. This is
// the post-upload read-back that makes "success" mean the staged file is
// observably correct (§5), not merely that the writes were accepted.
func VerifyUploadedHash(hc *hiero.Client, fileID hiero.FileID, wantRaw []byte) error {
	memo, err := FileMemo(hc, fileID)
	if err != nil {
		return err
	}

	// The special-file memo is the hex SHA-384 of the staged contents.
	got, err := hex.DecodeString(memo)
	if err != nil {
		return errx.Decorate(
			errValidation.New("on-chain memo for %s is not a hex hash: %q", fileID, memo),
			reasonUploadedHashMismatch,
			"The staged file's memo is not the expected SHA-384; re-upload the package",
		)
	}
	if !bytes.Equal(got, wantRaw) {
		return errx.Decorate(
			errValidation.New("staged hash %s does not match uploaded %s", memo, hex.EncodeToString(wantRaw)),
			reasonUploadedHashMismatch,
			"The staged file differs from the local package; re-upload before prepare-upgrade",
		)
	}
	return nil
}

// FileInfoOf reads fileID's on-chain metadata (size, expiry, keys, memo) via
// FileInfoQuery. For special files the memo holds the running SHA-384 of the
// staged contents (§7).
func FileInfoOf(hc *hiero.Client, fileID hiero.FileID) (hiero.FileInfo, error) {
	info, err := hiero.NewFileInfoQuery().SetFileID(fileID).Execute(hc)
	if err != nil {
		return hiero.FileInfo{}, errx.Decorate(
			errQuery.Wrap(err, "query info for %s", fileID),
			reasonFileInfoFailed,
			"Confirm the node is reachable and the payer can pay the query fee",
		)
	}
	return info, nil
}

// FileMemo reads fileID's memo — the running SHA-384 of a special file's staged
// contents (§7) — trimmed. It backs both the post-upload read-back
// (VerifyUploadedHash) and resume's boundary match (ResumeUpload).
func FileMemo(hc *hiero.Client, fileID hiero.FileID) (string, error) {
	info, err := FileInfoOf(hc, fileID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(info.FileMemo), nil
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
