// Package xof provides an interface for eXtendable-Output Functions.
//
// # Available Functions
//
// SHAKE functions are defined in FIPS-202, see https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.202.pdf.
// BLAKE2Xb and BLAKE2Xs are defined in https://www.blake2.net/blake2x.pdf.
package xof

import (
	"io"

	"github.com/cloudflare/circl/internal/sha3"
	"github.com/cloudflare/circl/xof/k12"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
)

// XOF defines the interface to hash functions that support arbitrary-length output.
type XOF interface {
	// Write absorbs more data into the XOF's state. It panics if called
	// after Read.
	io.Writer

	// Read reads more output from the XOF. It returns io.EOF if the limit
	// has been reached.
	io.Reader

	// Clone returns a copy of the XOF in its current state.
	Clone() XOF

	// Reset restores the XOF to its initial state and discards all data appended by Write.
	Reset()
}

type ID uint

const (
	SHAKE128 ID = iota + 1
	SHAKE256
	BLAKE2XB
	BLAKE2XS
	K12D10
)

func (x ID) New() XOF {
	switch x {
	case SHAKE128:
		s := sha3.NewShake128()
		return shakeBody{&s}
	case SHAKE256:
		s := sha3.NewShake256()
		return shakeBody{&s}
	case BLAKE2XB:
		x, _ := blake2b.NewXOF(blake2b.OutputLengthUnknown, nil)
		return blake2xb{x}
	case BLAKE2XS:
		x, _ := blake2s.NewXOF(blake2s.OutputLengthUnknown, nil)
		return blake2xs{x}
	case K12D10:
		x := k12.NewDraft10([]byte{})
		return k12d10{&x}
	default:
		panic("crypto: requested unavailable XOF function")
	}
}

type shakeBody struct{ sha3.ShakeHash }

func (s shakeBody) Clone() XOF { return shakeBody{s.ShakeHash.Clone()} }

type blake2xb struct{ blake2b.XOF }

func (s blake2xb) Clone() XOF { return blake2xb{s.XOF.Clone()} }

type blake2xs struct{ blake2s.XOF }

func (s blake2xs) Clone() XOF { return blake2xs{s.XOF.Clone()} }

type k12d10 struct{ *k12.State }

func (s k12d10) Clone() XOF {
	x := s.State.Clone()
	return k12d10{&x}
}
