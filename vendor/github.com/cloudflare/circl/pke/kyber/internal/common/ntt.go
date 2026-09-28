package common

// Zetas lists precomputed powers of the primitive root of unity in
// Montgomery representation used for the NTT:
//
//	Zetas[i] = ζᵇʳᵛ⁽ⁱ⁾ R mod q
//
// where ζ = 17, brv(i) is the bitreversal of a 7-bit number and R=2¹⁶ mod q.
//
// The following Python code generates the Zetas arrays:
//
//	q = 13*2**8 + 1; zeta = 17
//	R = 2**16 % q # Montgomery const.
//	def brv(x): return int(''.join(reversed(bin(x)[2:].zfill(7))),2)
//	print([(pow(zeta, brv(i), q)*R)%q for i in range(128)])
var Zetas = [128]int16{
	2285, 2571, 2970, 1812, 1493, 1422, 287, 202, 3158, 622, 1577, 182,
	962, 2127, 1855, 1468, 573, 2004, 264, 383, 2500, 1458, 1727, 3199,
	2648, 1017, 732, 608, 1787, 411, 3124, 1758, 1223, 652, 2777, 1015,
	2036, 1491, 3047, 1785, 516, 3321, 3009, 2663, 1711, 2167, 126,
	1469, 2476, 3239, 3058, 830, 107, 1908, 3082, 2378, 2931, 961, 1821,
	2604, 448, 2264, 677, 2054, 2226, 430, 555, 843, 2078, 871, 1550,
	105, 422, 587, 177, 3094, 3038, 2869, 1574, 1653, 3083, 778, 1159,
	3182, 2552, 1483, 2727, 1119, 1739, 644, 2457, 349, 418, 329, 3173,
	3254, 817, 1097, 603, 610, 1322, 2044, 1864, 384, 2114, 3193, 1218,
	1994, 2455, 220, 2142, 1670, 2144, 1799, 2051, 794, 1819, 2475,
	2459, 478, 3221, 3021, 996, 991, 958, 1869, 1522, 1628,
}

// InvNTTReductions keeps track of which coefficients to apply Barrett
// reduction to in Poly.InvNTT().
//
// Generated in a lazily: once a butterfly is computed which is about to
// overflow the int16, the largest coefficient is reduced.  If that is
// not enough, the other coefficient is reduced as well.
//
// This is actually optimal, as proven in https://eprint.iacr.org/2020/1377.pdf
var InvNTTReductions = [...]int{
	-1, // after layer 1
	-1, // after layer 2
	16, 17, 48, 49, 80, 81, 112, 113, 144, 145, 176, 177, 208, 209, 240,
	241, -1, // after layer 3
	0, 1, 32, 33, 34, 35, 64, 65, 96, 97, 98, 99, 128, 129, 160, 161, 162, 163,
	192, 193, 224, 225, 226, 227, -1, // after layer 4
	2, 3, 66, 67, 68, 69, 70, 71, 130, 131, 194, 195, 196, 197, 198,
	199, -1, // after layer 5
	4, 5, 6, 7, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142,
	143, -1, // after layer 6
	-1, //  after layer 7
}

