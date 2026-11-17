package dilithium

// Zetas lists precomputed powers of the root of unity in Montgomery
// representation used for the NTT:
//
//	Zetas[i] = zetaᵇʳᵛ⁽ⁱ⁾ R mod q,
//
// where zeta = 1753, brv(i) is the bitreversal of a 8-bit number
// and R=2³² mod q.
//
// The following Python code generates the Zetas (and InvZetas) lists:
//
//	q = 2**23 - 2**13 + 1; zeta = 1753
//	R = 2**32 % q # Montgomery const.
//	def brv(x): return int(''.join(reversed(bin(x)[2:].zfill(8))),2)
//	def inv(x): return pow(x, q-2, q) # inverse in F(q)
//	print([(pow(zeta, brv(i), q)*R)%q for i in range(256)])
//	print([(pow(inv(zeta), -(brv(255-i)-256), q)*R)%q for i in range(256)])
var Zetas = [N]uint32{
	4193792, 25847, 5771523, 7861508, 237124, 7602457, 7504169,
	466468, 1826347, 2353451, 8021166, 6288512, 3119733, 5495562,
	3111497, 2680103, 2725464, 1024112, 7300517, 3585928, 7830929,
	7260833, 2619752, 6271868, 6262231, 4520680, 6980856, 5102745,
	1757237, 8360995, 4010497, 280005, 2706023, 95776, 3077325,
	3530437, 6718724, 4788269, 5842901, 3915439, 4519302, 5336701,
	3574422, 5512770, 3539968, 8079950, 2348700, 7841118, 6681150,
	6736599, 3505694, 4558682, 3507263, 6239768, 6779997, 3699596,
	811944, 531354, 954230, 3881043, 3900724, 5823537, 2071892,
	5582638, 4450022, 6851714, 4702672, 5339162, 6927966, 3475950,
	2176455, 6795196, 7122806, 1939314, 4296819, 7380215, 5190273,
	5223087, 4747489, 126922, 3412210, 7396998, 2147896, 2715295,
	5412772, 4686924, 7969390, 5903370, 7709315, 7151892, 8357436,
	7072248, 7998430, 1349076, 1852771, 6949987, 5037034, 264944,
	508951, 3097992, 44288, 7280319, 904516, 3958618, 4656075,
	8371839, 1653064, 5130689, 2389356, 8169440, 759969, 7063561,
	189548, 4827145, 3159746, 6529015, 5971092, 8202977, 1315589,
	1341330, 1285669, 6795489, 7567685, 6940675, 5361315, 4499357,
	4751448, 3839961, 2091667, 3407706, 2316500, 3817976, 5037939,
	2244091, 5933984, 4817955, 266997, 2434439, 7144689, 3513181,
	4860065, 4621053, 7183191, 5187039, 900702, 1859098, 909542,
	819034, 495491, 6767243, 8337157, 7857917, 7725090, 5257975,
	2031748, 3207046, 4823422, 7855319, 7611795, 4784579, 342297,
	286988, 5942594, 4108315, 3437287, 5038140, 1735879, 203044,
	2842341, 2691481, 5790267, 1265009, 4055324, 1247620, 2486353,
	1595974, 4613401, 1250494, 2635921, 4832145, 5386378, 1869119,
	1903435, 7329447, 7047359, 1237275, 5062207, 6950192, 7929317,
	1312455, 3306115, 6417775, 7100756, 1917081, 5834105, 7005614,
	1500165, 777191, 2235880, 3406031, 7838005, 5548557, 6709241,
	6533464, 5796124, 4656147, 594136, 4603424, 6366809, 2432395,
	2454455, 8215696, 1957272, 3369112, 185531, 7173032, 5196991,
	162844, 1616392, 3014001, 810149, 1652634, 4686184, 6581310,
	5341501, 3523897, 3866901, 269760, 2213111, 7404533, 1717735,
	472078, 7953734, 1723600, 6577327, 1910376, 6712985, 7276084,
	8119771, 4546524, 5441381, 6144432, 7959518, 6094090, 183443,
	7403526, 1612842, 4834730, 7826001, 3919660, 8332111, 7018208,
	3937738, 1400424, 7534263, 1976782,
}

