// Package kem provides a unified interface for KEM schemes.
//
// A register of schemes is available in the package
//
//	github.com/cloudflare/circl/kem/schemes
package kem

import (
	"encoding"
	"errors"
)

// A KEM public key
type PublicKey interface {
	// Returns the scheme for this public key
	Scheme() Scheme

	encoding.BinaryMarshaler
	Equal(PublicKey) bool
}

// A KEM private key
type PrivateKey interface {
	// Returns the scheme for this private key
	Scheme() Scheme

	encoding.BinaryMarshaler
	Equal(PrivateKey) bool
	Public() PublicKey
}

// A Scheme represents a specific instance of a KEM.
type Scheme interface {
	// Name of the scheme
	Name() string

	// GenerateKeyPair creates a new key pair.
	GenerateKeyPair() (PublicKey, PrivateKey, error)

	// Encapsulate generates a shared key ss for the public key and
	// encapsulates it into a ciphertext ct.
	Encapsulate(pk PublicKey) (ct, ss []byte, err error)

	// Returns the shared key encapsulated in ciphertext ct for the
	// private key sk.
	Decapsulate(sk PrivateKey, ct []byte) ([]byte, error)

	// Unmarshals a PublicKey from the provided buffer.
	UnmarshalBinaryPublicKey([]byte) (PublicKey, error)

	// Unmarshals a PrivateKey from the provided buffer.
	UnmarshalBinaryPrivateKey([]byte) (PrivateKey, error)

	// Size of encapsulated keys.
	CiphertextSize() int

	// Size of established shared keys.
	SharedKeySize() int

	// Size of packed private keys.
	PrivateKeySize() int

	// Size of packed public keys.
	PublicKeySize() int

	// DeriveKeyPair deterministically derives a pair of keys from a seed.
	// Panics if the length of seed is not equal to the value returned by
	// SeedSize.
	DeriveKeyPair(seed []byte) (PublicKey, PrivateKey)

	// Size of seed used in DeriveKey
	SeedSize() int

	// EncapsulateDeterministically generates a shared key ss for the public
	// key deterministically from the given seed and encapsulates it into
	// a ciphertext ct. If unsure, you're better off using Encapsulate().
	EncapsulateDeterministically(pk PublicKey, seed []byte) (
		ct, ss []byte, err error)

	// Size of seed used in EncapsulateDeterministically().
	EncapsulationSeedSize() int
}

// AuthScheme represents a KEM that supports authenticated key encapsulation.
type AuthScheme interface {
	Scheme
	AuthEncapsulate(pkr PublicKey, sks PrivateKey) (ct, ss []byte, err error)
	AuthEncapsulateDeterministically(pkr PublicKey, sks PrivateKey, seed []byte) (ct, ss []byte, err error)
	AuthDecapsulate(skr PrivateKey, ct []byte, pks PublicKey) ([]byte, error)
}

var (
	// ErrTypeMismatch is the error used if types of, for instance, private
	// and public keys don't match
	ErrTypeMismatch = errors.New("types mismatch")

	// ErrSeedSize is the error used if the provided seed is of the wrong
	// size.
	ErrSeedSize = errors.New("wrong seed size")

	// ErrPubKeySize is the error used if the provided public key is of
	// the wrong size.
	ErrPubKeySize = errors.New("wrong size for public key")

	// ErrCiphertextSize is the error used if the provided ciphertext
	// is of the wrong size.
	ErrCiphertextSize = errors.New("wrong size for ciphertext")

	// ErrPrivKeySize is the error used if the provided private key is of
	// the wrong size.
	ErrPrivKeySize = errors.New("wrong size for private key")

	// ErrPubKey is the error used if the provided public key is invalid.
	ErrPubKey = errors.New("invalid public key")

	// ErrPrivKey is the error used if the provided private key is invalid.
	ErrPrivKey = errors.New("invalid private key")

	// ErrCipherText is the error used if the provided ciphertext is invalid.
	ErrCipherText = errors.New("invalid ciphertext")
)