// Executes an in-place forward "NTT" on p.
//
// Assumes the coefficients are in absolute value ≤q.  The resulting
// coefficients are in absolute value ≤7q.  If the input is in Montgomery
// form, then the result is in Montgomery form and so (by linearity of the NTT)
// if the input is in regular form, then the result is also in regular form.
// The order of coefficients will be "tangled". These can be put back into
// their proper order by calling Detangle().
func (p *Poly) nttGeneric() {
	// Note that ℤ_q does not have a primitive 512ᵗʰ root of unity (as 512
	// does not divide into q-1) and so we cannot do a regular NTT.  ℤ_q
	// does have a primitive 256ᵗʰ root of unity, the smallest of which
	// is ζ := 17.
	//
	// Recall that our base ring R := ℤ_q[x] / (x²⁵⁶ + 1).  The polynomial
	// x²⁵⁶+1 will not split completely (as its roots would be 512ᵗʰ roots
	// of unity.)  However, it does split almost (using ζ¹²⁸ = -1):
	//
	// x²⁵⁶ + 1 = (x²)¹²⁸ - ζ¹²⁸
	//          = ((x²)⁶⁴ - ζ⁶⁴)((x²)⁶⁴ + ζ⁶⁴)
	//          = ((x²)³² - ζ³²)((x²)³² + ζ³²)((x²)³² - ζ⁹⁶)((x²)³² + ζ⁹⁶)
	//          ⋮
	//          = (x² - ζ)(x² + ζ)(x² - ζ⁶⁵)(x² + ζ⁶⁵) … (x² + ζ¹²⁷)
	//
	// Note that the powers of ζ that appear (from the second line down) are
	// in binary
	//
	// 0100000 1100000
	// 0010000 1010000 0110000 1110000
	// 0001000 1001000 0101000 1101000 0011000 1011000 0111000 1111000
	//         …
	//
	// That is: brv(2), brv(3), brv(4), …, where brv(x) denotes the 7-bit
	// bitreversal of x.  These powers of ζ are given by the Zetas array.
	//
	// The polynomials x² ± ζⁱ are irreducible and coprime, hence by
	// the Chinese Remainder Theorem we know
	//
	//  ℤ_q[x]/(x²⁵⁶+1) → ℤ_q[x]/(x²-ζ) x … x  ℤ_q[x]/(x²+ζ¹²⁷)
	//
	// given by a ↦ ( a mod x²-ζ, …, a mod x²+ζ¹²⁷ )
	// is an isomorphism, which is the "NTT".  It can be efficiently computed by
	//
	//
	//  a ↦ ( a mod (x²)⁶⁴ - ζ⁶⁴, a mod (x²)⁶⁴ + ζ⁶⁴ )
	//    ↦ ( a mod (x²)³² - ζ³², a mod (x²)³² + ζ³²,
	//        a mod (x²)⁹⁶ - ζ⁹⁶, a mod (x²)⁹⁶ + ζ⁹⁶ )
	//
	//	    et cetera
	//
	// If N was 8 then this can be pictured in the following diagram:
	//
	//  https://cnx.org/resources/17ee4dfe517a6adda05377b25a00bf6e6c93c334/File0026.png
	//
	// Each cross is a Cooley-Tukey butterfly: it's the map
	//
	//  (a, b) ↦ (a + ζb, a - ζb)
	//
	// for the appropriate power ζ for that column and row group.

	k := 0 // Index into Zetas

	// l runs effectively over the columns in the diagram above; it is half the
	// height of a row group, i.e. the number of butterflies in each row group.
	// In the diagram above it would be 4, 2, 1.
	for l := N / 2; l > 1; l >>= 1 {
		// On the nᵗʰ iteration of the l-loop, the absolute value of the
		// coefficients are bounded by nq.

		// offset effectively loops over the row groups in this column; it is
		// the first row in the row group.
		for offset := 0; offset < N-l; offset += 2 * l {
			k++
			zeta := int32(Zetas[k])

			// j loops over each butterfly in the row group.
			for j := offset; j < offset+l; j++ {
				t := montReduce(zeta * int32(p[j+l]))
				p[j+l] = p[j] - t
				p[j] += t
			}
		}
	}
}

// Executes an in-place inverse "NTT" on p and multiply by the Montgomery
// factor R.
//
// Requires coefficients to be in "tangled" order, see Tangle().
// Assumes the coefficients are in absolute value ≤q.  The resulting
// coefficients are in absolute value ≤q.  If the input is in Montgomery
// form, then the result is in Montgomery form and so (by linearity)
// if the input is in regular form, then the result is also in regular form.
func (p *Poly) invNTTGeneric() {
	k := 127 // Index into Zetas
	r := -1  // Index into InvNTTReductions.

	// We basically do the opposite of NTT, but postpone dividing by 2 in the
	// inverse of the Cooley-Tukey butterfly and accumulate that into a big
	// division by 2⁷ at the end.  See the comments in the NTT() function.

	for l := 2; l < N; l <<= 1 {
		for offset := 0; offset < N-l; offset += 2 * l {
			// As we're inverting, we need powers of ζ⁻¹ (instead of ζ).
			// To be precise, we need ζᵇʳᵛ⁽ᵏ⁾⁻¹²⁸. However, as ζ⁻¹²⁸ = -1,
			// we can use the existing Zetas table instead of
			// keeping a separate InvZetas table as in Dilithium.

			minZeta := int32(Zetas[k])
			k--

			for j := offset; j < offset+l; j++ {
				// Gentleman-Sande butterfly: (a, b) ↦ (a + b, ζ(a-b))
				t := p[j+l] - p[j]
				p[j] += p[j+l]
				p[j+l] = montReduce(minZeta * int32(t))

				// Note that if we had |a| < αq and |b| < βq before the
				// butterfly, then now we have |a| < (α+β)q and |b| < q.
			}
		}

		// We let the InvNTTReductions instruct us which coefficients to
		// Barrett reduce.  See TestInvNTTReductions, which tests whether
		// there is an overflow.
		for {
			r++
			i := InvNTTReductions[r]
			if i < 0 {
				break
			}
			p[i] = barrettReduce(p[i])
		}
	}

	for j := 0; j < N; j++ {
		// Note 1441 = (128)⁻¹ R².  The coefficients are bounded by 9q, so
		// as 1441 * 9 ≈ 2¹⁴ < 2¹⁵, we're within the required bounds
		// for montReduce().
		p[j] = montReduce(1441 * int32(p[j]))
	}
}
