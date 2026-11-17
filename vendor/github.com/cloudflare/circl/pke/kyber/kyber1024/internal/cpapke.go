// Code generated from kyber512/internal/cpapke.go by gen.go

package internal

import (
	"bytes"

	"github.com/cloudflare/circl/internal/sha3"
	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/pke/kyber/internal/common"
)

// A Kyber.CPAPKE private key.
type PrivateKey struct {
	sh Vec // NTT(s), normalized
}

// A Kyber.CPAPKE public key.
type PublicKey struct {
	rho [32]byte // ρ, the seed for the matrix A
	th  Vec      // NTT(t), normalized

	// cached values
	aT Mat // the matrix Aᵀ
}

// Packs the private key to buf.
func (sk *PrivateKey) Pack(buf []byte) {
	sk.sh.Pack(buf)
}

// Unpacks the private key from buf.
func (sk *PrivateKey) Unpack(buf []byte) {
	sk.sh.Unpack(buf)
	sk.sh.Normalize()
}

// Packs the public key to buf.
func (pk *PublicKey) Pack(buf []byte) {
	pk.th.Pack(buf)
	copy(buf[K*common.PolySize:], pk.rho[:])
}

// Unpacks the public key from buf. Checks if the public key is normalized.
func (pk *PublicKey) UnpackMLKEM(buf []byte) error {
	pk.Unpack(buf)

	// FIPS 203 §7.2 "encapsulation key check" (2).
	var buf2 [K * common.PolySize]byte
	pk.th.Pack(buf2[:])
	if !bytes.Equal(buf[:len(buf2)], buf2[:]) {
		return kem.ErrPubKey
	}
	return nil
}

// Unpacks the public key from buf.
func (pk *PublicKey) Unpack(buf []byte) {
	pk.th.Unpack(buf)
	pk.th.Normalize()
	copy(pk.rho[:], buf[K*common.PolySize:])
	pk.aT.Derive(&pk.rho, true)
}

// Derives a new Kyber.CPAPKE keypair from the given seed.
func NewKeyFromSeed(seed []byte) (*PublicKey, *PrivateKey) {
	var pk PublicKey
	var sk PrivateKey

	var expandedSeed [64]byte

	h := sha3.New512()
	_, _ = h.Write(seed)

	// This writes hash into expandedSeed.  Yes, this is idiomatic Go.
	_, _ = h.Read(expandedSeed[:])

	copy(pk.rho[:], expandedSeed[:32])
	sigma := expandedSeed[32:] // σ, the noise seed

	pk.aT.Derive(&pk.rho, false) // Expand ρ to matrix A; we'll transpose later

	var eh Vec
	sk.sh.DeriveNoise(sigma, 0, Eta1) // Sample secret vector s
	sk.sh.NTT()
	sk.sh.Normalize()

	eh.DeriveNoise(sigma, K, Eta1) // Sample blind e
	eh.NTT()

	// Next, we compute t = A s + e.
	for i := 0; i < K; i++ {
		// Note that coefficients of s are bounded by q and those of A
		// are bounded by 4.5q and so their product is bounded by 2¹⁵q
		// as required for multiplication.
		PolyDotHat(&pk.th[i], &pk.aT[i], &sk.sh)

		// A and s were not in Montgomery form, so the Montgomery
		// multiplications in the inner product added a factor R⁻¹ which
		// we'll cancel out now.  This will also ensure the coefficients of
		// t are bounded in absolute value by q.
		pk.th[i].ToMont()
	}

	pk.th.Add(&pk.th, &eh) // bounded by 8q.
	pk.th.Normalize()
	pk.aT.Transpose()

	return &pk, &sk
}

// Decrypts ciphertext ct meant for private key sk to plaintext pt.
func (sk *PrivateKey) DecryptTo(pt, ct []byte) {
	var u Vec
	var v, m common.Poly

	u.Decompress(ct, DU)
	v.Decompress(ct[K*compressedPolySize(DU):], DV)

	// Compute m = v - <s, u>
	u.NTT()
	PolyDotHat(&m, &sk.sh, &u)
	m.BarrettReduce()
	m.InvNTT()
	m.Sub(&v, &m)
	m.Normalize()

	// Compress polynomial m to original message
	m.CompressMessageTo(pt)
}

// Encrypts message pt for the public key to ciphertext ct using randomness
// from seed.
//
// seed has to be of length SeedSize, pt of PlaintextSize and ct of
// CiphertextSize.
func (pk *PublicKey) EncryptTo(ct, pt, seed []byte) {
	var rh, e1, u Vec
	var e2, v, m common.Poly

	// Sample r, e₁ and e₂ from B_η
	rh.DeriveNoise(seed, 0, Eta1)
	rh.NTT()
	rh.BarrettReduce()

	e1.DeriveNoise(seed, K, common.Eta2)
	e2.DeriveNoise(seed, 2*K, common.Eta2)

	// Next we compute u = Aᵀ r + e₁.  First Aᵀ.
	for i := 0; i < K; i++ {
		// Note that coefficients of r are bounded by q and those of Aᵀ
		// are bounded by 4.5q and so their product is bounded by 2¹⁵q
		// as required for multiplication.
		PolyDotHat(&u[i], &pk.aT[i], &rh)
	}

	u.BarrettReduce()

	// Aᵀ and r were not in Montgomery form, so the Montgomery
	// multiplications in the inner product added a factor R⁻¹ which
	// the InvNTT cancels out.
	u.InvNTT()

	u.Add(&u, &e1) // u = Aᵀ r + e₁

	// Next compute v = <t, r> + e₂ + Decompress_q(m, 1).
	PolyDotHat(&v, &pk.th, &rh)
	v.BarrettReduce()
	v.InvNTT()

	m.DecompressMessage(pt)
	v.Add(&v, &m)
	v.Add(&v, &e2) // v = <t, r> + e₂ + Decompress_q(m, 1)

	// Pack ciphertext
	u.Normalize()
	v.Normalize()

	u.CompressTo(ct, DU)
	v.CompressTo(ct[K*compressedPolySize(DU):], DV)
}

// Returns whether sk equals other.
func (sk *PrivateKey) Equal(other *PrivateKey) bool {
	ret := int16(0)
	for i := 0; i < K; i++ {
		for j := 0; j < common.N; j++ {
			ret |= sk.sh[i][j] ^ other.sh[i][j]
		}
	}
	return ret == 0
}
