// SPDX-License-Identifier: Apache-2.0

package ops

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

func TestChunkBytes(t *testing.T) {
	cases := []struct {
		name string
		len  int
		size int
		want []int // expected chunk lengths
	}{
		{"empty", 0, 4, nil},
		{"smaller than size", 3, 4, []int{3}},
		{"exact multiple", 8, 4, []int{4, 4}},
		{"remainder", 10, 4, []int{4, 4, 2}},
		{"single byte", 1, 4, []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, tc.len)
			for i := range data {
				data[i] = byte(i)
			}
			chunks := chunkBytes(data, tc.size)
			if len(chunks) != len(tc.want) {
				t.Fatalf("chunk count = %d, want %d", len(chunks), len(tc.want))
			}
			var reassembled []byte
			for i, c := range chunks {
				if len(c) != tc.want[i] {
					t.Errorf("chunk %d len = %d, want %d", i, len(c), tc.want[i])
				}
				reassembled = append(reassembled, c...)
			}
			if !bytes.Equal(reassembled, data) {
				t.Errorf("reassembled chunks != original")
			}
		})
	}
}

func TestHashBytes_KnownVector(t *testing.T) {
	// SHA-384("abc") — NIST test vector.
	const want = "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed" +
		"8086072ba1e7cc2358baeca134c825a7"
	got := hex.EncodeToString(HashBytes([]byte("abc")))
	if got != want {
		t.Errorf("sha384(abc) = %s, want %s", got, want)
	}
	if len(HashBytes([]byte("abc"))) != 48 {
		t.Errorf("digest length = %d, want 48", len(HashBytes([]byte("abc"))))
	}
}

// recordingSubmit captures every submitted transaction.
type recordingSubmit struct {
	txs     []hiero.TransactionInterface
	failAt  int // 1-based op index to fail at; 0 = never
	failErr error
}

func (r *recordingSubmit) submit(tx hiero.TransactionInterface) error {
	r.txs = append(r.txs, tx)
	if r.failAt != 0 && len(r.txs) == r.failAt {
		return r.failErr
	}
	return nil
}

func TestUploadRawFile_SequencesUpdateThenAppends(t *testing.T) {
	// 3 full chunks + a remainder => 1 FileUpdate + 3 FileAppend.
	data := bytes.Repeat([]byte{0x7}, bytesPerOp*3+100)
	rec := &recordingSubmit{}
	fileID := hiero.FileID{File: 150}

	if err := UploadRawFile(rec.submit, fileID, data); err != nil {
		t.Fatalf("UploadRawFile: %v", err)
	}

	wantOps := 4 // one FileUpdate plus three FileAppend
	if len(rec.txs) != wantOps {
		t.Fatalf("submitted %d ops, want %d", len(rec.txs), wantOps)
	}

	// First op is a FileUpdate; the rest are FileAppend. Reassembling the
	// contents in submission order must reproduce data exactly.
	var reassembled []byte
	for i, tx := range rec.txs {
		switch op := tx.(type) {
		case *hiero.FileUpdateTransaction:
			if i != 0 {
				t.Errorf("FileUpdate at op %d, want only at op 0", i)
			}
			if op.GetFileID() != fileID {
				t.Errorf("update file id = %v, want %v", op.GetFileID(), fileID)
			}
			reassembled = append(reassembled, op.GetContents()...)
		case *hiero.FileAppendTransaction:
			if i == 0 {
				t.Error("first op must be a FileUpdate, got FileAppend")
			}
			if op.GetFileID() != fileID {
				t.Errorf("append file id = %v, want %v", op.GetFileID(), fileID)
			}
			reassembled = append(reassembled, op.GetContents()...)
		default:
			t.Fatalf("op %d has unexpected type %T", i, tx)
		}
	}
	if !bytes.Equal(reassembled, data) {
		t.Error("reassembled upload != original data")
	}
}

func TestUploadRawFile_Empty(t *testing.T) {
	rec := &recordingSubmit{}
	err := UploadRawFile(rec.submit, hiero.FileID{File: 150}, nil)
	wantReason(t, err, reasonEmptyUpload)
	if len(rec.txs) != 0 {
		t.Error("no ops should be submitted for empty data")
	}
}

func TestUploadRawFile_SubmitErrorPropagates(t *testing.T) {
	data := bytes.Repeat([]byte{0x1}, bytesPerOp*2) // update + 1 append
	sentinel := fmt.Errorf("node unreachable")
	rec := &recordingSubmit{failAt: 2, failErr: sentinel} // fail on the append

	err := UploadRawFile(rec.submit, hiero.FileID{File: 150}, data)
	if err == nil {
		t.Fatal("expected error when a chunk submit fails")
	}
	// Context is added but the underlying error is preserved via %w.
	if !bytes.Contains([]byte(err.Error()), []byte("append chunk 2/2")) {
		t.Errorf("error missing chunk context: %v", err)
	}
}

// fitsSingleChunk guards the append configuration: each append is one chunk.
func TestUploadRawFile_AppendIsSingleChunk(t *testing.T) {
	data := bytes.Repeat([]byte{0x2}, bytesPerOp+1) // update + 1 append of 1 byte
	rec := &recordingSubmit{}
	if err := UploadRawFile(rec.submit, hiero.FileID{File: 159}, data); err != nil {
		t.Fatalf("UploadRawFile: %v", err)
	}
	appendTx, ok := rec.txs[1].(*hiero.FileAppendTransaction)
	if !ok {
		t.Fatalf("op 1 type = %T, want FileAppendTransaction", rec.txs[1])
	}
	if appendTx.GetMaxChunks() != 1 {
		t.Errorf("append max chunks = %d, want 1", appendTx.GetMaxChunks())
	}
}
