// Code generated from mode3/internal/vec.go by gen.go

package internal

import (
	common "github.com/cloudflare/circl/sign/internal/dilithium"
)

// A vector of L polynomials.
type VecL [L]common.Poly

// A vector of K polynomials.
type VecK [K]common.Poly

// Normalize the polynomials in this vector.
func (v *VecL) Normalize() {
	for i := 0; i < L; i++ {
		v[i].Normalize()
	}
}

// Normalize the polynomials in this vector assuming their coefficients
// are already bounded by 2q.
func (v *VecL) NormalizeAssumingLe2Q() {
	for i := 0; i < L; i++ {
		v[i].NormalizeAssumingLe2Q()
	}
}

// Sets v to w + u.  Does not normalize.
func (v *VecL) Add(w, u *VecL) {
	for i := 0; i < L; i++ {
		v[i].Add(&w[i], &u[i])
	}
}

// Applies NTT componentwise. See Poly.NTT() for details.
func (v *VecL) NTT() {
	for i := 0; i < L; i++ {
		v[i].NTT()
	}
}

// Checks whether any of the coefficients exceeds the given bound in supnorm
//
// Requires the vector to be normalized.
func (v *VecL) Exceeds(bound uint32) bool {
	for i := 0; i < L; i++ {
		if v[i].Exceeds(bound) {
			return true
		}
	}
	return false
}

// Applies Poly.Power2Round componentwise.
//
// Requires the vector to be normalized.
func (v *VecL) Power2Round(v0PlusQ, v1 *VecL) {
	for i := 0; i < L; i++ {
		v[i].Power2Round(&v0PlusQ[i], &v1[i])
	}
}

// Applies Poly.Decompose componentwise.
//
// Requires the vector to be normalized.
func (v *VecL) Decompose(v0PlusQ, v1 *VecL) {
	for i := 0; i < L; i++ {
		PolyDecompose(&v[i], &v0PlusQ[i], &v1[i])
	}
}

// Sequentially packs each polynomial using Poly.PackLeqEta().
func (v *VecL) PackLeqEta(buf []byte) {
	offset := 0
	for i := 0; i < L; i++ {
		PolyPackLeqEta(&v[i], buf[offset:])
		offset += PolyLeqEtaSize
	}
}

// Sets v to the polynomials packed in buf using VecL.PackLeqEta().
func (v *VecL) UnpackLeqEta(buf []byte) {
	offset := 0
	for i := 0; i < L; i++ {
		PolyUnpackLeqEta(&v[i], buf[offset:])
		offset += PolyLeqEtaSize
	}
}

// Sequentially packs each polynomial using PolyPackLeGamma1().
func (v *VecL) PackLeGamma1(buf []byte) {
	offset := 0
	for i := 0; i < L; i++ {
		PolyPackLeGamma1(&v[i], buf[offset:])
		offset += PolyLeGamma1Size
	}
}

// Sets v to the polynomials packed in buf using VecL.PackLeGamma1().
func (v *VecL) UnpackLeGamma1(buf []byte) {
	offset := 0
	for i := 0; i < L; i++ {
		PolyUnpackLeGamma1(&v[i], buf[offset:])
		offset += PolyLeGamma1Size
	}
}

// Normalize the polynomials in this vector.
func (v *VecK) Normalize() {
	for i := 0; i < K; i++ {
		v[i].Normalize()
	}
}

// Normalize the polynomials in this vector assuming their coefficients
// are already bounded by 2q.
func (v *VecK) NormalizeAssumingLe2Q() {
	for i := 0; i < K; i++ {
		v[i].NormalizeAssumingLe2Q()
	}
}

// Sets v to w + u.  Does not normalize.
func (v *VecK) Add(w, u *VecK) {
	for i := 0; i < K; i++ {
		v[i].Add(&w[i], &u[i])
	}
}

// Checks whether any of the coefficients exceeds the given bound in supnorm
//
// Requires the vector to be normalized.
func (v *VecK) Exceeds(bound uint32) bool {
	for i := 0; i < K; i++ {
		if v[i].Exceeds(bound) {
			return true
		}
	}
	return false
}

