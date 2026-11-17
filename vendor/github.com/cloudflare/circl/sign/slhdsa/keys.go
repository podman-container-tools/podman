package slhdsa

import (
	"bytes"
	"crypto"
	"crypto/subtle"

	"github.com/cloudflare/circl/internal/conv"
	"golang.org/x/crypto/cryptobyte"
)

// [PrivateKey] stores a private key of the SLH-DSA scheme.
// It implements the [crypto.Signer] and [crypto.PrivateKey] interfaces.
// For serialization, it also implements [cryptobyte.MarshalingValue],
// [encoding.BinaryMarshaler], and [encoding.BinaryUnmarshaler].
type PrivateKey struct {
	seed, prfKey []byte
	publicKey    PublicKey
	ID
}

func (p *params) PrivateKeySize() int { return int(2*p.n) + p.PublicKeySize() }

// Marshal serializes the key using a [cryptobyte.Builder].
func (k PrivateKey) Marshal(b *cryptobyte.Builder) error {
	b.AddBytes(k.seed)
	b.AddBytes(k.prfKey)
	b.AddValue(k.publicKey)
	return nil
}

// Unmarshal recovers a [PrivateKey] from a [cryptobyte.String].
// Caller must specify the private key's [ID] in advance.
// Example:
//
//	key := PrivateKey{ID: SHA2Small192}
//	key.Unmarshal(str) // returns true
func (k *PrivateKey) Unmarshal(s *cryptobyte.String) bool {
	params := k.ID.params()
	b := make([]byte, params.PrivateKeySize())
	if !s.CopyBytes(b) {
		return false
	}

	c := cursor(b)
	return k.fromBytes(params, &c)
}

func (k *PrivateKey) fromBytes(p *params, c *cursor) bool {
	k.ID = p.ID
	k.seed = c.Next(p.n)
	k.prfKey = c.Next(p.n)
	return k.publicKey.fromBytes(p, c) && k.publicKey.ID == k.ID
}

// UnmarshalBinary recovers a [PrivateKey] from a slice of bytes.
// Caller must specify the private key's [ID] in advance.
// Example:
//
//	key := PrivateKey{ID: SHA2Small192}
//	key.UnmarshalBinary(bytes) // returns nil
func (k *PrivateKey) UnmarshalBinary(b []byte) error { return conv.UnmarshalBinary(k, b) }
func (k PrivateKey) MarshalBinary() ([]byte, error)  { return conv.MarshalBinary(k) }
func (k PrivateKey) Public() crypto.PublicKey        { return k.PublicKey() }
func (k PrivateKey) PublicKey() (pub PublicKey) {
	params := k.ID.params()
	c := cursor(make([]byte, params.PublicKeySize()))
	pub.fromBytes(params, &c)
	copy(pub.seed, k.publicKey.seed)
	copy(pub.root, k.publicKey.root)
	return
}

func (k PrivateKey) Equal(x crypto.PrivateKey) bool {
	other, ok := x.(PrivateKey)
	return ok && k.ID == other.ID &&
		subtle.ConstantTimeCompare(k.seed, other.seed) == 1 &&
		subtle.ConstantTimeCompare(k.prfKey, other.prfKey) == 1 &&
		k.publicKey.Equal(other.publicKey)
}

// [PublicKey] stores a public key of the SLH-DSA scheme.
// It implements the [crypto.PublicKey] interface.
// For serialization, it also implements [cryptobyte.MarshalingValue],
// [encoding.BinaryMarshaler], and [encoding.BinaryUnmarshaler].
type PublicKey struct {
	seed, root []byte
	ID
}

func (p *params) PublicKeySize() int { return int(2 * p.n) }

// Marshal serializes the key using a [cryptobyte.Builder].
func (k PublicKey) Marshal(b *cryptobyte.Builder) error {
	b.AddBytes(k.seed)
	b.AddBytes(k.root)
	return nil
}

// Unmarshal recovers a [PublicKey] from a [cryptobyte.String].
// Caller must specify the public key's [ID] in advance.
// Example:
//
//	key := PublicKey{ID: SHA2Small192}
//	key.Unmarshal(str) // returns true
func (k *PublicKey) Unmarshal(s *cryptobyte.String) bool {
	params := k.ID.params()
	b := make([]byte, params.PublicKeySize())
	if !s.CopyBytes(b) {
		return false
	}

	c := cursor(b)
	return k.fromBytes(params, &c)
}

func (k *PublicKey) fromBytes(p *params, c *cursor) bool {
	k.ID = p.ID
	k.seed = c.Next(p.n)
	k.root = c.Next(p.n)
	return len(*c) == 0
}

// UnmarshalBinary recovers a [PublicKey] from a slice of bytes.
// Caller must specify the public key's [ID] in advance.
// Example:
//
//	key := PublicKey{ID: SHA2Small192}
//	key.UnmarshalBinary(bytes) // returns nil
func (k *PublicKey) UnmarshalBinary(b []byte) error { return conv.UnmarshalBinary(k, b) }
func (k PublicKey) MarshalBinary() ([]byte, error)  { return conv.MarshalBinary(k) }
func (k PublicKey) Equal(x crypto.PublicKey) bool {
	other, ok := x.(PublicKey)
	return ok && k.ID == other.ID &&
		bytes.Equal(k.seed, other.seed) &&
		bytes.Equal(k.root, other.root)
}
