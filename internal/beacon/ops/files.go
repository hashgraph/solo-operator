// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// bytesPerOp is the per-operation payload size for special-file upload, matching
// yahcli's default: the initial FileUpdate and every FileAppend carry at most
// this many bytes (design §8.1).
const bytesPerOp = 4096

// DefaultCheckpointEvery is how often the upload driver awaits a receipt (rather
// than firing) during a bulk append. It bounds how far the running hash can
// silently diverge before a rejected chunk surfaces: a failure is caught within
// this many chunks instead of only at the final hash read-back, so resume has
// little to redo. Callers may override it via Uploader.CheckpointEvery.
const DefaultCheckpointEvery = 128

// SubmitFunc submits one transaction. Depending on which Uploader seam it backs
// it either fires the transaction (returns at precheck) or awaits its SUCCESS
// receipt. It is the seam between the network-free upload driver here and the
// client package, and lets the driver be unit-tested without a node.
type SubmitFunc func(tx hiero.TransactionInterface) error

// Uploader drives special-file upload transactions. It separates the fast Fire
// path (submit + precheck, no receipt) used for the bulk of the append chunks
// from the Verify path (submit + await SUCCESS receipt) used for the initial
// FileUpdate and periodic checkpoints. Firing most chunks lets a large upload run
// at the node's ingest rate instead of one consensus round-trip per chunk, while
// the checkpoints still catch a rejected chunk early; the caller's final on-chain
// hash read-back verifies the upload as a whole. Both seams are injected so the
// driver stays unit-testable without a node.
type Uploader struct {
	// Fire submits a transaction and returns at precheck, without a receipt.
	Fire SubmitFunc
	// Verify submits a transaction and awaits its SUCCESS receipt.
	Verify SubmitFunc
	// ReadMemo reads the target file's running-hash memo. When set, each checkpoint
	// compares it against the expected SHA-384 of the chunks staged so far, so a
	// fired chunk that silently failed is caught within CheckpointEvery chunks
	// rather than only at the final read-back. Leave nil for files whose memo is
	// not a running hash (checkpoints then rely on the append receipt alone).
	ReadMemo MemoFunc
	// CheckpointEvery awaits a receipt every N append chunks; <=0 uses the default.
	CheckpointEvery int
}

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
// defaults are bypassed by driving the loop ourselves for progress control and
// checkpointing). It is generic over the target file, so the telemetry file
// (0.0.159) reuses it.
func UploadRawFile(u Uploader, fileID hiero.FileID, data []byte) error {
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
	return uploadChunks(u, fileID, chunks, 0)
}

// ResumeUpload continues an interrupted UploadRawFile without re-sending chunks
// the network already has (design §8.2). It reads fileID's memo (the running
// SHA-384 of the staged contents), matches it to a chunk boundary — checking
// only boundaries, O(#chunks) — and resumes FileAppend from the first missing
// chunk. It never re-sends the initial FileUpdate past a match (that would
// truncate the file); if no boundary matches (empty/unreadable memo, or a
// mismatch), it falls back to a full upload from chunk 0, which FileUpdate
// safely resets.
func ResumeUpload(u Uploader, readMemo MemoFunc, fileID hiero.FileID, data []byte) error {
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
	return uploadChunks(u, fileID, chunks, done)
}

