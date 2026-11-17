package common

import (
	"encoding/binary"

	"github.com/cloudflare/circl/internal/sha3"
	"github.com/cloudflare/circl/simd/keccakf1600"
)

// DeriveX4Available indicates whether the system supports the quick fourway
// sampling variants like PolyDeriveUniformX4.
var DeriveX4Available = keccakf1600.IsEnabledX4()

// Samples p from a centered binomial distribution with given η.
//
// Essentially CBD_η(PRF(seed, nonce)) from the specification.
func (p *Poly) DeriveNoise(seed []byte, nonce uint8, eta int) {
	switch eta {
	case 2:
		p.DeriveNoise2(seed, nonce)
	case 3:
		p.DeriveNoise3(seed, nonce)
	default:
		panic("unsupported eta")
	}
}

// Sample p from a centered binomial distribution with n=6 and p=½ - that is:
// coefficients are in {-3, -2, -1, 0, 1, 2, 3} with probabilities {1/64, 3/32,
// 15/64, 5/16, 16/64, 3/32, 1/64}.
func (p *Poly) DeriveNoise3(seed []byte, nonce uint8) {
	keySuffix := [1]byte{nonce}
	h := sha3.NewShake256()
	_, _ = h.Write(seed[:])
	_, _ = h.Write(keySuffix[:])

	// The distribution at hand is exactly the same as that
	// of (a₁ + a₂ + a₃) - (b₁ + b₂+b₃) where a_i,b_i~U(1).  Thus we need
	// 6 bits per coefficients, thus 192 bytes of input entropy.

	// We add two extra zero bytes in the buffer to be able to read 8 bytes
	// at the same time (while using only 6.)
	var buf [192 + 2]byte
	_, _ = h.Read(buf[:192])

	for i := 0; i < 32; i++ {
		// t is interpreted as a₁ + 2a₂ + 4a₃ + 8b₁ + 16b₂ + ….
		t := binary.LittleEndian.Uint64(buf[6*i:])

		d := t & 0x249249249249        // a₁ + 8b₁ + …
		d += (t >> 1) & 0x249249249249 // a₁ + a₂ + 8(b₁ + b₂) + …
		d += (t >> 2) & 0x249249249249 // a₁ + a₂ + a₃ + 4(b₁ + b₂ + b₃) + …

		for j := 0; j < 8; j++ {
			a := int16(d) & 0x7 // a₁ + a₂ + a₃
			d >>= 3
			b := int16(d) & 0x7 // b₁ + b₂ + b₃
			d >>= 3
			p[8*i+j] = a - b
		}
	}
}

// Sample p from a centered binomial distribution with n=4 and p=½ - that is:
// coefficients are in {-2, -1, 0, 1, 2} with probabilities {1/16, 1/4,
// 3/8, 1/4, 1/16}.
func (p *Poly) DeriveNoise2(seed []byte, nonce uint8) {
	keySuffix := [1]byte{nonce}
	h := sha3.NewShake256()
	_, _ = h.Write(seed[:])
	_, _ = h.Write(keySuffix[:])

	// The distribution at hand is exactly the same as that
	// of (a + a') - (b + b') where a,a',b,b'~U(1).  Thus we need 4 bits per
	// coefficients, thus 128 bytes of input entropy.

	var buf [128]byte
	_, _ = h.Read(buf[:])

	for i := 0; i < 16; i++ {
		// t is interpreted as a + 2a' + 4b + 8b' + ….
		t := binary.LittleEndian.Uint64(buf[8*i:])

		d := t & 0x5555555555555555        // a + 4b + …
		d += (t >> 1) & 0x5555555555555555 // a+a' + 4(b + b') + …

		for j := 0; j < 16; j++ {
			a := int16(d) & 0x3
			d >>= 2
			b := int16(d) & 0x3
			d >>= 2
			p[16*i+j] = a - b
		}
	}
}

