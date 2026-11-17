package common

// An element of our base ring R which are polynomials over ℤ_q
// modulo the equation Xᴺ = -1, where q=3329 and N=256.
//
// This type is also used to store NTT-transformed polynomials,
// see Poly.NTT().
//
// Coefficients aren't always reduced.  See Normalize().
type Poly [N]int16

// Sets p to a + b.  Does not normalize coefficients.
func (p *Poly) addGeneric(a, b *Poly) {
	for i := 0; i < N; i++ {
		p[i] = a[i] + b[i]
	}
}

// Sets p to a - b.  Does not normalize coefficients.
func (p *Poly) subGeneric(a, b *Poly) {
	for i := 0; i < N; i++ {
		p[i] = a[i] - b[i]
	}
}

// Almost normalizes coefficients.
//
// Ensures each coefficient is in {0, …, q}.
func (p *Poly) barrettReduceGeneric() {
	for i := 0; i < N; i++ {
		p[i] = barrettReduce(p[i])
	}
}

// Normalizes coefficients.
//
// Ensures each coefficient is in {0, …, q-1}.
func (p *Poly) normalizeGeneric() {
	for i := 0; i < N; i++ {
		p[i] = csubq(barrettReduce(p[i]))
	}
}

// Multiplies p in-place by the Montgomery factor 2¹⁶.
//
// Coefficients of p can be arbitrary.  Resulting coefficients are bounded
// in absolute value by q.
func (p *Poly) ToMont() {
	for i := 0; i < N; i++ {
		p[i] = toMont(p[i])
	}
}

// Sets p to the "pointwise" multiplication of a and b.
//
// That is: InvNTT(p) = InvNTT(a) * InvNTT(b).  Assumes a and b are in
// Montgomery form.  Products between coefficients of a and b must be strictly
// bounded in absolute value by 2¹⁵q.  p will be in Montgomery form and
// bounded in absolute value by 2q.
//
// Requires a and b to be in "tangled" order, see Tangle().  p will be in
// tangled order as well.
func (p *Poly) mulHatGeneric(a, b *Poly) {
	// Recall from the discussion in NTT(), that a transformed polynomial is
	// an element of ℤ_q[x]/(x²-ζ) x … x  ℤ_q[x]/(x²+ζ¹²⁷);
	// that is: 128 degree-one polynomials instead of simply 256 elements
	// from ℤ_q as in the regular NTT.  So instead of pointwise multiplication,
	// we multiply the 128 pairs of degree-one polynomials modulo the
	// right equation:
	//
	//  (a₁ + a₂x)(b₁ + b₂x) = a₁b₁ + a₂b₂ζ' + (a₁b₂ + a₂b₁)x,
	//
	// where ζ' is the appropriate power of ζ.

	k := 64
	for i := 0; i < N; i += 4 {
		zeta := int32(Zetas[k])
		k++

		p0 := montReduce(int32(a[i+1]) * int32(b[i+1]))
		p0 = montReduce(int32(p0) * zeta)
		p0 += montReduce(int32(a[i]) * int32(b[i]))

		p1 := montReduce(int32(a[i]) * int32(b[i+1]))
		p1 += montReduce(int32(a[i+1]) * int32(b[i]))

		p[i] = p0
		p[i+1] = p1

		p2 := montReduce(int32(a[i+3]) * int32(b[i+3]))
		p2 = -montReduce(int32(p2) * zeta)
		p2 += montReduce(int32(a[i+2]) * int32(b[i+2]))

		p3 := montReduce(int32(a[i+2]) * int32(b[i+3]))
		p3 += montReduce(int32(a[i+3]) * int32(b[i+2]))

		p[i+2] = p2
		p[i+3] = p3
	}
}

// Packs p into buf.  buf should be of length PolySize.
//
// Assumes p is normalized (and not just Barrett reduced) and "tangled",
// see Tangle().
func (p *Poly) Pack(buf []byte) {
	q := *p
	q.Detangle()
	for i := 0; i < 128; i++ {
		t0 := q[2*i]
		t1 := q[2*i+1]
		buf[3*i] = byte(t0)
		buf[3*i+1] = byte(t0>>8) | byte(t1<<4)
		buf[3*i+2] = byte(t1 >> 4)
	}
}

// Unpacks p from buf.
//
// buf should be of length PolySize.  p will be "tangled", see Detangle().
//
// p will not be normalized; instead 0 ≤ p[i] < 4096.
func (p *Poly) Unpack(buf []byte) {
	for i := 0; i < 128; i++ {
		p[2*i] = int16(buf[3*i]) | ((int16(buf[3*i+1]) << 8) & 0xfff)
		p[2*i+1] = int16(buf[3*i+1]>>4) | (int16(buf[3*i+2]) << 4)
	}
	p.Tangle()
}

