package common

// Given -2¹⁵ q ≤ x < 2¹⁵ q, returns -q < y < q with x 2⁻¹⁶ = y (mod q).
func montReduce(x int32) int16 {
	// This is Montgomery reduction with R=2¹⁶.
	//
	// Note gcd(2¹⁶, q) = 1 as q is prime.  Write q' := 62209 = q⁻¹ mod R.
	// First we compute
	//
	//	m := ((x mod R) q') mod R
	//     = x q' mod R
	//	   = int16(x q')
	//	   = int16(int32(x) * int32(q'))
	//
	// Note that x q' might be as big as 2³² and could overflow the int32
	// multiplication in the last line.  However for any int32s a and b,
	// we have int32(int64(a)*int64(b)) = int32(a*b) and so the result is ok.
	m := int16(x * 62209)

	// Note that x - m q is divisible by R; indeed modulo R we have
	//
	//  x - m q ≡ x - x q' q ≡ x - x q⁻¹ q ≡ x - x = 0.
	//
	// We return y := (x - m q) / R.  Note that y is indeed correct as
	// modulo q we have
	//
	//  y ≡ x R⁻¹ - m q R⁻¹ = x R⁻¹
	//
	// and as both 2¹⁵ q ≤ m q, x < 2¹⁵ q, we have
	// 2¹⁶ q ≤ x - m q < 2¹⁶ and so q ≤ (x - m q) / R < q as desired.
	return int16(uint32(x-int32(m)*int32(Q)) >> 16)
}

// Given any x, returns x R mod q where R=2¹⁶.
func toMont(x int16) int16 {
	// Note |1353 x| ≤ 1353 2¹⁵ ≤ 13318 q ≤ 2¹⁵ q and so we're within
	// the bounds of montReduce.
	return montReduce(int32(x) * 1353) // 1353 = R² mod q.
}

// Given any x, compute 0 ≤ y ≤ q with x = y (mod q).
//
// Beware: we might have barrettReduce(x) = q ≠ 0 for some x.  In fact,
// this happens if and only if x = -nq for some positive integer n.
func barrettReduce(x int16) int16 {
	// This is standard Barrett reduction.
	//
	// For any x we have x mod q = x - ⌊x/q⌋ q.  We will use 20159/2²⁶ as
	// an approximation of 1/q. Note that  0 ≤ 20159/2²⁶ - 1/q ≤ 0.135/2²⁶
	// and so | x 20156/2²⁶ - x/q | ≤ 2⁻¹⁰ for |x| ≤ 2¹⁶.  For all x
	// not a multiple of q, the number x/q is further than 1/q from any integer
	// and so ⌊x 20156/2²⁶⌋ = ⌊x/q⌋.  If x is a multiple of q and x is positive,
	// then x 20156/2²⁶ is larger than x/q so ⌊x 20156/2²⁶⌋ = ⌊x/q⌋ as well.
	// Finally, if x is negative multiple of q, then ⌊x 20156/2²⁶⌋ = ⌊x/q⌋-1.
	// Thus
	//                        [ q        if x=-nq for pos. integer n
	//  x - ⌊x 20156/2²⁶⌋ q = [
	//                        [ x mod q  otherwise
	//
	// To compute actually compute this, note that
	//
	//  ⌊x 20156/2²⁶⌋ = (20159 x) >> 26.
	return x - int16((int32(x)*20159)>>26)*Q
}

// Returns x if x < q and x - q otherwise.  Assumes x ≥ -29439.
func csubq(x int16) int16 {
	x -= Q // no overflow due to assumption x ≥ -29439.
	// If x is positive, then x >> 15 = 0.  If x is negative,
	// then uint16(x >> 15) = 2¹⁶-1.  So this will add back in q
	// if x was smaller than q.
	x += (x >> 15) & Q
	return x
}
