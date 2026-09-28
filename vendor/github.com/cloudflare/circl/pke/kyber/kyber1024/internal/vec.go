// Code generated from kyber512/internal/vec.go by gen.go

package internal

import (
	"github.com/cloudflare/circl/pke/kyber/internal/common"
)

// A vector of K polynomials
type Vec [K]common.Poly

// Samples v[i] from a centered binomial distribution with given η,
// seed and nonce+i.
//
// Essentially CBD_η(PRF(seed, nonce+i)) from the specification.
func (v *Vec) DeriveNoise(seed []byte, nonce uint8, eta int) {
	for i := 0; i < K; i++ {
		v[i].DeriveNoise(seed, nonce+uint8(i), eta)
	}
}

// Sets p to the inner product of a and b using "pointwise" multiplication.
//
// See MulHat() and NTT() for a description of the multiplication.
// Assumes a and b are in Montgomery form.  p will be in Montgomery form,
// and its coefficients will be bounded in absolute value by 2kq.
// If a and b are not in Montgomery form, then the action is the same
// as "pointwise" multiplication followed by multiplying by R⁻¹, the inverse
// of the Montgomery factor.
func PolyDotHat(p *common.Poly, a, b *Vec) {
	var t common.Poly
	*p = common.Poly{} // set p to zero
	for i := 0; i < K; i++ {
		t.MulHat(&a[i], &b[i])
		p.Add(&t, p)
	}
}

// Almost normalizes coefficients in-place.
//
// Ensures each coefficient is in {0, …, q}.
func (v *Vec) BarrettReduce() {
	for i := 0; i < K; i++ {
		v[i].BarrettReduce()
	}
}

// Normalizes coefficients in-place.
//
// Ensures each coefficient is in {0, …, q-1}.
func (v *Vec) Normalize() {
	for i := 0; i < K; i++ {
		v[i].Normalize()
	}
}

// Applies in-place inverse NTT().  See Poly.InvNTT() for assumptions.
func (v *Vec) InvNTT() {
	for i := 0; i < K; i++ {
		v[i].InvNTT()
	}
}

// Applies in-place forward NTT().  See Poly.NTT() for assumptions.
func (v *Vec) NTT() {
	for i := 0; i < K; i++ {
		v[i].NTT()
	}
}

// Sets v to a + b.
func (v *Vec) Add(a, b *Vec) {
	for i := 0; i < K; i++ {
		v[i].Add(&a[i], &b[i])
	}
}

// Packs v into buf, which must be of length K*PolySize.
func (v *Vec) Pack(buf []byte) {
	for i := 0; i < K; i++ {
		v[i].Pack(buf[common.PolySize*i:])
	}
}

// Unpacks v from buf which must be of length K*PolySize.
func (v *Vec) Unpack(buf []byte) {
	for i := 0; i < K; i++ {
		v[i].Unpack(buf[common.PolySize*i:])
	}
}

// Writes Compress_q(v, d) to m.
//
// Assumes v is normalized and d is in {3, 4, 5, 10, 11}.
func (v *Vec) CompressTo(m []byte, d int) {
	size := compressedPolySize(d)
	for i := 0; i < K; i++ {
		v[i].CompressTo(m[size*i:], d)
	}
}

// Set v to Decompress_q(m, 1).
//
// Assumes d is in {3, 4, 5, 10, 11}.  v will be normalized.
func (v *Vec) Decompress(m []byte, d int) {
	size := compressedPolySize(d)
	for i := 0; i < K; i++ {
		v[i].Decompress(m[size*i:], d)
	}
}

// ⌈(256 d)/8⌉
func compressedPolySize(d int) int {
	switch d {
	case 4:
		return 128
	case 5:
		return 160
	case 10:
		return 320
	case 11:
		return 352
	}
	panic("unsupported d")
}
