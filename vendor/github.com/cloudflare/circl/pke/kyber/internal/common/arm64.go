//go:build arm64 && !purego
// +build arm64,!purego

package common

// Sets p to a + b.  Does not normalize coefficients.
func (p *Poly) Add(a, b *Poly) {
	polyAddARM64(p, a, b)
}

// Sets p to a - b.  Does not normalize coefficients.
func (p *Poly) Sub(a, b *Poly) {
	polySubARM64(p, a, b)
}

// Executes an in-place forward "NTT" on p.
//
// Assumes the coefficients are in absolute value ≤q.  The resulting
// coefficients are in absolute value ≤7q.  If the input is in Montgomery
// form, then the result is in Montgomery form and so (by linearity of the NTT)
// if the input is in regular form, then the result is also in regular form.
// The order of coefficients will be "tangled". These can be put back into
// their proper order by calling Detangle().
func (p *Poly) NTT() {
	p.nttGeneric()
}

// Executes an in-place inverse "NTT" on p and multiply by the Montgomery
// factor R.
//
// Requires coefficients to be in "tangled" order, see Tangle().
// Assumes the coefficients are in absolute value ≤q.  The resulting
// coefficients are in absolute value ≤q.  If the input is in Montgomery
// form, then the result is in Montgomery form and so (by linearity)
// if the input is in regular form, then the result is also in regular form.
func (p *Poly) InvNTT() {
	p.invNTTGeneric()
}

// Sets p to the "pointwise" multiplication of a and b.
//
// That is: InvNTT(p) = InvNTT(a) * InvNTT(b).  Assumes a and b are in
// Montgomery form.  Products between coefficients of a and b must be strictly
// bounded in absolute value by 2¹⁵q.  p will be in Montgomery form and
// bounded in absolute value by 2q.
//
// Requires a and b to be in "tangled" order, see Tangle().  p will be in
// tangled order as well.
func (p *Poly) MulHat(a, b *Poly) {
	p.mulHatGeneric(a, b)
}

// Puts p into the right form to be used with (among others) InvNTT().
func (p *Poly) Tangle() {
	// In the generic implementation there is no advantage to using a
	// different order, so we use the standard order everywhere.
}

// Puts p back into standard form.
func (p *Poly) Detangle() {
	// In the generic implementation there is no advantage to using a
	// different order, so we use the standard order everywhere.
}

// Almost normalizes coefficients.
//
// Ensures each coefficient is in {0, …, q}.
func (p *Poly) BarrettReduce() {
	p.barrettReduceGeneric()
}

// Normalizes coefficients.
//
// Ensures each coefficient is in {0, …, q-1}.
func (p *Poly) Normalize() {
	p.normalizeGeneric()
}

//go:noescape
func polyAddARM64(p, a, b *Poly)

//go:noescape
func polySubARM64(p, a, b *Poly)