// uploadChunks writes chunks[start:] to fileID. When start is 0 the first op is
// a FileUpdate (replacing the file's contents with chunk 0) and appends follow;
// when start is greater than 0 it emits FileAppend only, resuming from that
// chunk without disturbing the chunks already on-chain. Each append is forced to
// a single chunk (the payload never exceeds bytesPerOp).
//
// The initial FileUpdate and every u.CheckpointEvery-th append await a receipt
// (Verify); the remaining appends are fired without waiting (Fire), which is the
// bulk of the speed-up. Because appends are submitted in order to a single node
// they are applied in that order at consensus, so the running hash stays correct.
// At each checkpoint, when u.ReadMemo is set, the on-chain running-hash memo is
// compared against the expected SHA-384 of everything staged so far — so a fired
// chunk that silently failed is caught within CheckpointEvery chunks (bounding
// what resume must redo) rather than only at the caller's final read-back.
func uploadChunks(u Uploader, fileID hiero.FileID, chunks [][]byte, start int) error {
	total := len(chunks)
	every := u.CheckpointEvery
	if every <= 0 {
		every = DefaultCheckpointEvery
	}
	t0 := time.Now()

	// running mirrors the network's incremental SHA-384 of the staged contents.
	// Seed it with the chunks already on-chain when resuming from start > 0.
	running := sha512.New384()
	for k := 0; k < start; k++ {
		running.Write(chunks[k])
	}
	// checkpoint compares the on-chain memo against running after chunk done-1 (0
	// based) has been applied. A SUCCESS receipt on the checkpoint append means it
	// (and every earlier chunk, by consensus order) is applied, so the memo is a
	// valid cumulative check.
	checkpoint := func(done int) error {
		if u.ReadMemo == nil {
			return nil
		}
		memo, err := u.ReadMemo(fileID)
		if err != nil {
			return err
		}
		want := running.Sum(nil)
		if !bytes.Equal(decodeMemo(fileID, memo), want) {
			return errx.Decorate(
				errValidation.New("staged hash %s does not match expected %s at chunk %d/%d",
					strings.TrimSpace(memo), hex.EncodeToString(want), done, total),
				reasonCheckpointHashMismatch,
				"An earlier chunk did not apply; re-run the upload to resume and re-send the missing chunks",
			)
		}
		logx.As().Info().
			Int("chunk", done).
			Int("chunks", total).
			Str("file", fileID.String()).
			Int("percent", done*100/total).
			Str("hash", hex.EncodeToString(want)).
			Str("elapsed", time.Since(t0).Truncate(time.Millisecond).String()).
			Msg("checkpoint verified: staged hash matches")
		return nil
	}

	i := start
	if start == 0 {
		// Initial op: FileUpdate with chunk 0 (replaces the file's contents). Always
		// verified — the entire upload is appended onto it.
		if err := u.Verify(hiero.NewFileUpdateTransaction().SetFileID(fileID).SetContents(chunks[0])); err != nil {
			// The submit error is already decorated at the client boundary; wrap
			// for context with %w so its reason survives.
			return fmt.Errorf("initial file update (chunk 1/%d): %w", total, err)
		}
		running.Write(chunks[0])
		logUploadProgress(fileID, 1, total, t0)
		if err := checkpoint(1); err != nil {
			return err
		}
		i = 1
	}
	for ; i < total; i++ {
		tx := hiero.NewFileAppendTransaction().
			SetFileID(fileID).
			SetContents(chunks[i]).
			SetMaxChunkSize(bytesPerOp).
			SetMaxChunks(1)
		// Await a receipt at each checkpoint boundary and on the final chunk; fire
		// the rest without waiting.
		isCheckpoint := i == total-1 || (i+1)%every == 0
		submit := u.Fire
		if isCheckpoint {
			submit = u.Verify
		}
		if err := submit(tx); err != nil {
			return fmt.Errorf("append chunk %d/%d: %w", i+1, total, err)
		}
		running.Write(chunks[i])
		logUploadProgress(fileID, i+1, total, t0)
		if isCheckpoint {
			if err := checkpoint(i + 1); err != nil {
				return err
			}
		}
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

// logUploadProgress emits a per-chunk debug line (so a --log-level debug run shows
// live chunk progress alongside each SUCCESS receipt) plus an info line at each 10%
// boundary and at completion (so a normal run reports progress without a line per
// chunk).
func logUploadProgress(fileID hiero.FileID, done, total int, t0 time.Time) {
	if total == 0 {
		return
	}
	pct := done * 100 / total
	elapsed := time.Since(t0).Truncate(time.Millisecond).String()
	logx.As().Debug().
		Str("reason", reasonUploadProgress.String()).
		Uint64("file", fileID.File).
		Int("chunk", done).
		Int("chunks", total).
		Int("percent", pct).
		Str("elapsed", elapsed).
		Msg("chunk uploaded")
	prev := (done - 1) * 100 / total
	if done == total || pct/10 != prev/10 {
		logx.As().Info().
			Str("reason", reasonUploadProgress.String()).
			Uint64("file", fileID.File).
			Int("chunk", done).
			Int("chunks", total).
			Int("percent", pct).
			Str("elapsed", elapsed).
			Msg("upload progress")
	}
}
