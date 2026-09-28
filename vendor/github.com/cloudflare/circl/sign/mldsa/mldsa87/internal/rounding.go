// Code generated from mode3/internal/rounding.go by gen.go

package internal

import (
	common "github.com/cloudflare/circl/sign/internal/dilithium"
)

// Splits 0 ≤ a < q into a₀ and a₁ with a = a₁*α + a₀ with -α/2 < a₀ ≤ α/2,
// except for when we would have a₁ = (q-1)/α in which case a₁=0 is taken
// and -α/2 ≤ a₀ < 0.  Returns a₀ + q.  Note 0 ≤ a₁ < (q-1)/α.
// Recall α = 2γ₂.
func decompose(a uint32) (a0plusQ, a1 uint32) {
	// a₁ = ⌈a / 128⌉
	a1 = (a + 127) >> 7

	if Alpha == 523776 {
		// 1025/2²² is close enough to 1/4092 so that a₁
		// becomes a/α rounded down.
		a1 = ((a1*1025 + (1 << 21)) >> 22)

		// For the corner-case a₁ = (q-1)/α = 16, we have to set a₁=0.
		a1 &= 15
	} else if Alpha == 190464 {
		// 1488/2²⁴ is close enough to 1/1488 so that a₁
		// becomes a/α rounded down.
		a1 = ((a1 * 11275) + (1 << 23)) >> 24

		// For the corner-case a₁ = (q-1)/α = 44, we have to set a₁=0.
		a1 ^= uint32(int32(43-a1)>>31) & a1
	} else {
		panic("unsupported α")
	}

	a0plusQ = a - a1*Alpha

	// In the corner-case, when we set a₁=0, we will incorrectly
	// have a₀ > (q-1)/2 and we'll need to subtract q.  As we
	// return a₀ + q, that comes down to adding q if a₀ < (q-1)/2.
	a0plusQ += uint32(int32(a0plusQ-(common.Q-1)/2)>>31) & common.Q

	return
}

// Assume 0 ≤ r, f < Q with ‖f‖_∞ ≤ α/2.  Decompose r as r = r1*α + r0 as
// computed by decompose().  Write r' := r - f (mod Q).  Now, decompose
// r'=r-f again as  r' = r'1*α + r'0 using decompose().  As f is small, we
// have r'1 = r1 + h, where h ∈ {-1, 0, 1}.  makeHint() computes |h|
// given z0 := r0 - f (mod Q) and r1.  With |h|, which is called the hint,
// we can reconstruct r1 using only r' = r - f, which is done by useHint().
// To wit:
//
//	useHint( r - f, makeHint( r0 - f, r1 ) ) = r1.
//
// Assumes 0 ≤ z0 < Q.
func makeHint(z0, r1 uint32) uint32 {
	// If -α/2 < r0 - f ≤ α/2, then r1*α + r0 - f is a valid decomposition of r'
	// with the restrictions of decompose() and so r'1 = r1.  So the hint
	// should be 0. This is covered by the first two inequalities.
	// There is one other case: if r0 - f = -α/2, then r1*α + r0 - f is also
	// a valid decomposition if r1 = 0.  In the other cases a one is carried
	// and the hint should be 1.
	if z0 <= Gamma2 || z0 > common.Q-Gamma2 || (z0 == common.Q-Gamma2 && r1 == 0) {
		return 0
	}
	return 1
}

// Uses the hint created by makeHint() to reconstruct r1 from r'=r-f; see
// documentation of makeHint() for context.
// Assumes 0 ≤ r' < Q.
func useHint(rp uint32, hint uint32) uint32 {
	rp0plusQ, rp1 := decompose(rp)
	if hint == 0 {
		return rp1
	}
	if rp0plusQ > common.Q {
		return (rp1 + 1) & 15
	}
	return (rp1 - 1) & 15
}

// Sets p to the hint polynomial for p0 the modified low bits and p1
// the unmodified high bits --- see makeHint().
//
// Returns the number of ones in the hint polynomial.
func PolyMakeHint(p, p0, p1 *common.Poly) (pop uint32) {
	for i := 0; i < common.N; i++ {
		h := makeHint(p0[i], p1[i])
		pop += h
		p[i] = h
	}
	return
}

// Computes corrections to the high bits of the polynomial q according
// to the hints in h and sets p to the corrected high bits.  Returns p.
func PolyUseHint(p, q, hint *common.Poly) {
	var q0PlusQ common.Poly

	// See useHint() and makeHint() for an explanation.  We reimplement it
	// here so that we can call Poly.Decompose(), which might be way faster
	// than calling decompose() in a loop (for instance when having AVX2.)

	PolyDecompose(q, &q0PlusQ, p)

	for i := 0; i < common.N; i++ {
		if hint[i] == 0 {
			continue
		}
		if Gamma2 == 261888 {
			if q0PlusQ[i] > common.Q {
				p[i] = (p[i] + 1) & 15
			} else {
				p[i] = (p[i] - 1) & 15
			}
		} else if Gamma2 == 95232 {
			if q0PlusQ[i] > common.Q {
				if p[i] == 43 {
					p[i] = 0
				} else {
					p[i]++
				}
			} else {
				if p[i] == 0 {
					p[i] = 43
				} else {
					p[i]--
				}
			}
		} else {
			panic("unsupported γ₂")
		}
	}
}

// Splits each of the coefficients of p using decompose.
func PolyDecompose(p, p0PlusQ, p1 *common.Poly) {
	for i := 0; i < common.N; i++ {
		p0PlusQ[i], p1[i] = decompose(p[i])
	}
}