// Set p to Decompress_q(m, 1).
//
// p will be normalized.  m has to be of PlaintextSize.
func (p *Poly) DecompressMessage(m []byte) {
	// Decompress_q(x, 1) = ⌈xq/2⌋ = ⌊xq/2+½⌋ = (xq+1) >> 1 and so
	// Decompress_q(0, 1) = 0 and Decompress_q(1, 1) = (q+1)/2.
	for i := 0; i < 32; i++ {
		for j := 0; j < 8; j++ {
			bit := (m[i] >> uint(j)) & 1

			// Set coefficient to either 0 or (q+1)/2 depending on the bit.
			p[8*i+j] = -int16(bit) & ((Q + 1) / 2)
		}
	}
}

// Writes Compress_q(p, 1) to m.
//
// Assumes p is normalized.  m has to be of length at least PlaintextSize.
func (p *Poly) CompressMessageTo(m []byte) {
	// Compress_q(x, 1) is 1 on {833, …, 2496} and zero elsewhere.
	for i := 0; i < 32; i++ {
		m[i] = 0
		for j := 0; j < 8; j++ {
			x := 1664 - p[8*i+j]
			// With the previous substitution, we want to return 1 if
			// and only if x is in {831, …, -832}.
			x = (x >> 15) ^ x
			// Note (x >> 15)ˣ if x≥0 and -x-1 otherwise. Thus now we want
			// to return 1 iff x ≤ 831, ie. x - 832 < 0.
			x -= 832
			m[i] |= ((byte(x >> 15)) & 1) << uint(j)
		}
	}
}

// Set p to Decompress_q(m, 1).
//
// Assumes d is in {4, 5, 10, 11}.  p will be normalized.
func (p *Poly) Decompress(m []byte, d int) {
	// Decompress_q(x, d) = ⌈(q/2ᵈ)x⌋
	//                    = ⌊(q/2ᵈ)x+½⌋
	//                    = ⌊(qx + 2ᵈ⁻¹)/2ᵈ⌋
	//                    = (qx + (1<<(d-1))) >> d
	switch d {
	case 4:
		for i := 0; i < N/2; i++ {
			p[2*i] = int16(((1 << 3) +
				uint32(m[i]&15)*uint32(Q)) >> 4)
			p[2*i+1] = int16(((1 << 3) +
				uint32(m[i]>>4)*uint32(Q)) >> 4)
		}
	case 5:
		var t [8]uint16
		idx := 0
		for i := 0; i < N/8; i++ {
			t[0] = uint16(m[idx])
			t[1] = (uint16(m[idx]) >> 5) | (uint16(m[idx+1] << 3))
			t[2] = uint16(m[idx+1]) >> 2
			t[3] = (uint16(m[idx+1]) >> 7) | (uint16(m[idx+2] << 1))
			t[4] = (uint16(m[idx+2]) >> 4) | (uint16(m[idx+3] << 4))
			t[5] = uint16(m[idx+3]) >> 1
			t[6] = (uint16(m[idx+3]) >> 6) | (uint16(m[idx+4] << 2))
			t[7] = uint16(m[idx+4]) >> 3

			for j := 0; j < 8; j++ {
				p[8*i+j] = int16(((1 << 4) +
					uint32(t[j]&((1<<5)-1))*uint32(Q)) >> 5)
			}

			idx += 5
		}

	case 10:
		var t [4]uint16
		idx := 0
		for i := 0; i < N/4; i++ {
			t[0] = uint16(m[idx]) | (uint16(m[idx+1]) << 8)
			t[1] = (uint16(m[idx+1]) >> 2) | (uint16(m[idx+2]) << 6)
			t[2] = (uint16(m[idx+2]) >> 4) | (uint16(m[idx+3]) << 4)
			t[3] = (uint16(m[idx+3]) >> 6) | (uint16(m[idx+4]) << 2)

			for j := 0; j < 4; j++ {
				p[4*i+j] = int16(((1 << 9) +
					uint32(t[j]&((1<<10)-1))*uint32(Q)) >> 10)
			}

			idx += 5
		}
	case 11:
		var t [8]uint16
		idx := 0
		for i := 0; i < N/8; i++ {
			t[0] = uint16(m[idx]) | (uint16(m[idx+1]) << 8)
			t[1] = (uint16(m[idx+1]) >> 3) | (uint16(m[idx+2]) << 5)
			t[2] = (uint16(m[idx+2]) >> 6) | (uint16(m[idx+3]) << 2) | (uint16(m[idx+4]) << 10)
			t[3] = (uint16(m[idx+4]) >> 1) | (uint16(m[idx+5]) << 7)
			t[4] = (uint16(m[idx+5]) >> 4) | (uint16(m[idx+6]) << 4)
			t[5] = (uint16(m[idx+6]) >> 7) | (uint16(m[idx+7]) << 1) | (uint16(m[idx+8]) << 9)
			t[6] = (uint16(m[idx+8]) >> 2) | (uint16(m[idx+9]) << 6)
			t[7] = (uint16(m[idx+9]) >> 5) | (uint16(m[idx+10]) << 3)

			for j := 0; j < 8; j++ {
				p[8*i+j] = int16(((1 << 10) +
					uint32(t[j]&((1<<11)-1))*uint32(Q)) >> 11)
			}

			idx += 11
		}
	default:
		panic("unsupported d")
	}
}

