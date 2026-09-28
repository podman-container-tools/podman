package dilithium

// Returns a y with y < 2q and y = x mod q.
// Note that in general *not*: ReduceLe2Q(ReduceLe2Q(x)) == x.
func ReduceLe2Q(x uint32) uint32 {
	// Note 2²³ = 2¹³ - 1 mod q. So, writing  x = x₁ 2²³ + x₂ with x₂ < 2²³
	// and x₁ < 2⁹, we have x = y (mod q) where
	// y = x₂ + x₁ 2¹³ - x₁ ≤ 2²³ + 2¹³ < 2q.
	x1 := x >> 23
	x2 := x & 0x7FFFFF // 2²³-1
	return x2 + (x1 << 13) - x1
}

// Returns x mod q.
func modQ(x uint32) uint32 {
	return le2qModQ(ReduceLe2Q(x))
}

// For x R ≤ q 2³², find y ≤ 2q with y = x mod q.
func montReduceLe2Q(x uint64) uint32 {
	// Qinv = 4236238847 = -(q⁻¹) mod 2³²
	m := (x * Qinv) & 0xffffffff
	return uint32((x + m*uint64(Q)) >> 32)
}

// Returns x mod q for 0 ≤ x < 2q.
func le2qModQ(x uint32) uint32 {
	x -= Q
	mask := uint32(int32(x) >> 31) // mask is 2³²-1 if x was neg.; 0 otherwise
	return x + (mask & Q)
}

// Splits 0 ≤ a < Q into a0 and a1 with a = a1*2ᴰ + a0
// and -2ᴰ⁻¹ < a0 < 2ᴰ⁻¹.  Returns a0 + Q and a1.
func power2round(a uint32) (a0plusQ, a1 uint32) {
	// We effectively compute a0 = a mod± 2ᵈ
	//                    and a1 = (a - a0) / 2ᵈ.
	a0 := a & ((1 << D) - 1) // a mod 2ᵈ

	// a0 is one of  0, 1, ..., 2ᵈ⁻¹-1, 2ᵈ⁻¹, 2ᵈ⁻¹+1, ..., 2ᵈ-1
	a0 -= (1 << (D - 1)) + 1
	// now a0 is     -2ᵈ⁻¹-1, -2ᵈ⁻¹, ..., -2, -1, 0, ..., 2ᵈ⁻¹-2
	// Next, we add 2ᴰ to those a0 that are negative (seen as int32).
	a0 += uint32(int32(a0)>>31) & (1 << D)
	// now a0 is     2ᵈ⁻¹-1, 2ᵈ⁻¹, ..., 2ᵈ-2, 2ᵈ-1, 0, ..., 2ᵈ⁻¹-2
	a0 -= (1 << (D - 1)) - 1
	// now a0 id     0, 1, 2, ..., 2ᵈ⁻¹-1, 2ᵈ⁻¹-1, -2ᵈ⁻¹-1, ...
	// which is what we want.
	a0plusQ = Q + a0
	a1 = (a - a0) >> D
	return
}
