// SPDX-License-Identifier: Apache-2.0

package keys

import (
	"bytes"
	"testing"
)

func TestIsValidEd25519PublicKey(t *testing.T) {
	if !IsValidEd25519PublicKey(bytes.Repeat([]byte{0x01}, 32)) {
		t.Error("32 bytes should be a valid Ed25519 public key")
	}
	for _, n := range []int{0, 31, 33, 64} {
		if IsValidEd25519PublicKey(bytes.Repeat([]byte{0x01}, n)) {
			t.Errorf("%d bytes should not be a valid Ed25519 public key", n)
		}
	}
}

func TestIsValidECDSASecp256k1PublicKey(t *testing.T) {
	for _, prefix := range []byte{0x02, 0x03} {
		key := append([]byte{prefix}, bytes.Repeat([]byte{0xcd}, 32)...)
		if !IsValidECDSASecp256k1PublicKey(key) {
			t.Errorf("33 bytes with prefix 0x%02x should be valid", prefix)
		}
	}
	// Wrong prefix (uncompressed/invalid), and wrong lengths.
	if IsValidECDSASecp256k1PublicKey(append([]byte{0x04}, bytes.Repeat([]byte{0xcd}, 32)...)) {
		t.Error("33 bytes with prefix 0x04 should not be a valid compressed key")
	}
	for _, n := range []int{0, 32, 34} {
		if IsValidECDSASecp256k1PublicKey(bytes.Repeat([]byte{0x02}, n)) {
			t.Errorf("%d bytes should not be a valid compressed secp256k1 key", n)
		}
	}
}
