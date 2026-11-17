package slhdsa

import (
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"hash"
	"io"
	"strings"

	"github.com/cloudflare/circl/internal/sha3"
)

// [ID] identifies the supported parameter sets of SLH-DSA.
// Note that the zero value is not a valid identifier.
type ID byte

//nolint:stylecheck
const (
	SHA2_128s  ID = iota + 1 // SLH-DSA-SHA2-128s
	SHAKE_128s               // SLH-DSA-SHAKE-128s
	SHA2_128f                // SLH-DSA-SHA2-128f
	SHAKE_128f               // SLH-DSA-SHAKE-128f
	SHA2_192s                // SLH-DSA-SHA2-192s
	SHAKE_192s               // SLH-DSA-SHAKE-192s
	SHA2_192f                // SLH-DSA-SHA2-192f
	SHAKE_192f               // SLH-DSA-SHAKE-192f
	SHA2_256s                // SLH-DSA-SHA2-256s
	SHAKE_256s               // SLH-DSA-SHAKE-256s
	SHA2_256f                // SLH-DSA-SHA2-256f
	SHAKE_256f               // SLH-DSA-SHAKE-256f
	_MaxParams
)

// [IDByName] returns the [ID] that corresponds to the given name,
// or an error if no parameter set was found.
// See [ID] documentation for the specific names of each parameter set.
// Names are case insensitive.
//
// Example:
//
//	IDByName("SLH-DSA-SHAKE-256s") // returns (SHAKESmall256, nil)
func IDByName(name string) (ID, error) {
	v := strings.ToLower(name)
	for i := range supportedParams {
		if strings.ToLower(supportedParams[i].name) == v {
			return supportedParams[i].ID, nil
		}
	}

	return ID(0), ErrParam
}

// IsValid returns true if the parameter set is supported.
func (id ID) IsValid() bool { return 0 < id && id < _MaxParams }

func (id ID) String() string {
	if !id.IsValid() {
		return ErrParam.Error()
	}
	return supportedParams[id-1].name
}

func (id ID) params() *params {
	if !id.IsValid() {
		panic(ErrParam)
	}
	return &supportedParams[id-1]
}

// params contains all the relevant constants of a parameter set.
type params struct {
	name   string // Name of the parameter set.
	n      uint32 // Length of WOTS+ messages.
	hPrime uint32 // XMSS Merkle tree height.
	h      uint32 // Total height of a hypertree.
	d      uint32 // Hypertree has d layers of XMSS trees.
	a      uint32 // FORS signs a-bit messages.
	k      uint32 // FORS generates k private keys.
	m      uint32 // Used by HashMSG function.
	isSHA2 bool   // True, if the hash function is SHA2, otherwise is SHAKE.
	ID            // Identifier of the parameter set.
}

// Stores all the supported (read-only) parameter sets.
var supportedParams = [_MaxParams - 1]params{
	{ID: SHA2_128s, n: 16, h: 63, d: 7, hPrime: 9, a: 12, k: 14, m: 30, isSHA2: true, name: "SLH-DSA-SHA2-128s"},
	{ID: SHAKE_128s, n: 16, h: 63, d: 7, hPrime: 9, a: 12, k: 14, m: 30, isSHA2: false, name: "SLH-DSA-SHAKE-128s"},
	{ID: SHA2_128f, n: 16, h: 66, d: 22, hPrime: 3, a: 6, k: 33, m: 34, isSHA2: true, name: "SLH-DSA-SHA2-128f"},
	{ID: SHAKE_128f, n: 16, h: 66, d: 22, hPrime: 3, a: 6, k: 33, m: 34, isSHA2: false, name: "SLH-DSA-SHAKE-128f"},
	{ID: SHA2_192s, n: 24, h: 63, d: 7, hPrime: 9, a: 14, k: 17, m: 39, isSHA2: true, name: "SLH-DSA-SHA2-192s"},
	{ID: SHAKE_192s, n: 24, h: 63, d: 7, hPrime: 9, a: 14, k: 17, m: 39, isSHA2: false, name: "SLH-DSA-SHAKE-192s"},
	{ID: SHA2_192f, n: 24, h: 66, d: 22, hPrime: 3, a: 8, k: 33, m: 42, isSHA2: true, name: "SLH-DSA-SHA2-192f"},
	{ID: SHAKE_192f, n: 24, h: 66, d: 22, hPrime: 3, a: 8, k: 33, m: 42, isSHA2: false, name: "SLH-DSA-SHAKE-192f"},
	{ID: SHA2_256s, n: 32, h: 64, d: 8, hPrime: 8, a: 14, k: 22, m: 47, isSHA2: true, name: "SLH-DSA-SHA2-256s"},
	{ID: SHAKE_256s, n: 32, h: 64, d: 8, hPrime: 8, a: 14, k: 22, m: 47, isSHA2: false, name: "SLH-DSA-SHAKE-256s"},
	{ID: SHA2_256f, n: 32, h: 68, d: 17, hPrime: 4, a: 9, k: 35, m: 49, isSHA2: true, name: "SLH-DSA-SHA2-256f"},
	{ID: SHAKE_256f, n: 32, h: 68, d: 17, hPrime: 4, a: 9, k: 35, m: 49, isSHA2: false, name: "SLH-DSA-SHAKE-256f"},
}

// See FIPS-205, Section 11.1 and Section 11.2.
func (p *params) PRFMsg(out, skPrf, optRand, msg []byte) {
	if p.isSHA2 {
		var h crypto.Hash
		if p.n == 16 {
			h = crypto.SHA256
		} else {
			h = crypto.SHA512
		}

		mac := hmac.New(h.New, skPrf)
		concat(mac, optRand, msg)
		mac.Sum(out[:0])
	} else {
		state := sha3.NewShake256()
		concat(&state, skPrf, optRand, msg)
		_, _ = state.Read(out)
	}
}

// See FIPS-205, Section 11.1 and Section 11.2.
func (p *params) HashMsg(out, r, msg []byte, pk *PublicKey) {
	if p.isSHA2 {
		var hLen uint32
		var state hash.Hash
		if p.n == 16 {
			hLen = sha256.Size
			state = sha256.New()
		} else {
			hLen = sha512.Size
			state = sha512.New()
		}

		mgfSeed := make([]byte, 2*p.n+hLen+4)
		c := cursor(mgfSeed)
		copy(c.Next(p.n), r)
		copy(c.Next(p.n), pk.seed)
		sumInter := c.Next(hLen)

		concat(state, r, pk.seed, pk.root, msg)
		state.Sum(sumInter[:0])
		p.mgf1(out, mgfSeed, p.m)
	} else {
		state := sha3.NewShake256()
		concat(&state, r, pk.seed, pk.root, msg)
		_, _ = state.Read(out)
	}
}

// MGF1 described in Appendix B.2.1 of RFC 8017.
func (p *params) mgf1(out, mgfSeed []byte, maskLen uint32) {
	var hLen uint32
	var hashFn func(out, in []byte)
	if p.n == 16 {
		hLen = sha256.Size
		hashFn = sha256sum
	} else {
		hLen = sha512.Size
		hashFn = sha512sum
	}

	offset := uint32(0)
	end := (maskLen + hLen - 1) / hLen
	counterBytes := mgfSeed[len(mgfSeed)-4:]

	for counter := range end {
		binary.BigEndian.PutUint32(counterBytes, counter)
		hashFn(out[offset:], mgfSeed)
		offset += hLen
	}
}

func concat(w io.Writer, list ...[]byte) {
	for _, li := range list {
		_, err := w.Write(li)
		if err != nil {
			panic(ErrWriting)
		}
	}
}