// Applies Poly.Power2Round componentwise.
//
// Requires the vector to be normalized.
func (v *VecK) Power2Round(v0PlusQ, v1 *VecK) {
	for i := 0; i < K; i++ {
		v[i].Power2Round(&v0PlusQ[i], &v1[i])
	}
}

// Applies Poly.Decompose componentwise.
//
// Requires the vector to be normalized.
func (v *VecK) Decompose(v0PlusQ, v1 *VecK) {
	for i := 0; i < K; i++ {
		PolyDecompose(&v[i], &v0PlusQ[i], &v1[i])
	}
}

// Sets v to the hint vector for v0 the modified low bits and v1
// the unmodified high bits --- see makeHint().
//
// Returns the number of ones in the hint vector.
func (v *VecK) MakeHint(v0, v1 *VecK) (pop uint32) {
	for i := 0; i < K; i++ {
		pop += PolyMakeHint(&v[i], &v0[i], &v1[i])
	}
	return
}

// Computes corrections to the high bits of the polynomials in the vector
// w using the hints in h and sets v to the corrected high bits.  Returns v.
// See useHint().
func (v *VecK) UseHint(q, hint *VecK) *VecK {
	for i := 0; i < K; i++ {
		PolyUseHint(&v[i], &q[i], &hint[i])
	}
	return v
}

// Sequentially packs each polynomial using Poly.PackT1().
func (v *VecK) PackT1(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		v[i].PackT1(buf[offset:])
		offset += common.PolyT1Size
	}
}

// Sets v to the vector packed into buf by PackT1().
func (v *VecK) UnpackT1(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		v[i].UnpackT1(buf[offset:])
		offset += common.PolyT1Size
	}
}

// Sequentially packs each polynomial using Poly.PackT0().
func (v *VecK) PackT0(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		v[i].PackT0(buf[offset:])
		offset += common.PolyT0Size
	}
}

// Sets v to the vector packed into buf by PackT0().
func (v *VecK) UnpackT0(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		v[i].UnpackT0(buf[offset:])
		offset += common.PolyT0Size
	}
}

// Sequentially packs each polynomial using Poly.PackLeqEta().
func (v *VecK) PackLeqEta(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		PolyPackLeqEta(&v[i], buf[offset:])
		offset += PolyLeqEtaSize
	}
}

// Sets v to the polynomials packed in buf using VecK.PackLeqEta().
func (v *VecK) UnpackLeqEta(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		PolyUnpackLeqEta(&v[i], buf[offset:])
		offset += PolyLeqEtaSize
	}
}

// Applies NTT componentwise. See Poly.NTT() for details.
func (v *VecK) NTT() {
	for i := 0; i < K; i++ {
		v[i].NTT()
	}
}

// Sequentially packs each polynomial using PolyPackW1().
func (v *VecK) PackW1(buf []byte) {
	offset := 0
	for i := 0; i < K; i++ {
		PolyPackW1(&v[i], buf[offset:])
		offset += PolyW1Size
	}
}

// Sets v to a - b.
//
// Warning: assumes coefficients of the polynomials of  b are less than 2q.
func (v *VecK) Sub(a, b *VecK) {
	for i := 0; i < K; i++ {
		v[i].Sub(&a[i], &b[i])
	}
}

// Sets v to 2ᵈ w without reducing.
func (v *VecK) MulBy2toD(w *VecK) {
	for i := 0; i < K; i++ {
		v[i].MulBy2toD(&w[i])
	}
}

// Applies InvNTT componentwise. See Poly.InvNTT() for details.
func (v *VecK) InvNTT() {
	for i := 0; i < K; i++ {
		v[i].InvNTT()
	}
}

// Applies Poly.ReduceLe2Q() componentwise.
func (v *VecK) ReduceLe2Q() {
	for i := 0; i < K; i++ {
		v[i].ReduceLe2Q()
	}
}
