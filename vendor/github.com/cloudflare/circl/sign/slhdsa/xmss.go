package slhdsa

// See FIPS 205 -- Section 6
// eXtended Merkle Signature Scheme (XMSS) extends the WOTS+ signature
// scheme into one that can sign multiple messages.

type (
	xmssPublicKey []byte // n bytes
	xmssSignature struct {
		wotsSig  wotsSignature // wotsSigSize() bytes
		authPath []byte        // hPrime*n bytes
	} // wotsSigSize() + hPrime*n bytes
)

func (p *params) xmssPkSize() uint32       { return p.n }
func (p *params) xmssAuthPathSize() uint32 { return p.hPrime * p.n }
func (p *params) xmssSigSize() uint32 {
	return p.wotsSigSize() + p.xmssAuthPathSize()
}

func (xs *xmssSignature) fromBytes(p *params, c *cursor) {
	xs.wotsSig.fromBytes(p, c)
	xs.authPath = c.Next(p.xmssAuthPathSize())
}

// See FIPS 205 -- Section 6.1 -- Algorithm 9 -- Iterative version.
//
// This is a stack-based implementation that computes the tree leaves
// in order (from the left to the right).
// Its recursive version can be found at xmss_test.go file.
func (s *statePriv) xmssNodeIter(
	stack stackNode, root []byte, i, z uint32, addr address,
) {
	if !(z <= s.hPrime && i < (1<<(s.hPrime-z))) {
		panic(ErrTree)
	}

	s.H.address.Set(addr)
	s.H.address.SetTypeAndClear(addressTree)

	twoZ := uint32(1) << z
	iTwoZ := i << z
	for k := range twoZ {
		li := iTwoZ + k
		lz := uint32(0)

		addr.SetTypeAndClear(addressWotsHash)
		addr.SetKeyPairAddress(li)
		node := s.wotsPkGen(addr)

		for !stack.isEmpty() && stack.top().z == lz {
			left := stack.pop()
			li = (li - 1) >> 1
			lz = lz + 1

			s.H.address.SetTreeHeight(lz)
			s.H.address.SetTreeIndex(li)
			s.H.SetMsgs(left.node, node)
			node = s.H.Final()
		}

		stack.push(item{node, lz})
	}

	copy(root, stack.pop().node)
}

// See FIPS 205 -- Section 6.2 -- Algorithm 10.
func (s *statePriv) xmssSign(
	stack stackNode, sig xmssSignature, msg []byte, idx uint32, addr address,
) {
	authPath := cursor(sig.authPath)
	for j := range s.hPrime {
		k := (idx >> j) ^ 1
		s.xmssNodeIter(stack, authPath.Next(s.n), k, j, addr)
	}

	addr.SetTypeAndClear(addressWotsHash)
	addr.SetKeyPairAddress(idx)
	s.wotsSign(sig.wotsSig, msg, addr)
}

// See FIPS 205 -- Section 6.3 -- Algorithm 11.
func (s *state) xmssPkFromSig(
	out xmssPublicKey, msg []byte, sig xmssSignature, idx uint32, addr address,
) {
	addr.SetTypeAndClear(addressWotsHash)
	addr.SetKeyPairAddress(idx)
	pk := xmssPublicKey(s.wotsPkFromSig(sig.wotsSig, msg, addr))

	treeIdx := idx
	s.H.address.Set(addr)
	s.H.address.SetTypeAndClear(addressTree)

	authPath := cursor(sig.authPath)
	for k := range s.hPrime {
		if (idx>>k)&0x1 == 0 {
			treeIdx = treeIdx >> 1
			s.H.SetMsgs(pk, authPath.Next(s.n))
		} else {
			treeIdx = (treeIdx - 1) >> 1
			s.H.SetMsgs(authPath.Next(s.n), pk)
		}

		s.H.address.SetTreeHeight(k + 1)
		s.H.address.SetTreeIndex(treeIdx)
		pk = s.H.Final()
	}

	copy(out, pk)
}
