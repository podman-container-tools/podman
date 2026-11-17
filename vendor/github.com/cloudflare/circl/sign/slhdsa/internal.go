package slhdsa

import "encoding/binary"

// See FIPS 205 -- Section 9
// SLH-DSA Internal Functions

// See FIPS 205 -- Section 9.1 -- Algorithm 18.
func slhKeyGenInternal(
	p *params, skSeed, skPrf, pkSeed []byte,
) (pub PublicKey, priv PrivateKey) {
	s := p.NewStatePriv(skSeed, pkSeed)
	defer s.Clear()

	stack := p.NewStack(p.hPrime)
	defer stack.Clear()

	addr := p.NewAddress()
	addr.SetLayerAddress(p.d - 1)
	pkRoot := make([]byte, p.n)
	s.xmssNodeIter(stack, pkRoot, 0, p.hPrime, addr)

	pub.ID = p.ID
	pub.seed = pkSeed
	pub.root = pkRoot

	priv.ID = p.ID
	priv.prfKey = skPrf
	priv.seed = skSeed
	priv.publicKey = pub

	return
}

func (p *params) parseMsg(
	digest []byte,
) (md []byte, idxTree [3]uint32, idxLeaf uint32) {
	l1 := (p.k*p.a + 7) / 8
	l2 := (p.h - p.h/p.d + 7) / 8
	l3 := (p.h + 8*p.d - 1) / (8 * p.d)

	c := cursor(digest)
	md = c.Next(l1)
	s2 := c.Next(l2)
	s3 := c.Next(l3)

	var b2 [12]byte
	copy(b2[12-len(s2):], s2)
	n2 := p.h - p.h/p.d
	idxTree[0] = binary.BigEndian.Uint32(b2[8:]) & ((1 << n2) - 1)
	n2 -= 32
	idxTree[1] = binary.BigEndian.Uint32(b2[4:]) & ((1 << n2) - 1)
	idxTree[2] = binary.BigEndian.Uint32(b2[0:])

	var b3 [4]byte
	copy(b3[4-len(s3):], s3)
	mask32 := (uint32(1) << (p.h / p.d)) - 1
	idxLeaf = mask32 & binary.BigEndian.Uint32(b3[0:])

	return
}

// See FIPS 205 -- Section 9.2 -- Algorithm 19.
func slhSignInternal(sk *PrivateKey, message, addRand []byte) ([]byte, error) {
	p := sk.ID.params()
	sigBytes := make([]byte, p.SignatureSize())

	var sig signature
	curSig := cursor(sigBytes)
	if !sig.fromBytes(p, &curSig) {
		return nil, ErrSigParse
	}

	p.PRFMsg(sig.rnd, sk.prfKey, addRand, message)
	digest := make([]byte, p.m)
	p.HashMsg(digest, sig.rnd, message, &sk.publicKey)

	md, idxTree, idxLeaf := p.parseMsg(digest)
	addr := p.NewAddress()
	addr.SetTreeAddress(idxTree)
	addr.SetTypeAndClear(addressForsTree)
	addr.SetKeyPairAddress(idxLeaf)

	s := p.NewStatePriv(sk.seed, sk.publicKey.seed)
	defer s.Clear()

	s.forsSign(sig.forsSig, md, addr)
	pkFors := s.forsPkFromSig(sig.forsSig, md, addr)
	s.htSign(sig.htSig, pkFors, idxTree, idxLeaf)

	return sigBytes, nil
}

// See FIPS 205 -- Section 9.3 -- Algorithm 20.
func slhVerifyInternal(pub *PublicKey, message, sigBytes []byte) bool {
	p := pub.ID.params()
	var sig signature
	curSig := cursor(sigBytes)
	if len(sigBytes) != p.SignatureSize() || !sig.fromBytes(p, &curSig) {
		return false
	}

	digest := make([]byte, p.m)
	p.HashMsg(digest, sig.rnd, message, pub)

	md, idxTree, idxLeaf := p.parseMsg(digest)
	addr := p.NewAddress()
	addr.SetTreeAddress(idxTree)
	addr.SetTypeAndClear(addressForsTree)
	addr.SetKeyPairAddress(idxLeaf)

	s := p.NewStatePub(pub.seed)
	defer s.Clear()

	pkFors := s.forsPkFromSig(sig.forsSig, md, addr)
	return s.htVerify(pkFors, pub.root, idxTree, idxLeaf, sig.htSig)
}

// signature represents a SLH-DSA signature.
type signature struct {
	rnd     []byte             // n bytes
	forsSig forsSignature      // forsSigSize() bytes
	htSig   hyperTreeSignature // hyperTreeSigSize() bytes
}

func (p *params) SignatureSize() int {
	return int(p.n + p.forsSigSize() + p.hyperTreeSigSize())
}

func (s *signature) fromBytes(p *params, c *cursor) bool {
	s.rnd = c.Next(p.n)
	s.forsSig.fromBytes(p, c)
	s.htSig.fromBytes(p, c)
	return len(*c) == 0
}
