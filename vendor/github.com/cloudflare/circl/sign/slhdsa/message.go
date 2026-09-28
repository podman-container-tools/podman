package slhdsa

import (
	"crypto"
	"hash"
	"io"

	"github.com/cloudflare/circl/xof"
	_ "golang.org/x/crypto/sha3"
)

// [PreHash] is a helper for hashing a message before signing.
// It implements the [io.Writer] interface, so the message can be provided
// in chunks before calling the [SignDeterministic], [SignRandomized], or
// [Verify] functions.
// Pre-hash must not be used for generating pure signatures.
type PreHash struct {
	writer interface {
		io.Writer
		Reset()
	}
	size int
	oid  byte
}

// [NewPreHashWithHash] is used to prehash messages using either the SHA2 or
// SHA3 hash functions.
// Returns [ErrPreHash] if the function is not supported.
func NewPreHashWithHash(h crypto.Hash) (*PreHash, error) {
	hash2oid := [...]byte{
		crypto.SHA256:     1,
		crypto.SHA384:     2,
		crypto.SHA512:     3,
		crypto.SHA224:     4,
		crypto.SHA512_224: 5,
		crypto.SHA512_256: 6,
		crypto.SHA3_224:   7,
		crypto.SHA3_256:   8,
		crypto.SHA3_384:   9,
		crypto.SHA3_512:   10,
	}

	oid := hash2oid[h]
	if oid == 0 {
		return nil, ErrPreHash
	}

	return &PreHash{h.New(), h.Size(), oid}, nil
}

// [NewPreHashWithXof] is used to prehash messages using either SHAKE-128
// or SHAKE-256.
// Returns [ErrPreHash] if the function is not supported.
func NewPreHashWithXof(x xof.ID) (*PreHash, error) {
	switch x {
	case xof.SHAKE128:
		return &PreHash{x.New(), 32, 11}, nil
	case xof.SHAKE256:
		return &PreHash{x.New(), 64, 12}, nil
	default:
		return nil, ErrPreHash
	}
}

func (ph *PreHash) Reset()                      { ph.writer.Reset() }
func (ph *PreHash) Write(b []byte) (int, error) { return ph.writer.Write(b) }

// BuildMessage returns a [Message] for signing, and resets the writer.
func (ph *PreHash) BuildMessage() (*Message, error) {
	// Source https://csrc.nist.gov/Projects/computer-security-objects-register/algorithm-registration
	const oidLen = 11
	oid := [oidLen]byte{
		0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, ph.oid,
	}

	msg := make([]byte, oidLen+ph.size)
	copy(msg, oid[:])
	switch f := ph.writer.(type) {
	case hash.Hash:
		msg = f.Sum(msg[:oidLen])
	case xof.XOF:
		_, err := f.Read(msg[oidLen:])
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrPreHash
	}

	ph.Reset()
	return &Message{msg, 1}, nil
}

// [Message] wraps a message for signing.
type Message struct {
	msg       []byte
	isPreHash byte
}

// For pure signatures, use [NewMessage] to pass the message to be signed.
// For pre-hashed signatures, use [PreHash] to hash the message first, and
// then use [PreHash.BuildMessage] to get a [Message] to be signed.
func NewMessage(msg []byte) *Message { return &Message{msg, 0} }

func (m *Message) getMsgPrime(context []byte) ([]byte, error) {
	// See FIPS 205 -- Section 10.2 -- Algorithm 23 and Algorithm 25.
	const MaxContextSize = 255
	if len(context) > MaxContextSize {
		return nil, ErrContext
	}

	return append(append(
		[]byte{m.isPreHash, byte(len(context))}, context...), m.msg...,
	), nil
}
