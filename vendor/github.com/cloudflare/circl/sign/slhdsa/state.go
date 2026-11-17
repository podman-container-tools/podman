package slhdsa

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"io"

	"github.com/cloudflare/circl/internal/sha3"
)

// statePriv encapsulates common data for performing a private operation.
type statePriv struct {
	state
	PRF statePRF
}

func (s *statePriv) Size(p *params) uint32 {
	return s.state.Size(p) + s.PRF.Size(p)
}

func (p *params) NewStatePriv(skSeed, pkSeed []byte) (s statePriv) {
	c := cursor(make([]byte, s.Size(p)))
	s.state.init(p, &c, pkSeed)
	s.PRF.Init(p, &c, skSeed, pkSeed)

	return
}

func (s *statePriv) Clear() {
	s.PRF.Clear()
	s.state.Clear()
}

// state encapsulates common data for performing a public operation.
type state struct {
	*params

	F stateF
	H stateH
	T stateT
}

func (s *state) Size(p *params) uint32 {
	return s.F.Size(p) + s.H.Size(p) + s.T.Size(p)
}

func (p *params) NewStatePub(pkSeed []byte) (s state) {
	c := cursor(make([]byte, s.Size(p)))
	s.init(p, &c, pkSeed)

	return
}

func (s *state) init(p *params, c *cursor, pkSeed []byte) {
	s.params = p
	s.F.Init(p, c, pkSeed)
	s.H.Init(p, c, pkSeed)
	s.T.Init(p, c, pkSeed)
}

func (s *state) Clear() {
	s.F.Clear()
	s.H.Clear()
	s.T.Clear()
	s.params = nil
}

func sha256sum(out, in []byte) { s := sha256.Sum256(in); copy(out, s[:]) }
func sha512sum(out, in []byte) { s := sha512.Sum512(in); copy(out, s[:]) }

type baseHasher struct {
	hash          func(out, in []byte)
	input, output []byte
	address
}

func (b *baseHasher) Size(p *params) uint32 {
	return p.n + p.addressSize()
}

func (b *baseHasher) Clear() {
	clearSlice(&b.input)
	clearSlice(&b.output)
	b.address.Clear()
}

func (b *baseHasher) Final() []byte {
	b.hash(b.output, b.input)
	return b.output
}

type statePRF struct{ baseHasher }

func (s *statePRF) Init(p *params, cur *cursor, skSeed, pkSeed []byte) {
	c := cursor(cur.Next(s.Size(p)))
	s.output = c.Next(p.n)
	s.input = c.Rest()
	copy(c.Next(p.n), pkSeed)
	_ = c.Next(s.padSize(p))
	s.address.fromBytes(p, &c)
	copy(c.Next(p.n), skSeed)

	if p.isSHA2 {
		s.hash = sha256sum
	} else {
		s.hash = sha3.ShakeSum256
	}
}

func (s *statePRF) Size(p *params) uint32 {
	return 2*p.n + s.padSize(p) + s.baseHasher.Size(p)
}

func (s *statePRF) padSize(p *params) uint32 {
	if p.isSHA2 {
		return 64 - p.n
	} else {
		return 0
	}
}

type stateF struct {
	msg []byte
	baseHasher
}

func (s *stateF) Init(p *params, cur *cursor, pkSeed []byte) {
	c := cursor(cur.Next(s.Size(p)))
	s.output = c.Next(p.n)
	s.input = c.Rest()
	copy(c.Next(p.n), pkSeed)
	_ = c.Next(s.padSize(p))
	s.address.fromBytes(p, &c)
	s.msg = c.Next(p.n)

	if p.isSHA2 {
		s.hash = sha256sum
	} else {
		s.hash = sha3.ShakeSum256
	}
}

func (s *stateF) SetMessage(msg []byte) { copy(s.msg, msg) }

func (s *stateF) Clear() {
	s.baseHasher.Clear()
	clearSlice(&s.msg)
}

func (s *stateF) Size(p *params) uint32 {
	return 2*p.n + s.padSize(p) + s.baseHasher.Size(p)
}

func (s *stateF) padSize(p *params) uint32 {
	if p.isSHA2 {
		return 64 - p.n
	} else {
		return 0
	}
}

type stateH struct {
	msg0, msg1 []byte
	baseHasher
}

func (s *stateH) Init(p *params, cur *cursor, pkSeed []byte) {
	c := cursor(cur.Next(s.Size(p)))
	s.output = c.Next(p.n)
	s.input = c.Rest()
	copy(c.Next(p.n), pkSeed)
	_ = c.Next(s.padSize(p))
	s.address.fromBytes(p, &c)
	s.msg0 = c.Next(p.n)
	s.msg1 = c.Next(p.n)

	if p.isSHA2 {
		if p.n == 16 {
			s.hash = sha256sum
		} else {
			s.hash = sha512sum
		}
	} else {
		s.hash = sha3.ShakeSum256
	}
}

