// Code generated from mode3/internal/mat.go by gen.go

package internal

import (
	common "github.com/cloudflare/circl/sign/internal/dilithium"
)

// A k by l matrix of polynomials.
type Mat [K]VecL

// Expands the given seed to a complete matrix.
//
// This function is called ExpandA in the specification.
func (m *Mat) Derive(seed *[32]byte) {
	if !DeriveX4Available {
		for i := uint16(0); i < K; i++ {
			for j := uint16(0); j < L; j++ {
				PolyDeriveUniform(&m[i][j], seed, (i<<8)+j)
			}
		}
		return
	}

	idx := 0
	var nonces [4]uint16
	var ps [4]*common.Poly
	for i := uint16(0); i < K; i++ {
		for j := uint16(0); j < L; j++ {
			nonces[idx] = (i << 8) + j
			ps[idx] = &m[i][j]
			idx++
			if idx == 4 {
				idx = 0
				PolyDeriveUniformX4(ps, seed, nonces)
			}
		}
	}
	if idx != 0 {
		for i := idx; i < 4; i++ {
			ps[i] = nil
		}
		PolyDeriveUniformX4(ps, seed, nonces)
	}
}

// Set p to the inner product of a and b using pointwise multiplication.
//
// Assumes a and b are in Montgomery form and their coefficients are
// pairwise sufficiently small to multiply, see Poly.MulHat().  Resulting
// coefficients are bounded by 2Lq.
func PolyDotHat(p *common.Poly, a, b *VecL) {
	var t common.Poly
	*p = common.Poly{} // zero p
	for i := 0; i < L; i++ {
		t.MulHat(&a[i], &b[i])
		p.Add(&t, p)
	}
}
