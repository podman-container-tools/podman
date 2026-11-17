package params

// We put these parameters in a separate package so that the Go code,
// such as asm/src.go, that generates assembler can import it.

const (
	// Q is the parameter q ≡ 3329 = 2¹¹ + 2¹⁰ + 2⁸ + 1.
	Q int16 = 3329

	// N is the parameter N: the length of the polynomials
	N = 256

	// PolySize is the size of a packed polynomial.
	PolySize = 384

	// PlaintextSize is the size of the plaintext
	PlaintextSize = 32

	// Eta2 is the parameter η₂
	Eta2 = 2
)
