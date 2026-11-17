//go:build arm64 && !purego
// +build arm64,!purego

package dilithium

// Execute an in-place forward NTT on as.
//
// Assumes the coefficients are in Montgomery representation and bounded
// by 2*Q.  The resulting coefficients are again in Montgomery representation,
// but are only bounded bt 18*Q.
func (p *Poly) NTT() {
	p.nttGeneric()
}

// Execute an in-place inverse NTT and multiply by Montgomery factor R
//
// Assumes the coefficients are in Montgomery representation and bounded
// by 2*Q.  The resulting coefficients are again in Montgomery representation
// and bounded by 2*Q.
func (p *Poly) InvNTT() {
	p.invNttGeneric()
}

// Sets p to the polynomial whose coefficients are the pointwise multiplication
// of those of a and b.  The coefficients of p are bounded by 2q.
//
// Assumes a and b are in Montgomery form and that the pointwise product
// of each coefficient is below 2³² q.
func (p *Poly) MulHat(a, b *Poly) {
	p.mulHatGeneric(a, b)
}

// Sets p to a + b.  Does not normalize polynomials.
func (p *Poly) Add(a, b *Poly) {
	polyAddARM64(p, a, b)
}

// Sets p to a - b.
//
// Warning: assumes coefficients of b are less than 2q.
// Sets p to a + b.  Does not normalize polynomials.
func (p *Poly) Sub(a, b *Poly) {
	polySubARM64(p, a, b)
}

// Writes p whose coefficients are in [0, 16) to buf, which must be of
// length N/2.
func (p *Poly) PackLe16(buf []byte) {
	// early bounds so we don't have to in assembly code
	// compiler may inline this func, so it may remove the bounds check
	_ = buf[PolyLe16Size-1]

	polyPackLe16ARM64(p, &buf[0])
}

// Reduces each of the coefficients to <2q.
func (p *Poly) ReduceLe2Q() {
	p.reduceLe2QGeneric()
}

// Reduce each of the coefficients to <q.
func (p *Poly) Normalize() {
	p.normalizeGeneric()
}

// Normalize the coefficients in this polynomial assuming they are already
// bounded by 2q.
func (p *Poly) NormalizeAssumingLe2Q() {
	p.normalizeAssumingLe2QGeneric()
}

// Checks whether the "supnorm" (see sec 2.1 of the spec) of p is equal
// or greater than the given bound.
//
// Requires the coefficients of p to be normalized.
func (p *Poly) Exceeds(bound uint32) bool {
	return p.exceedsGeneric(bound)
}

// Sets p to 2ᵈ q without reducing.
//
// So it requires the coefficients of p  to be less than 2³²⁻ᴰ.
func (p *Poly) MulBy2toD(q *Poly) {
	polyMulBy2toDARM64(p, q)
}

// Splits p into p1 and p0 such that [i]p1 * 2ᴰ + [i]p0 = [i]p
// with -2ᴰ⁻¹ < [i]p0 ≤ 2ᴰ⁻¹.  Returns p0 + Q and p1.
//
// Requires the coefficients of p to be normalized.
func (p *Poly) Power2Round(p0PlusQ, p1 *Poly) {
	// implementation in assembly follows
	p.power2RoundGeneric(p0PlusQ, p1)
}

//go:noescape
func polyAddARM64(p, a, b *Poly)

//go:noescape
func polyPackLe16ARM64(p *Poly, buf *byte)

//go:noescape
func polyMulBy2toDARM64(p, q *Poly)

//go:noescape
func polySubARM64(p, a, b *Poly)
