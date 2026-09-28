// Code generated from mode3/internal/sample.go by gen.go

package internal

import (
	"encoding/binary"

	"github.com/cloudflare/circl/internal/sha3"
	common "github.com/cloudflare/circl/sign/internal/dilithium"
	"github.com/cloudflare/circl/simd/keccakf1600"
)

// DeriveX4Available indicates whether the system supports the quick fourway
// sampling variants like PolyDeriveUniformX4.
var DeriveX4Available = keccakf1600.IsEnabledX4()

// For each i, sample ps[i] uniformly from the given seed and nonces[i].
// ps[i] may be nil and is ignored in that case.
//
// Can only be called when DeriveX4Available is true.
func PolyDeriveUniformX4(ps [4]*common.Poly, seed *[32]byte, nonces [4]uint16) {
	var perm keccakf1600.StateX4
	state := perm.Initialize(false)

	// Absorb the seed in the four states
	for i := 0; i < 4; i++ {
		v := binary.LittleEndian.Uint64(seed[8*i : 8*(i+1)])
		for j := 0; j < 4; j++ {
			state[i*4+j] = v
		}
	}

	// Absorb the nonces, the SHAKE128 domain separator (0b1111), the
	// start of the padding (0b...001) and the end of the padding 0b100...
	// Recall that the rate of SHAKE128 is 168 --- i.e. 21 uint64s.
	for j := 0; j < 4; j++ {
		state[4*4+j] = uint64(nonces[j]) | (0x1f << 16)
		state[20*4+j] = 0x80 << 56
	}

	var idx [4]int // indices into ps
	for j := 0; j < 4; j++ {
		if ps[j] == nil {
			idx[j] = common.N // mark nil polynomial as completed
		}
	}

	done := false
	for !done {
		// Applies KeccaK-f[1600] to state to get the next 21 uint64s of each
		// of the four SHAKE128 streams.
		perm.Permute()

		done = true

	PolyLoop:
		for j := 0; j < 4; j++ {
			if idx[j] == common.N {
				continue
			}
			for i := 0; i < 7; i++ {
				var t [8]uint32
				t[0] = uint32(state[i*3*4+j] & 0x7fffff)
				t[1] = uint32((state[i*3*4+j] >> 24) & 0x7fffff)
				t[2] = uint32((state[i*3*4+j] >> 48) |
					((state[(i*3+1)*4+j] & 0x7f) << 16))
				t[3] = uint32((state[(i*3+1)*4+j] >> 8) & 0x7fffff)
				t[4] = uint32((state[(i*3+1)*4+j] >> 32) & 0x7fffff)
				t[5] = uint32((state[(i*3+1)*4+j] >> 56) |
					((state[(i*3+2)*4+j] & 0x7fff) << 8))
				t[6] = uint32((state[(i*3+2)*4+j] >> 16) & 0x7fffff)
				t[7] = uint32((state[(i*3+2)*4+j] >> 40) & 0x7fffff)

				for k := 0; k < 8; k++ {
					if t[k] < common.Q {
						ps[j][idx[j]] = t[k]
						idx[j]++
						if idx[j] == common.N {
							continue PolyLoop
						}
					}
				}
			}
			done = false
		}
	}
}

// Sample p uniformly from the given seed and nonce.
//
// p will be normalized.
func PolyDeriveUniform(p *common.Poly, seed *[32]byte, nonce uint16) {
	var i, length int
	var buf [12 * 16]byte // fits 168B SHAKE-128 rate

	length = 168

	sample := func() {
		// Note that 3 divides into 168 and 12*16, so we use up buf completely.
		for j := 0; j < length && i < common.N; j += 3 {
			t := (uint32(buf[j]) | (uint32(buf[j+1]) << 8) |
				(uint32(buf[j+2]) << 16)) & 0x7fffff

			// We use rejection sampling
			if t < common.Q {
				p[i] = t
				i++
			}
		}
	}

	var iv [32 + 2]byte // 32 byte seed + uint16 nonce
	h := sha3.NewShake128()
	copy(iv[:32], seed[:])
	iv[32] = uint8(nonce)
	iv[33] = uint8(nonce >> 8)
	_, _ = h.Write(iv[:])

	for i < common.N {
		_, _ = h.Read(buf[:168])
		sample()
	}
}

// Sample p uniformly with coefficients of norm less than or equal η,
// using the given seed and nonce.
//
// p will not be normalized, but will have coefficients in [q-η,q+η].
func PolyDeriveUniformLeqEta(p *common.Poly, seed *[64]byte, nonce uint16) {
	// Assumes 2 < η < 8.
	var i, length int
	var buf [9 * 16]byte // fits 136B SHAKE-256 rate

	length = 136

	sample := func() {
		// We use rejection sampling
		for j := 0; j < length && i < common.N; j++ {
			t1 := uint32(buf[j]) & 15
			t2 := uint32(buf[j]) >> 4
			if Eta == 2 { // branch is eliminated by compiler
				if t1 <= 14 {
					t1 -= ((205 * t1) >> 10) * 5 // reduce mod  5
					p[i] = common.Q + Eta - t1
					i++
				}
				if t2 <= 14 && i < common.N {
					t2 -= ((205 * t2) >> 10) * 5 // reduce mod 5
					p[i] = common.Q + Eta - t2
					i++
				}
			} else if Eta == 4 {
				if t1 <= 2*Eta {
					p[i] = common.Q + Eta - t1
					i++
				}
				if t2 <= 2*Eta && i < common.N {
					p[i] = common.Q + Eta - t2
					i++
				}
			} else {
				panic("unsupported η")
			}
		}
	}

	var iv [64 + 2]byte // 64 byte seed + uint16 nonce

	h := sha3.NewShake256()
	copy(iv[:64], seed[:])
	iv[64] = uint8(nonce)
	iv[65] = uint8(nonce >> 8)

	// 136 is SHAKE-256 rate
	_, _ = h.Write(iv[:])

	for i < common.N {
		_, _ = h.Read(buf[:136])
		sample()
	}
}