// InvZetas lists precomputed powers of the inverse root of unity in Montgomery
// representation used for the inverse NTT:
//
//	InvZetas[i] = zetaᵇʳᵛ⁽²⁵⁵⁻ⁱ⁾⁻²⁵⁶ R mod q,
//
// where zeta = 1753, brv(i) is the bitreversal of a 8-bit number
// and R=2³² mod q.
var InvZetas = [N]uint32{
	6403635, 846154, 6979993, 4442679, 1362209, 48306, 4460757,
	554416, 3545687, 6767575, 976891, 8196974, 2286327, 420899,
	2235985, 2939036, 3833893, 260646, 1104333, 1667432, 6470041,
	1803090, 6656817, 426683, 7908339, 6662682, 975884, 6167306,
	8110657, 4513516, 4856520, 3038916, 1799107, 3694233, 6727783,
	7570268, 5366416, 6764025, 8217573, 3183426, 1207385, 8194886,
	5011305, 6423145, 164721, 5925962, 5948022, 2013608, 3776993,
	7786281, 3724270, 2584293, 1846953, 1671176, 2831860, 542412,
	4974386, 6144537, 7603226, 6880252, 1374803, 2546312, 6463336,
	1279661, 1962642, 5074302, 7067962, 451100, 1430225, 3318210,
	7143142, 1333058, 1050970, 6476982, 6511298, 2994039, 3548272,
	5744496, 7129923, 3767016, 6784443, 5894064, 7132797, 4325093,
	7115408, 2590150, 5688936, 5538076, 8177373, 6644538, 3342277,
	4943130, 4272102, 2437823, 8093429, 8038120, 3595838, 768622,
	525098, 3556995, 5173371, 6348669, 3122442, 655327, 522500,
	43260, 1613174, 7884926, 7561383, 7470875, 6521319, 7479715,
	3193378, 1197226, 3759364, 3520352, 4867236, 1235728, 5945978,
	8113420, 3562462, 2446433, 6136326, 3342478, 4562441, 6063917,
	4972711, 6288750, 4540456, 3628969, 3881060, 3019102, 1439742,
	812732, 1584928, 7094748, 7039087, 7064828, 177440, 2409325,
	1851402, 5220671, 3553272, 8190869, 1316856, 7620448, 210977,
	5991061, 3249728, 6727353, 8578, 3724342, 4421799, 7475901,
	1100098, 8336129, 5282425, 7871466, 8115473, 3343383, 1430430,
	6527646, 7031341, 381987, 1308169, 22981, 1228525, 671102,
	2477047, 411027, 3693493, 2967645, 5665122, 6232521, 983419,
	4968207, 8253495, 3632928, 3157330, 3190144, 1000202, 4083598,
	6441103, 1257611, 1585221, 6203962, 4904467, 1452451, 3041255,
	3677745, 1528703, 3930395, 2797779, 6308525, 2556880, 4479693,
	4499374, 7426187, 7849063, 7568473, 4680821, 1600420, 2140649,
	4873154, 3821735, 4874723, 1643818, 1699267, 539299, 6031717,
	300467, 4840449, 2867647, 4805995, 3043716, 3861115, 4464978,
	2537516, 3592148, 1661693, 4849980, 5303092, 8284641, 5674394,
	8100412, 4369920, 19422, 6623180, 3277672, 1399561, 3859737,
	2118186, 2108549, 5760665, 1119584, 549488, 4794489, 1079900,
	7356305, 5654953, 5700314, 5268920, 2884855, 5260684, 2091905,
	359251, 6026966, 6554070, 7913949, 876248, 777960, 8143293,
	518909, 2608894, 8354570, 4186625,
}

