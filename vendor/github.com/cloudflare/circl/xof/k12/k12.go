// k12 implements the KangarooTwelve XOF.
//
// KangarooTwelve is being standardised at the CFRG working group
// of the IRTF. This package implements draft 10.
//
// https://datatracker.ietf.org/doc/draft-irtf-cfrg-kangarootwelve/10/
package k12

import (
	"encoding/binary"

	"github.com/cloudflare/circl/internal/sha3"
	"github.com/cloudflare/circl/simd/keccakf1600"
)

const chunkSize = 8192 // aka B

// KangarooTwelve splits the message into chunks of 8192 bytes each.
// The first chunk is absorbed directly in a TurboSHAKE128 instance, which
// we call the stalk. The subsequent chunks aren't absorbed directly, but
// instead their hash is absorbed: they're like leafs on a stalk.
// If we have a fast TurboSHAKE128 available, we buffer chunks until we have
// enough to do the parallel TurboSHAKE128. If not, we absorb directly into
// a separate TurboSHAKE128 state.

type State struct {
	initialTodo int // Bytes left to absorb for the first chunk.

	stalk sha3.State

	context []byte // context string "C" provided by the user

	// buffer of incoming data so we can do parallel TurboSHAKE128:
	// nil when we haven't absorbed the first chunk yet;
	// empty if we have, but we do not have a fast parallel TurboSHAKE128;
	// and chunkSize*lanes in length if we have.
	buf []byte

	offset int // offset in buf or bytes written to leaf

	// Number of chunk hashes ("CV_i") absorbed into the stalk.
	chunk uint

	// TurboSHAKE128 instance to compute the leaf in case we don't have
	// a fast parallel TurboSHAKE128, viz when lanes == 1.
	leaf *sha3.State

	lanes uint8 // number of TurboSHAKE128s to compute in parallel
}

// NewDraft10 creates a new instance of Kangaroo12 draft version -10.
func NewDraft10(c []byte) State {
	var lanes byte = 1

	if keccakf1600.IsEnabledX4() {
		lanes = 4
	} else if keccakf1600.IsEnabledX2() {
		lanes = 2
	}

	return newDraft10(c, lanes)
}

func newDraft10(c []byte, lanes byte) State {
	return State{
		initialTodo: chunkSize,
		stalk:       sha3.NewTurboShake128(0x07),
		context:     c,
		lanes:       lanes,
	}
}

func (s *State) Reset() {
	s.initialTodo = chunkSize
	s.stalk.Reset()
	s.stalk.SwitchDS(0x07)
	s.buf = nil
	s.offset = 0
	s.chunk = 0
}

func (s *State) Clone() State {
	stalk := s.stalk.Clone().(*sha3.State)
	ret := State{
		initialTodo: s.initialTodo,
		stalk:       *stalk,
		context:     s.context,
		offset:      s.offset,
		chunk:       s.chunk,
		lanes:       s.lanes,
	}

	if s.leaf != nil {
		ret.leaf = s.leaf.Clone().(*sha3.State)
	}

	if s.buf != nil {
		ret.buf = make([]byte, len(s.buf))
		copy(ret.buf, s.buf)
	}

	return ret
}

func Draft10Sum(hash []byte, msg []byte, c []byte) {
	// TODO Tweak number of lanes depending on the length of the message
	s := NewDraft10(c)
	_, _ = s.Write(msg)
	_, _ = s.Read(hash)
}

