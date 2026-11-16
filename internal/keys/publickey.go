// SPDX-License-Identifier: Apache-2.0

// Package keys holds reusable predicates for classifying public-key bytes by
// signature scheme, mirroring the consensus node's own checks (Ed25519Utils /
// Secp256k1Utils in hapi-utils) so the operator agrees with it on what a valid
// node key looks like.
package keys

const (
	// Ed25519PublicKeyLength is the raw length of an Ed25519 public key.
	Ed25519PublicKeyLength = 32
	// ECDSASecp256k1CompressedLength is the length of a compressed secp256k1
	// public key (a 1-byte parity prefix plus the 32-byte X coordinate).
	ECDSASecp256k1CompressedLength = 33

	// compressedEvenParity / compressedOddParity are the SEC1 prefix bytes that
	// begin a compressed secp256k1 point.
	compressedEvenParity = 0x02
	compressedOddParity  = 0x03
)

// IsValidEd25519PublicKey reports whether raw is a valid Ed25519 public key.
func IsValidEd25519PublicKey(raw []byte) bool {
	return len(raw) == Ed25519PublicKeyLength
}

// IsValidECDSASecp256k1PublicKey reports whether raw is a valid compressed
// ECDSA(secp256k1) public key.
func IsValidECDSASecp256k1PublicKey(raw []byte) bool {
	return len(raw) == ECDSASecp256k1CompressedLength &&
		(raw[0] == compressedEvenParity || raw[0] == compressedOddParity)
}
