// Package slhdsa provides Stateless Hash-based Digital Signature Algorithm.
//
// This package is compliant with [FIPS 205] and the [ID] represents
// the following parameter sets:
//
// Category 1
//   - Based on SHA2: [SHA2_128s] and [SHA2_128f].
//   - Based on SHAKE: [SHAKE_128s] and [SHAKE_128f].
//
// Category 3
//   - Based on SHA2: [SHA2_192s] and [SHA2_192f]
//   - Based on SHAKE: [SHAKE_192s] and [SHAKE_192f]
//
// Category 5
//   - Based on SHA2: [SHA2_256s] and [SHA2_256f].
//   - Based on SHAKE: [SHAKE_256s] and [SHAKE_256f].
//
// [FIPS 205]: https://doi.org/10.6028/NIST.FIPS.205
package slhdsa

import (
	"crypto"
	"crypto/rand"
	"errors"
	"io"
)

// [GenerateKey] returns a pair of keys using the parameter set specified.
// It returns an error if it fails reading from the random source.
func GenerateKey(
	random io.Reader, id ID,
) (pub PublicKey, priv PrivateKey, err error) {
	// See FIPS 205 -- Section 10.1 -- Algorithm 21.
	params := id.params()

	var skSeed, skPrf, pkSeed []byte
	skSeed, err = readRandom(random, params.n)
	if err != nil {
		return
	}

	skPrf, err = readRandom(random, params.n)
	if err != nil {
		return
	}

	pkSeed, err = readRandom(random, params.n)
	if err != nil {
		return
	}

	pub, priv = slhKeyGenInternal(params, skSeed, skPrf, pkSeed)

	return
}

// [SignDeterministic] returns the signature of the message with the
// specified context.
func SignDeterministic(
	priv *PrivateKey, message *Message, context []byte,
) (signature []byte, err error) {
	return priv.doSign(message, context, priv.publicKey.seed)
}

// [SignRandomized] returns a random signature of the message with the
// specified context.
// It returns an error if it fails reading from the random source.
func SignRandomized(
	priv *PrivateKey, random io.Reader, message *Message, context []byte,
) (signature []byte, err error) {
	params := priv.ID.params()
	addRand, err := readRandom(random, params.n)
	if err != nil {
		return nil, err
	}

	return priv.doSign(message, context, addRand)
}

// [PrivateKey.Sign] returns a randomized signature of the message with an
// empty context.
// Any parameter passed in [crypto.SignerOpts] is discarded.
// It returns an error if it fails reading from the random source.
func (k PrivateKey) Sign(
	random io.Reader, message []byte, _ crypto.SignerOpts,
) (signature []byte, err error) {
	return SignRandomized(&k, random, NewMessage(message), nil)
}

func (k *PrivateKey) doSign(
	message *Message, context, addRand []byte,
) ([]byte, error) {
	// See FIPS 205 -- Section 10.2 -- Algorithm 22 and Algorithm 23.
	msgPrime, err := message.getMsgPrime(context)
	if err != nil {
		return nil, err
	}

	return slhSignInternal(k, msgPrime, addRand)
}

// [Verify] returns true if the signature of the message with the specified
// context is valid.
func Verify(key *PublicKey, message *Message, signature, context []byte) bool {
	// See FIPS 205 -- Section 10.3 -- Algorithm 24.
	msgPrime, err := message.getMsgPrime(context)
	if err != nil {
		return false
	}

	return slhVerifyInternal(key, msgPrime, signature)
}

func readRandom(random io.Reader, size uint32) (out []byte, err error) {
	out = make([]byte, size)
	if random == nil {
		random = rand.Reader
	}
	_, err = random.Read(out)
	return
}

var (
	ErrContext  = errors.New("sign/slhdsa: context is larger than 255 bytes")
	ErrMsgLen   = errors.New("sign/slhdsa: invalid message length")
	ErrParam    = errors.New("sign/slhdsa: invalid SLH-DSA parameter")
	ErrPreHash  = errors.New("sign/slhdsa: invalid prehash function")
	ErrSigParse = errors.New("sign/slhdsa: failed to decode the signature")
	ErrTree     = errors.New("sign/slhdsa: invalid tree height or tree index")
	ErrWriting  = errors.New("sign/slhdsa: failed to write to a hash function")
)