// Execute an in-place forward NTT on as.
//
// Assumes the coefficients are in Montgomery representation and bounded
// by 2*Q.  The resulting coefficients are again in Montgomery representation,
// but are only bounded bt 18*Q.
func (p *Poly) nttGeneric() {
	// Writing z := zeta for our root of unity zeta := 1753, note z²⁵⁶=-1
	// (otherwise the order of z wouldn't be 512) and so
	//
	//  x²⁵⁶ + 1 = x²⁵⁶ - z²⁵⁶
	//           = (x¹²⁸ - z¹²⁸)(x¹²⁸ + z¹²⁸)
	//           = (x⁶⁴ - z⁶⁴)(x⁶⁴ + z⁶⁴)(x⁶⁴ + z¹⁹²)(x⁶⁴ - z¹⁹²)
	//          ...
	//           = (x-z)(x+z)(x - z¹²⁹)(x + z¹²⁹) ... (x - z²⁵⁵)(x + z²⁵⁵)
	//
	// Note that the powers of z that appear (from the second line) are
	//  in binary
	//
	//  01000000 11000000
	//  00100000 10100000 01100000 11100000
	//  00010000 10010000 01010000 11010000 00110000 10110000 01110000 11110000
	//     ...
	//
	// i.e. brv(2), brv(3), brv(4), ... and these powers of z are given by
	// the Zetas array.
	//
	// The polynomials x ± zⁱ are irreducible and coprime, hence by the
	// Chinese Remainder Theorem we know
	//
	//  R[x]/(x²⁵⁶+1) → R[x] / (x-z) x ... x R[x] / (x+z²⁵⁵)
	//                      ~= ∏_i R
	//
	// given by
	//
	//  a ↦ ( a mod x-z, ..., a mod x+z²⁵⁵ )
	//    ~ ( a(z), a(-z), a(z¹²⁹), a(-z¹²⁹), ..., a(z²⁵⁵), a(-z²⁵⁵) )
	//
	// is an isomorphism, which is the forward NTT.  It can be computed
	// efficiently by computing
	//
	//  a ↦ ( a mod x¹²⁸ - z¹²⁸, a mod x¹²⁸ + z¹²⁸ )
	//    ↦ ( a mod x⁶⁴ - z⁶⁴,  a mod x⁶⁴ + z⁶⁴,
	//        a mod x⁶⁴ - z¹⁹², a mod x⁶⁴ + z¹⁹² )
	//       et cetera
	//
	// If N was 8 then this can be pictured in the following diagram:
	//
	//  https://cnx.org/resources/17ee4dfe517a6adda05377b25a00bf6e6c93c334/File0026.png
	//
	// Each cross is a Cooley--Tukey butterfly: it's the map
	//
	//      (a, b) ↦ (a + ζ, a - ζ)
	//
	// for the appropriate ζ for that column and row group.

	k := 0 // Index into Zetas

	// l runs effectively over the columns in the diagram above; it is
	// half the height of a row group, i.e. the number of butterflies in
	// each row group.  In the diagram above it would be 4, 2, 1.
	for l := uint(N / 2); l > 0; l >>= 1 {
		// On the n-th iteration of the l-loop, the coefficients start off
		// bounded by n*2*Q.
		//
		// offset effectively loops over the row groups in this column; it
		// is the first row in the row group.
		for offset := uint(0); offset < N-l; offset += 2 * l {
			k++
			zeta := uint64(Zetas[k])

			// j loops over each butterfly in the row group.
			for j := offset; j < offset+l; j++ {
				t := montReduceLe2Q(zeta * uint64(p[j+l]))
				p[j+l] = p[j] + (2*Q - t) // Cooley--Tukey butterfly
				p[j] += t
			}
		}
	}
}

// Execute an in-place inverse NTT and multiply by Montgomery factor R
//
// Assumes the coefficients are in Montgomery representation and bounded
// by 2*Q.  The resulting coefficients are again in Montgomery representation
// and bounded by 2*Q.
func (p *Poly) invNttGeneric() {
	k := 0 // Index into InvZetas

	// We basically do the opposite of NTT, but postpone dividing by 2 in the
	// inverse of the Cooley--Tukey butterfly and accumulate that to a big
	// division by 2⁸ at the end.  See comments in the NTT() function.

	for l := uint(1); l < N; l <<= 1 {
		// On the n-th iteration of the l-loop, the coefficients start off
		// bounded by 2ⁿ⁻¹*2*Q, so by 256*Q on the last.
		for offset := uint(0); offset < N-l; offset += 2 * l {
			zeta := uint64(InvZetas[k])
			k++
			for j := offset; j < offset+l; j++ {
				t := p[j] // Gentleman--Sande butterfly
				p[j] = t + p[j+l]
				t += 256*Q - p[j+l]
				p[j+l] = montReduceLe2Q(zeta * uint64(t))
			}
		}
	}

	for j := uint(0); j < N; j++ {
		// ROver256 = 41978 = (256)⁻¹ R²
		p[j] = montReduceLe2Q(ROver256 * uint64(p[j]))
	}
}