// For each i, sample ps[i] uniformly from the given seed for coordinates
// xs[i] and ys[i]. ps[i] may be nil and is ignored in that case.
//
// Can only be called when DeriveX4Available is true.
func PolyDeriveUniformX4(ps [4]*Poly, seed *[32]byte, xs, ys [4]uint8) {
	var perm keccakf1600.StateX4
	state := perm.Initialize(false)

	// Absorb the seed in the four states
	for i := 0; i < 4; i++ {
		v := binary.LittleEndian.Uint64(seed[8*i : 8*(i+1)])
		for j := 0; j < 4; j++ {
			state[i*4+j] = v
		}
	}

	// Absorb the coordinates, the SHAKE128 domain separator (0b1111), the
	// start of the padding (0b…001) and the end of the padding 0b100….
	// Recall that the rate of SHAKE128 is 168; ie. 21 uint64s.
	for j := 0; j < 4; j++ {
		state[4*4+j] = uint64(xs[j]) | (uint64(ys[j]) << 8) | (0x1f << 16)
		state[20*4+j] = 0x80 << 56
	}

	var idx [4]int // indices into ps
	for j := 0; j < 4; j++ {
		if ps[j] == nil {
			idx[j] = N // mark nil polynomials as completed
		}
	}

	done := false
	for !done {
		// Applies KeccaK-f[1600] to state to get the next 21 uint64s of each of
		// the four SHAKE128 streams.
		perm.Permute()

		done = true

	PolyLoop:
		for j := 0; j < 4; j++ {
			if idx[j] == N {
				continue
			}
			for i := 0; i < 7; i++ {
				var t [16]uint16

				v1 := state[i*3*4+j]
				v2 := state[(i*3+1)*4+j]
				v3 := state[(i*3+2)*4+j]

				t[0] = uint16(v1) & 0xfff
				t[1] = uint16(v1>>12) & 0xfff
				t[2] = uint16(v1>>24) & 0xfff
				t[3] = uint16(v1>>36) & 0xfff
				t[4] = uint16(v1>>48) & 0xfff
				t[5] = uint16((v1>>60)|(v2<<4)) & 0xfff

				t[6] = uint16(v2>>8) & 0xfff
				t[7] = uint16(v2>>20) & 0xfff
				t[8] = uint16(v2>>32) & 0xfff
				t[9] = uint16(v2>>44) & 0xfff
				t[10] = uint16((v2>>56)|(v3<<8)) & 0xfff

				t[11] = uint16(v3>>4) & 0xfff
				t[12] = uint16(v3>>16) & 0xfff
				t[13] = uint16(v3>>28) & 0xfff
				t[14] = uint16(v3>>40) & 0xfff
				t[15] = uint16(v3>>52) & 0xfff

				for k := 0; k < 16; k++ {
					if t[k] < uint16(Q) {
						ps[j][idx[j]] = int16(t[k])
						idx[j]++
						if idx[j] == N {
							continue PolyLoop
						}
					}
				}
			}

			done = false
		}
	}

	for i := 0; i < 4; i++ {
		if ps[i] != nil {
			ps[i].Tangle()
		}
	}
}

// Sample p uniformly from the given seed and x and y coordinates.
//
// Coefficients are reduced and will be in "tangled" order.  See Tangle().
func (p *Poly) DeriveUniform(seed *[32]byte, x, y uint8) {
	var seedSuffix [2]byte
	var buf [168]byte // rate of SHAKE-128

	seedSuffix[0] = x
	seedSuffix[1] = y

	h := sha3.NewShake128()
	_, _ = h.Write(seed[:])
	_, _ = h.Write(seedSuffix[:])

	i := 0
	for {
		_, _ = h.Read(buf[:])

		for j := 0; j < 168; j += 3 {
			t1 := (uint16(buf[j]) | (uint16(buf[j+1]) << 8)) & 0xfff
			t2 := (uint16(buf[j+1]>>4) | (uint16(buf[j+2]) << 4)) & 0xfff

			if t1 < uint16(Q) {
				p[i] = int16(t1)
				i++

				if i == N {
					break
				}
			}

			if t2 < uint16(Q) {
				p[i] = int16(t2)
				i++

				if i == N {
					break
				}
			}
		}

		if i == N {
			break
		}
	}

	p.Tangle()
}