func (s *State) Write(p []byte) (int, error) {
	written := len(p)

	// The first chunk is written directly to the stalk.
	if s.initialTodo > 0 {
		taken := s.initialTodo
		if len(p) < taken {
			taken = len(p)
		}
		headP := p[:taken]
		_, _ = s.stalk.Write(headP)
		s.initialTodo -= taken
		p = p[taken:]
	}

	if len(p) == 0 {
		return written, nil
	}

	// If this is the first bit of data written after the initial chunk,
	// we're out of the fast-path and allocate some buffers.
	if s.buf == nil {
		if s.lanes != 1 {
			s.buf = make([]byte, int(s.lanes)*chunkSize)
		} else {
			// We create the buffer to signal we're past the first chunk,
			// but do not use it.
			s.buf = make([]byte, 0)
			h := sha3.NewTurboShake128(0x0B)
			s.leaf = &h
		}
		_, _ = s.stalk.Write([]byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		s.stalk.SwitchDS(0x06)
	}

	// If we're just using one lane, we don't need to cache in a buffer
	// for parallel hashing. Instead, we feed directly to TurboSHAKE.
	if s.lanes == 1 {
		for len(p) > 0 {
			// Write to current leaf.
			to := chunkSize - s.offset
			if len(p) < to {
				to = len(p)
			}
			_, _ = s.leaf.Write(p[:to])
			p = p[to:]
			s.offset += to

			// Did we fill the chunk?
			if s.offset == chunkSize {
				var cv [32]byte
				_, _ = s.leaf.Read(cv[:])
				_, _ = s.stalk.Write(cv[:])
				s.leaf.Reset()
				s.offset = 0
				s.chunk++
			}
		}

		return written, nil
	}

	// If we can't fill all our lanes or the buffer isn't empty, we write the
	// data to the buffer.
	if s.offset != 0 || len(p) < len(s.buf) {
		to := len(s.buf) - s.offset
		if len(p) < to {
			to = len(p)
		}
		p2 := p[:to]
		p = p[to:]
		copy(s.buf[s.offset:], p2)
		s.offset += to
	}

	// Absorb the buffer if we filled it
	if s.offset == len(s.buf) {
		s.writeX(s.buf)
		s.offset = 0
	}

	// Note that at this point we may assume that s.offset = 0 if len(p) != 0
	if len(p) != 0 && s.offset != 0 {
		panic("shouldn't happen")
	}

	// Absorb a bunch of chunks at the same time.
	if len(p) >= int(s.lanes)*chunkSize {
		p = s.writeX(p)
	}

	// Put the remainder in the buffer.
	if len(p) > 0 {
		copy(s.buf, p)
		s.offset = len(p)
	}

	return written, nil
}

// Absorb a multiple of a multiple of lanes * chunkSize.
// Returns the remainder.
func (s *State) writeX(p []byte) []byte {
	switch s.lanes {
	case 4:
		return s.writeX4(p)
	default:
		return s.writeX2(p)
	}
}

func (s *State) writeX4(p []byte) []byte {
	for len(p) >= 4*chunkSize {
		var x4 keccakf1600.StateX4
		a := x4.Initialize(true)

		for offset := 0; offset < 48*168; offset += 168 {
			for i := 0; i < 21; i++ {
				a[i*4] ^= binary.LittleEndian.Uint64(
					p[8*i+offset:],
				)
				a[i*4+1] ^= binary.LittleEndian.Uint64(
					p[chunkSize+8*i+offset:],
				)
				a[i*4+2] ^= binary.LittleEndian.Uint64(
					p[chunkSize*2+8*i+offset:],
				)
				a[i*4+3] ^= binary.LittleEndian.Uint64(
					p[chunkSize*3+8*i+offset:],
				)
			}

			x4.Permute()
		}

		for i := 0; i < 16; i++ {
			a[i*4] ^= binary.LittleEndian.Uint64(
				p[8*i+48*168:],
			)
			a[i*4+1] ^= binary.LittleEndian.Uint64(
				p[chunkSize+8*i+48*168:],
			)
			a[i*4+2] ^= binary.LittleEndian.Uint64(
				p[chunkSize*2+8*i+48*168:],
			)
			a[i*4+3] ^= binary.LittleEndian.Uint64(
				p[chunkSize*3+8*i+48*168:],
			)
		}

		a[16*4] ^= 0x0b
		a[16*4+1] ^= 0x0b
		a[16*4+2] ^= 0x0b
		a[16*4+3] ^= 0x0b
		a[20*4] ^= 0x80 << 56
		a[20*4+1] ^= 0x80 << 56
		a[20*4+2] ^= 0x80 << 56
		a[20*4+3] ^= 0x80 << 56

		x4.Permute()

		var buf [32 * 4]byte
		for i := 0; i < 4; i++ {
			binary.LittleEndian.PutUint64(buf[8*i:], a[4*i])
			binary.LittleEndian.PutUint64(buf[32+8*i:], a[4*i+1])
			binary.LittleEndian.PutUint64(buf[32*2+8*i:], a[4*i+2])
			binary.LittleEndian.PutUint64(buf[32*3+8*i:], a[4*i+3])
		}

		_, _ = s.stalk.Write(buf[:])
		p = p[chunkSize*4:]
		s.chunk += 4
	}

	return p
}

func (s *State) writeX2(p []byte) []byte {
	// TODO On M2 Pro, 1/3 of the time is spent on this function
	// and LittleEndian.Uint64 excluding the actual permutation.
	// Rewriting in assembler might be worthwhile.
	for len(p) >= 2*chunkSize {
		var x2 keccakf1600.StateX2
		a := x2.Initialize(true)

		for offset := 0; offset < 48*168; offset += 168 {
			for i := 0; i < 21; i++ {
				a[i*2] ^= binary.LittleEndian.Uint64(
					p[8*i+offset:],
				)
				a[i*2+1] ^= binary.LittleEndian.Uint64(
					p[chunkSize+8*i+offset:],
				)
			}

			x2.Permute()
		}

		for i := 0; i < 16; i++ {
			a[i*2] ^= binary.LittleEndian.Uint64(
				p[8*i+48*168:],
			)
			a[i*2+1] ^= binary.LittleEndian.Uint64(
				p[chunkSize+8*i+48*168:],
			)
		}

		a[16*2] ^= 0x0b
		a[16*2+1] ^= 0x0b
		a[20*2] ^= 0x80 << 56
		a[20*2+1] ^= 0x80 << 56

		x2.Permute()

		var buf [32 * 2]byte
		for i := 0; i < 4; i++ {
			binary.LittleEndian.PutUint64(buf[8*i:], a[2*i])
			binary.LittleEndian.PutUint64(buf[32+8*i:], a[2*i+1])
		}

		_, _ = s.stalk.Write(buf[:])
		p = p[chunkSize*2:]
		s.chunk += 2
	}

	return p
}

func (s *State) Read(p []byte) (int, error) {
	if s.stalk.IsAbsorbing() {
		// Write context string C
		_, _ = s.Write(s.context)

		// Write length_encode( |C| )
		var buf [9]byte
		binary.BigEndian.PutUint64(buf[:8], uint64(len(s.context)))

		// Find first non-zero digit in big endian encoding of context length
		i := 0
		for buf[i] == 0 && i < 8 {
			i++
		}

		buf[8] = byte(8 - i) // number of bytes to represent |C|
		_, _ = s.Write(buf[i:])

		// We need to write the chunk number if we're past the first chunk.
		if s.buf != nil {
			// Write last remaining chunk(s)
			var cv [32]byte
			if s.lanes == 1 {
				if s.offset != 0 {
					_, _ = s.leaf.Read(cv[:])
					_, _ = s.stalk.Write(cv[:])
					s.chunk++
				}
			} else {
				remainingBuf := s.buf[:s.offset]
				for len(remainingBuf) > 0 {
					h := sha3.NewTurboShake128(0x0B)
					to := chunkSize
					if len(remainingBuf) < to {
						to = len(remainingBuf)
					}
					_, _ = h.Write(remainingBuf[:to])
					_, _ = h.Read(cv[:])
					_, _ = s.stalk.Write(cv[:])
					s.chunk++
					remainingBuf = remainingBuf[to:]
				}
			}

			// Write length_encode( chunk )
			binary.BigEndian.PutUint64(buf[:8], uint64(s.chunk))

			// Find first non-zero digit in big endian encoding of number of chunks
			i = 0
			for buf[i] == 0 && i < 8 {
				i++
			}

			buf[8] = byte(8 - i) // number of bytes to represent number of chunks.
			_, _ = s.stalk.Write(buf[i:])
			_, _ = s.stalk.Write([]byte{0xff, 0xff})
		}
	}

	return s.stalk.Read(p)
}