// Writes Compress_q(p, d) to m.
//
// Assumes p is normalized and d is in {4, 5, 10, 11}.
func (p *Poly) CompressTo(m []byte, d int) {
	// Compress_q(x, d) = ⌈(2ᵈ/q)x⌋ mod⁺ 2ᵈ
	//                  = ⌊(2ᵈ/q)x+½⌋ mod⁺ 2ᵈ
	//					= ⌊((x << d) + q/2) / q⌋ mod⁺ 2ᵈ
	//					= DIV((x << d) + q/2, q) & ((1<<d) - 1)
	//
	// We approximate DIV(x, q) by computing (x*a)>>e, where a/(2^e) ≈ 1/q.
	// For d in {10,11} we use 20,642,679/2^36, which computes division by x/q
	// correctly for 0 ≤ x < 41,522,616, which fits (q << 11) + q/2 comfortably.
	// For d in {4,5} we use 315/2^20, which doesn't compute division by x/q
	// correctly for all inputs, but it's close enough that the end result
	// of the compression is correct. The advantage is that we do not need
	// to use a 64-bit intermediate value.
	switch d {
	case 4:
		var t [8]uint16
		idx := 0
		for i := 0; i < N/8; i++ {
			for j := 0; j < 8; j++ {
				t[j] = uint16((((uint32(p[8*i+j])<<4)+uint32(Q)/2)*315)>>
					20) & ((1 << 4) - 1)
			}
			m[idx] = byte(t[0]) | byte(t[1]<<4)
			m[idx+1] = byte(t[2]) | byte(t[3]<<4)
			m[idx+2] = byte(t[4]) | byte(t[5]<<4)
			m[idx+3] = byte(t[6]) | byte(t[7]<<4)
			idx += 4
		}

	case 5:
		var t [8]uint16
		idx := 0
		for i := 0; i < N/8; i++ {
			for j := 0; j < 8; j++ {
				t[j] = uint16((((uint32(p[8*i+j])<<5)+uint32(Q)/2)*315)>>
					20) & ((1 << 5) - 1)
			}
			m[idx] = byte(t[0]) | byte(t[1]<<5)
			m[idx+1] = byte(t[1]>>3) | byte(t[2]<<2) | byte(t[3]<<7)
			m[idx+2] = byte(t[3]>>1) | byte(t[4]<<4)
			m[idx+3] = byte(t[4]>>4) | byte(t[5]<<1) | byte(t[6]<<6)
			m[idx+4] = byte(t[6]>>2) | byte(t[7]<<3)
			idx += 5
		}

	case 10:
		var t [4]uint16
		idx := 0
		for i := 0; i < N/4; i++ {
			for j := 0; j < 4; j++ {
				t[j] = uint16((uint64((uint32(p[4*i+j])<<10)+uint32(Q)/2)*
					20642679)>>36) & ((1 << 10) - 1)
			}
			m[idx] = byte(t[0])
			m[idx+1] = byte(t[0]>>8) | byte(t[1]<<2)
			m[idx+2] = byte(t[1]>>6) | byte(t[2]<<4)
			m[idx+3] = byte(t[2]>>4) | byte(t[3]<<6)
			m[idx+4] = byte(t[3] >> 2)
			idx += 5
		}
	case 11:
		var t [8]uint16
		idx := 0
		for i := 0; i < N/8; i++ {
			for j := 0; j < 8; j++ {
				t[j] = uint16((uint64((uint32(p[8*i+j])<<11)+uint32(Q)/2)*
					20642679)>>36) & ((1 << 11) - 1)
			}
			m[idx] = byte(t[0])
			m[idx+1] = byte(t[0]>>8) | byte(t[1]<<3)
			m[idx+2] = byte(t[1]>>5) | byte(t[2]<<6)
			m[idx+3] = byte(t[2] >> 2)
			m[idx+4] = byte(t[2]>>10) | byte(t[3]<<1)
			m[idx+5] = byte(t[3]>>7) | byte(t[4]<<4)
			m[idx+6] = byte(t[4]>>4) | byte(t[5]<<7)
			m[idx+7] = byte(t[5] >> 1)
			m[idx+8] = byte(t[5]>>9) | byte(t[6]<<2)
			m[idx+9] = byte(t[6]>>6) | byte(t[7]<<5)
			m[idx+10] = byte(t[7] >> 3)
			idx += 11
		}
	default:
		panic("unsupported d")
	}
}
