// Code generated from mode3/internal/pack.go by gen.go

package internal

import (
	common "github.com/cloudflare/circl/sign/internal/dilithium"
)

// Writes p with norm less than or equal η into buf, which must be of
// size PolyLeqEtaSize.
//
// Assumes coefficients of p are not normalized, but in [q-η,q+η].
func PolyPackLeqEta(p *common.Poly, buf []byte) {
	if DoubleEtaBits == 4 { // compiler eliminates branch
		j := 0
		for i := 0; i < PolyLeqEtaSize; i++ {
			buf[i] = (byte(common.Q+Eta-p[j]) |
				byte(common.Q+Eta-p[j+1])<<4)
			j += 2
		}
	} else if DoubleEtaBits == 3 {
		j := 0
		for i := 0; i < PolyLeqEtaSize; i += 3 {
			buf[i] = (byte(common.Q+Eta-p[j]) |
				(byte(common.Q+Eta-p[j+1]) << 3) |
				(byte(common.Q+Eta-p[j+2]) << 6))
			buf[i+1] = ((byte(common.Q+Eta-p[j+2]) >> 2) |
				(byte(common.Q+Eta-p[j+3]) << 1) |
				(byte(common.Q+Eta-p[j+4]) << 4) |
				(byte(common.Q+Eta-p[j+5]) << 7))
			buf[i+2] = ((byte(common.Q+Eta-p[j+5]) >> 1) |
				(byte(common.Q+Eta-p[j+6]) << 2) |
				(byte(common.Q+Eta-p[j+7]) << 5))
			j += 8
		}
	} else {
		panic("eta not supported")
	}
}

// Sets p to the polynomial of norm less than or equal η encoded in the
// given buffer of size PolyLeqEtaSize.
//
// Output coefficients of p are not normalized, but in [q-η,q+η] provided
// buf was created using PackLeqEta.
//
// Beware, for arbitrary buf the coefficients of p might end up in
// the interval [q-2^b,q+2^b] where b is the least b with η≤2^b.
func PolyUnpackLeqEta(p *common.Poly, buf []byte) {
	if DoubleEtaBits == 4 { // compiler eliminates branch
		j := 0
		for i := 0; i < PolyLeqEtaSize; i++ {
			p[j] = common.Q + Eta - uint32(buf[i]&15)
			p[j+1] = common.Q + Eta - uint32(buf[i]>>4)
			j += 2
		}
	} else if DoubleEtaBits == 3 {
		j := 0
		for i := 0; i < PolyLeqEtaSize; i += 3 {
			p[j] = common.Q + Eta - uint32(buf[i]&7)
			p[j+1] = common.Q + Eta - uint32((buf[i]>>3)&7)
			p[j+2] = common.Q + Eta - uint32((buf[i]>>6)|((buf[i+1]<<2)&7))
			p[j+3] = common.Q + Eta - uint32((buf[i+1]>>1)&7)
			p[j+4] = common.Q + Eta - uint32((buf[i+1]>>4)&7)
			p[j+5] = common.Q + Eta - uint32((buf[i+1]>>7)|((buf[i+2]<<1)&7))
			p[j+6] = common.Q + Eta - uint32((buf[i+2]>>2)&7)
			p[j+7] = common.Q + Eta - uint32((buf[i+2]>>5)&7)
			j += 8
		}
	} else {
		panic("eta not supported")
	}
}

// Writes v with coefficients in {0, 1} of which at most ω non-zero
// to buf, which must have length ω+k.
func (v *VecK) PackHint(buf []byte) {
	// The packed hint starts with the indices of the non-zero coefficients
	// For instance:
	//
	//    (x⁵⁶ + x¹⁰⁰, x²⁵⁵, 0, x² + x²³, x¹)
	//
	// Yields
	//
	//  56, 100, 255, 2, 23, 1
	//
	// Then we pad with zeroes until we have a list of ω items:
	// //  56, 100, 255, 2, 23, 1, 0, 0, ..., 0
	//
	// Then we finish with a list of the switch-over-indices in this
	// list between polynomials, so:
	//
	//  56, 100, 255, 2, 23, 1, 0, 0, ..., 0, 2, 3, 3, 5, 6

	off := uint8(0)
	for i := 0; i < K; i++ {
		for j := uint16(0); j < common.N; j++ {
			if v[i][j] != 0 {
				buf[off] = uint8(j)
				off++
			}
		}
		buf[Omega+i] = off
	}
	for ; off < Omega; off++ {
		buf[off] = 0
	}
}

// Sets v to the vector encoded using VecK.PackHint()
//
// Returns whether unpacking was successful.
func (v *VecK) UnpackHint(buf []byte) bool {
	// A priori, there would be several reasonable ways to encode the same
	// hint vector.  We take care to only allow only one encoding, to ensure
	// "strong unforgeability".
	//
	// See PackHint() source for description of the encoding.
	*v = VecK{}         // zero v
	prevSOP := uint8(0) // previous switch-over-point
	for i := 0; i < K; i++ {
		SOP := buf[Omega+i]
		if SOP < prevSOP || SOP > Omega {
			return false // ensures switch-over-points are increasing
		}
		for j := prevSOP; j < SOP; j++ {
			if j > prevSOP && buf[j] <= buf[j-1] {
				return false // ensures indices are increasing (within a poly)
			}
			v[i][buf[j]] = 1
		}
		prevSOP = SOP
	}
	for j := prevSOP; j < Omega; j++ {
		if buf[j] != 0 {
			return false // ensures padding indices are zero
		}
	}

	return true
}

