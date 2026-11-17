package params

// We put these parameters in a separate package so that the Go code,
// such as ntt_amd64_src.go, that generates assembler can import it.

const (
	SeedSize = 32
	N        = 256
	Q        = 8380417 // 2²³ - 2¹³ + 1
	QBits    = 23
	Qinv     = 4236238847 // = -(q^-1) mod 2³²
	ROver256 = 41978      // = (256)⁻¹ R² mod q, where R=2³²
	D        = 13

	// Size of T1 packed.  (Note that the formula is not valid in general,
	// but it is for the parameters used in the modes of Dilithium.)
	PolyT1Size = (N * (QBits - D)) / 8

	// Size of T0 packed.  (Note that the formula is not valid in general,
	// but it is for the parameters used in the modes of Dilithium.)
	PolyT0Size = (N * D) / 8

	// Size of a packed polynomial whose coefficients are in [0,16).
	PolyLe16Size = N / 2
)