// Sample v[i] uniformly with coefficients in (-γ₁,…,γ₁]  using the
// given seed and nonce+i
//
// p will be normalized.
func VecLDeriveUniformLeGamma1(v *VecL, seed *[64]byte, nonce uint16) {
	for i := 0; i < L; i++ {
		PolyDeriveUniformLeGamma1(&v[i], seed, nonce+uint16(i))
	}
}

// Sample p uniformly with coefficients in (-γ₁,…,γK1s] using the
// given seed and nonce.
//
// p will be normalized.
func PolyDeriveUniformLeGamma1(p *common.Poly, seed *[64]byte, nonce uint16) {
	var buf [PolyLeGamma1Size]byte

	var iv [66]byte
	h := sha3.NewShake256()
	copy(iv[:64], seed[:])
	iv[64] = uint8(nonce)
	iv[65] = uint8(nonce >> 8)
	_, _ = h.Write(iv[:])
	_, _ = h.Read(buf[:])

	PolyUnpackLeGamma1(p, buf[:])
}

// For each i, sample ps[i] uniformly with τ non-zero coefficients in {q-1,1}
// using the given seed and w1[i].  ps[i] may be nil and is ignored
// in that case.  ps[i] will be normalized.
//
// Can only be called when DeriveX4Available is true.
//
// This function is currently not used (yet).
func PolyDeriveUniformBallX4(ps [4]*common.Poly, seed []byte) {
	var perm keccakf1600.StateX4
	state := perm.Initialize(false)

	// Absorb the seed in the four states
	for i := 0; i < CTildeSize/8; i++ {
		v := binary.LittleEndian.Uint64(seed[8*i : 8*(i+1)])
		for j := 0; j < 4; j++ {
			state[i*4+j] = v
		}
	}

	// SHAKE256 domain separator and padding
	for j := 0; j < 4; j++ {
		state[(CTildeSize/8)*4+j] ^= 0x1f
		state[16*4+j] ^= 0x80 << 56
	}
	perm.Permute()

	var signs [4]uint64
	var idx [4]uint16 // indices into ps

	for j := 0; j < 4; j++ {
		if ps[j] != nil {
			signs[j] = state[j]
			*ps[j] = common.Poly{} // zero ps[j]
			idx[j] = common.N - Tau
		} else {
			idx[j] = common.N // mark as completed
		}
	}

	stateOffset := 1
	for {
		done := true

	PolyLoop:
		for j := 0; j < 4; j++ {
			if idx[j] == common.N {
				continue
			}

			for i := stateOffset; i < 17; i++ {
				var bs [8]byte
				binary.LittleEndian.PutUint64(bs[:], state[4*i+j])
				for k := 0; k < 8; k++ {
					b := uint16(bs[k])

					if b > idx[j] {
						continue
					}

					ps[j][idx[j]] = ps[j][b]
					ps[j][b] = 1
					// Takes least significant bit of signs and uses it for the sign.
					// Note 1 ^ (1 | (Q-1)) = Q-1.
					ps[j][b] ^= uint32((-(signs[j] & 1)) & (1 | (common.Q - 1)))
					signs[j] >>= 1

					idx[j]++
					if idx[j] == common.N {
						continue PolyLoop
					}
				}
			}

			done = false
		}

		if done {
			break
		}

		perm.Permute()
		stateOffset = 0
	}
}

// Samples p uniformly with τ non-zero coefficients in {q-1,1}.
//
// The polynomial p will be normalized.
func PolyDeriveUniformBall(p *common.Poly, seed []byte) {
	var buf [136]byte // SHAKE-256 rate is 136

	h := sha3.NewShake256()
	_, _ = h.Write(seed[:])
	_, _ = h.Read(buf[:])

	// Essentially we generate a sequence of τ ones or minus ones,
	// prepend 196 zeroes and shuffle the concatenation using the
	// usual algorithm (Fisher--Yates.)
	signs := binary.LittleEndian.Uint64(buf[:])
	bufOff := 8 // offset into buf

	*p = common.Poly{} // zero p
	for i := uint16(common.N - Tau); i < common.N; i++ {
		var b uint16

		// Find location of where to move the new coefficient to using
		// rejection sampling.
		for {
			if bufOff >= 136 {
				_, _ = h.Read(buf[:])
				bufOff = 0
			}

			b = uint16(buf[bufOff])
			bufOff++

			if b <= i {
				break
			}
		}

		p[i] = p[b]
		p[b] = 1
		// Takes least significant bit of signs and uses it for the sign.
		// Note 1 ^ (1 | (Q-1)) = Q-1.
		p[b] ^= uint32((-(signs & 1)) & (1 | (common.Q - 1)))
		signs >>= 1
	}
}