// Sets p to the polynomial packed into buf by PolyPackLeGamma1.
//
// p will be normalized.
func PolyUnpackLeGamma1(p *common.Poly, buf []byte) {
	if Gamma1Bits == 17 {
		j := 0
		for i := 0; i < PolyLeGamma1Size; i += 9 {
			p0 := uint32(buf[i]) | (uint32(buf[i+1]) << 8) |
				(uint32(buf[i+2]&0x3) << 16)
			p1 := uint32(buf[i+2]>>2) | (uint32(buf[i+3]) << 6) |
				(uint32(buf[i+4]&0xf) << 14)
			p2 := uint32(buf[i+4]>>4) | (uint32(buf[i+5]) << 4) |
				(uint32(buf[i+6]&0x3f) << 12)
			p3 := uint32(buf[i+6]>>6) | (uint32(buf[i+7]) << 2) |
				(uint32(buf[i+8]) << 10)

			// coefficients in [0,…,2γ₁)
			p0 = Gamma1 - p0 // (-γ₁,…,γ₁]
			p1 = Gamma1 - p1
			p2 = Gamma1 - p2
			p3 = Gamma1 - p3

			p0 += uint32(int32(p0)>>31) & common.Q // normalize
			p1 += uint32(int32(p1)>>31) & common.Q
			p2 += uint32(int32(p2)>>31) & common.Q
			p3 += uint32(int32(p3)>>31) & common.Q

			p[j] = p0
			p[j+1] = p1
			p[j+2] = p2
			p[j+3] = p3

			j += 4
		}
	} else if Gamma1Bits == 19 {
		j := 0
		for i := 0; i < PolyLeGamma1Size; i += 5 {
			p0 := uint32(buf[i]) | (uint32(buf[i+1]) << 8) |
				(uint32(buf[i+2]&0xf) << 16)
			p1 := uint32(buf[i+2]>>4) | (uint32(buf[i+3]) << 4) |
				(uint32(buf[i+4]) << 12)

			p0 = Gamma1 - p0
			p1 = Gamma1 - p1

			p0 += uint32(int32(p0)>>31) & common.Q
			p1 += uint32(int32(p1)>>31) & common.Q

			p[j] = p0
			p[j+1] = p1

			j += 2
		}
	} else {
		panic("γ₁ not supported")
	}
}

// Writes p whose coefficients are in (-γ₁,γ₁] into buf
// which has to be of length PolyLeGamma1Size.
//
// Assumes p is normalized.
func PolyPackLeGamma1(p *common.Poly, buf []byte) {
	if Gamma1Bits == 17 {
		j := 0
		// coefficients in [0,…,γ₁] ∪ (q-γ₁,…,q)
		for i := 0; i < PolyLeGamma1Size; i += 9 {
			p0 := Gamma1 - p[j]                    // [0,…,γ₁] ∪ (γ₁-q,…,2γ₁-q)
			p0 += uint32(int32(p0)>>31) & common.Q // [0,…,2γ₁)
			p1 := Gamma1 - p[j+1]
			p1 += uint32(int32(p1)>>31) & common.Q
			p2 := Gamma1 - p[j+2]
			p2 += uint32(int32(p2)>>31) & common.Q
			p3 := Gamma1 - p[j+3]
			p3 += uint32(int32(p3)>>31) & common.Q

			buf[i+0] = byte(p0)
			buf[i+1] = byte(p0 >> 8)
			buf[i+2] = byte(p0>>16) | byte(p1<<2)
			buf[i+3] = byte(p1 >> 6)
			buf[i+4] = byte(p1>>14) | byte(p2<<4)
			buf[i+5] = byte(p2 >> 4)
			buf[i+6] = byte(p2>>12) | byte(p3<<6)
			buf[i+7] = byte(p3 >> 2)
			buf[i+8] = byte(p3 >> 10)

			j += 4
		}
	} else if Gamma1Bits == 19 {
		j := 0
		for i := 0; i < PolyLeGamma1Size; i += 5 {
			// Coefficients are in [0, γ₁] ∪ (Q-γ₁, Q)
			p0 := Gamma1 - p[j]
			p0 += uint32(int32(p0)>>31) & common.Q
			p1 := Gamma1 - p[j+1]
			p1 += uint32(int32(p1)>>31) & common.Q

			buf[i+0] = byte(p0)
			buf[i+1] = byte(p0 >> 8)
			buf[i+2] = byte(p0>>16) | byte(p1<<4)
			buf[i+3] = byte(p1 >> 4)
			buf[i+4] = byte(p1 >> 12)

			j += 2
		}
	} else {
		panic("γ₁ not supported")
	}
}

// Pack w₁ into buf, which must be of length PolyW1Size.
//
// Assumes w₁ is normalized.
func PolyPackW1(p *common.Poly, buf []byte) {
	if Gamma1Bits == 19 {
		p.PackLe16(buf)
	} else if Gamma1Bits == 17 {
		j := 0
		for i := 0; i < PolyW1Size; i += 3 {
			buf[i] = byte(p[j]) | byte(p[j+1]<<6)
			buf[i+1] = byte(p[j+1]>>2) | byte(p[j+2]<<4)
			buf[i+2] = byte(p[j+2]>>4) | byte(p[j+3]<<2)
			j += 4
		}
	} else {
		panic("unsupported γ₁")
	}
}
