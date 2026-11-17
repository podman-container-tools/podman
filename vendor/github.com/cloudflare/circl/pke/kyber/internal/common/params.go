package common

import (
	"github.com/cloudflare/circl/pke/kyber/internal/common/params"
)

const (
	// Q is the parameter q ≡ 3329 = 2¹¹ + 2¹⁰ + 2⁸ + 1.
	Q = params.Q

	// N is the parameter N: the length of the polynomials
	N = params.N

	// PolySize is the size of a packed polynomial.
	PolySize = params.PolySize

	// PlaintextSize is the size of the plaintext
	PlaintextSize = params.PlaintextSize

	// Eta2 is the parameter η₂
	Eta2 = params.Eta2
)
