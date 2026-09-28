// Code generated from pkg.templ.go. DO NOT EDIT.

// kyber1024 implements the IND-CPA-secure Public Key Encryption
// scheme Kyber1024.CPAPKE as submitted to round 3 of the NIST PQC competition
// and described in
//
// https://pq-crystals.org/kyber/data/kyber-specification-round3.pdf
package kyber1024

import (
	cryptoRand "crypto/rand"
	"io"

	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/pke/kyber/kyber1024/internal"
)

const (
	// Size of seed for NewKeyFromSeed
	KeySeedSize = internal.SeedSize

	// Size of seed for EncryptTo
	EncryptionSeedSize = internal.SeedSize

	// Size of a packed PublicKey
	PublicKeySize = internal.PublicKeySize

	// Size of a packed PrivateKey
	PrivateKeySize = internal.PrivateKeySize

	// Size of a ciphertext
	CiphertextSize = internal.CiphertextSize

	// Size of a plaintext
	PlaintextSize = internal.PlaintextSize
)

// PublicKey is the type of Kyber1024.CPAPKE public key
type PublicKey internal.PublicKey

// PrivateKey is the type of Kyber1024.CPAPKE private key
type PrivateKey internal.PrivateKey

// GenerateKey generates a public/private key pair using entropy from rand.
// If rand is nil, crypto/rand.Reader will be used.
func GenerateKey(rand io.Reader) (*PublicKey, *PrivateKey, error) {
	var seed [KeySeedSize]byte
	if rand == nil {
		rand = cryptoRand.Reader
	}
	_, err := io.ReadFull(rand, seed[:])
	if err != nil {
		return nil, nil, err
	}
	pk, sk := internal.NewKeyFromSeed(seed[:])
	return (*PublicKey)(pk), (*PrivateKey)(sk), nil
}

// NewKeyFromSeed derives a public/private key pair using the given seed.
//
// Note: does not include the domain separation of ML-KEM (line 1, algorithm 13
// of FIPS 203). For that use NewKeyFromSeedMLKEM().
//
// Panics if seed is not of length KeySeedSize.
func NewKeyFromSeed(seed []byte) (*PublicKey, *PrivateKey) {
	if len(seed) != KeySeedSize {
		panic("seed must be of length KeySeedSize")
	}
	pk, sk := internal.NewKeyFromSeed(seed)
	return (*PublicKey)(pk), (*PrivateKey)(sk)
}

// NewKeyFromSeedMLKEM derives a public/private key pair using the given seed
// using the domain separation of ML-KEM.
//
// Panics if seed is not of length KeySeedSize.
func NewKeyFromSeedMLKEM(seed []byte) (*PublicKey, *PrivateKey) {
	if len(seed) != KeySeedSize {
		panic("seed must be of length KeySeedSize")
	}
	var seed2 [33]byte
	copy(seed2[:32], seed)
	seed2[32] = byte(internal.K)
	pk, sk := internal.NewKeyFromSeed(seed2[:])
	return (*PublicKey)(pk), (*PrivateKey)(sk)
}

// EncryptTo encrypts message pt for the public key and writes the ciphertext
// to ct using randomness from seed.
//
// This function panics if the lengths of pt, seed, and ct are not
// PlaintextSize, EncryptionSeedSize, and CiphertextSize respectively.
func (pk *PublicKey) EncryptTo(ct []byte, pt []byte, seed []byte) {
	if len(pt) != PlaintextSize {
		panic("pt must be of length PlaintextSize")
	}
	if len(ct) != CiphertextSize {
		panic("ct must be of length CiphertextSize")
	}
	if len(seed) != EncryptionSeedSize {
		panic("seed must be of length EncryptionSeedSize")
	}
	(*internal.PublicKey)(pk).EncryptTo(ct, pt, seed)
}

// DecryptTo decrypts message ct for the private key and writes the
// plaintext to pt.
//
// This function panics if the lengths of ct and pt are not
// CiphertextSize and PlaintextSize respectively.
func (sk *PrivateKey) DecryptTo(pt []byte, ct []byte) {
	if len(pt) != PlaintextSize {
		panic("pt must be of length PlaintextSize")
	}
	if len(ct) != CiphertextSize {
		panic("ct must be of length CiphertextSize")
	}
	(*internal.PrivateKey)(sk).DecryptTo(pt, ct)
}

// Packs pk into the given buffer.
//
// Panics if buf is not of length PublicKeySize.
func (pk *PublicKey) Pack(buf []byte) {
	if len(buf) != PublicKeySize {
		panic("buf must be of size PublicKeySize")
	}
	(*internal.PublicKey)(pk).Pack(buf)
}

// Packs sk into the given buffer.
//
// Panics if buf is not of length PrivateKeySize.
func (sk *PrivateKey) Pack(buf []byte) {
	if len(buf) != PrivateKeySize {
		panic("buf must be of size PrivateKeySize")
	}
	(*internal.PrivateKey)(sk).Pack(buf)
}

// Unpacks pk from the given buffer.
//
// Panics if buf is not of length PublicKeySize.
func (pk *PublicKey) Unpack(buf []byte) {
	if len(buf) != PublicKeySize {
		panic("buf must be of size PublicKeySize")
	}
	(*internal.PublicKey)(pk).Unpack(buf)
}

// Unpacks pk from the given buffer.
//
// Returns an error if the buffer is not of the right size, or the public
// key is not normalized.
func (pk *PublicKey) UnpackMLKEM(buf []byte) error {
	if len(buf) != PublicKeySize {
		return kem.ErrPubKeySize
	}
	return (*internal.PublicKey)(pk).UnpackMLKEM(buf)
}

// Unpacks sk from the given buffer.
//
// Panics if buf is not of length PrivateKeySize.
func (sk *PrivateKey) Unpack(buf []byte) {
	if len(buf) != PrivateKeySize {
		panic("buf must be of size PrivateKeySize")
	}
	(*internal.PrivateKey)(sk).Unpack(buf)
}

// Returns whether the two private keys are equal.
func (sk *PrivateKey) Equal(other *PrivateKey) bool {
	return (*internal.PrivateKey)(sk).Equal((*internal.PrivateKey)(other))
}