func (s *stateH) SetMsgs(m0, m1 []byte) {
	copy(s.msg0, m0)
	copy(s.msg1, m1)
}

func (s *stateH) Clear() {
	s.baseHasher.Clear()
	clearSlice(&s.msg0)
	clearSlice(&s.msg1)
}

func (s *stateH) Size(p *params) uint32 {
	return 3*p.n + s.padSize(p) + s.baseHasher.Size(p)
}

func (s *stateH) padSize(p *params) uint32 {
	if p.isSHA2 {
		if p.n == 16 {
			return 64 - p.n
		} else {
			return 128 - p.n
		}
	} else {
		return 0
	}
}

type stateT struct {
	hash interface {
		io.Writer
		Reset()
		Final([]byte)
	}
	input, output []byte
	address
}

func (s *stateT) Init(p *params, cur *cursor, pkSeed []byte) {
	c := cursor(cur.Next(s.Size(p)))
	s.output = c.Next(s.outputSize(p))[:p.n]
	s.input = c.Rest()
	copy(c.Next(p.n), pkSeed)
	_ = c.Next(s.padSize(p))
	s.address.fromBytes(p, &c)

	if p.isSHA2 {
		if p.n == 16 {
			s.hash = &sha2rw{sha256.New()}
		} else {
			s.hash = &sha2rw{sha512.New()}
		}
	} else {
		s.hash = &sha3rw{sha3.NewShake256()}
	}
}

func (s *stateT) Clear() {
	clearSlice(&s.input)
	clearSlice(&s.output)
	s.address.Clear()
	s.hash.Reset()
}

func (s *stateT) Reset() {
	s.hash.Reset()
	_, _ = s.hash.Write(s.input)
}

func (s *stateT) WriteMessage(msg []byte) { _, _ = s.hash.Write(msg) }

func (s *stateT) Final() []byte {
	s.hash.Final(s.output)
	return s.output
}

func (s *stateT) Size(p *params) uint32 {
	return s.outputSize(p) + s.padSize(p) + p.n + p.addressSize()
}

func (s *stateT) outputSize(p *params) uint32 {
	if p.isSHA2 {
		if p.n == 16 {
			return sha256.Size
		} else {
			return sha512.Size
		}
	} else {
		return p.n
	}
}

func (s *stateT) padSize(p *params) uint32 {
	if p.isSHA2 {
		if p.n == 16 {
			return 64 - p.n
		} else {
			return 128 - p.n
		}
	} else {
		return 0
	}
}

type sha2rw struct{ hash.Hash }

func (s *sha2rw) Final(out []byte)         { s.Sum(out[:0]) }
func (s *sha2rw) SumIdempotent(out []byte) { s.Sum(out[:0]) }

type sha3rw struct{ sha3.State }

func (s *sha3rw) Final(out []byte)         { _, _ = s.Read(out) }
func (s *sha3rw) SumIdempotent(out []byte) { _, _ = s.Clone().Read(out) }

type (
	item struct {
		node []byte
		z    uint32
	}
	stackNode []item
)

func (p *params) NewStack(z uint32) stackNode {
	s := make([]item, z)
	c := cursor(make([]byte, z*p.n))
	for i := range s {
		s[i].node = c.Next(p.n)
	}

	return s[:0]
}

func (s stackNode) isEmpty() bool { return len(s) == 0 }
func (s stackNode) top() item     { return s[len(s)-1] }
func (s *stackNode) push(v item) {
	next := len(*s)
	*s = (*s)[:next+1]
	(*s)[next].z = v.z
	copy((*s)[next].node, v.node)
}

func (s *stackNode) pop() (v item) {
	last := len(*s) - 1
	if last >= 0 {
		v = (*s)[last]
		*s = (*s)[:last]
	}
	return
}

func (s *stackNode) Clear() {
	*s = (*s)[:cap(*s)]
	for i := range *s {
		clearSlice(&(*s)[i].node)
	}
	clear((*s)[:])
}

type cursor []byte

func (c *cursor) Rest() []byte { return (*c)[:] }
func (c *cursor) Next(n uint32) (out []byte) {
	if len(*c) >= int(n) {
		out = (*c)[:n]
		*c = (*c)[n:]
	}
	return
}

func clearSlice(s *[]byte) { clear(*s); *s = nil }
