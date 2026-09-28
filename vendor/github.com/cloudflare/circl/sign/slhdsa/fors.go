package slhdsa

// See FIPS 205 -- Section 8
// Forest of Random Subsets (FORS) is a few-time signature scheme that is
// used to sign the digests of the actual messages.

type (
	forsPublicKey  []byte     // n bytes
	forsPrivateKey []byte     // n bytes
	forsSignature  []forsPair // k*forsPairSize() bytes
	forsPair       struct {
		sk   forsPrivateKey // forsSkSize() bytes
		auth [][]byte       // a*n bytes
	} // forsSkSize() + a*n bytes
)

func (p *params) forsMsgSize() uint32  { return (p.k*p.a + 7) / 8 }
func (p *params) forsPkSize() uint32   { return p.n }
func (p *params) forsSkSize() uint32   { return p.n }
func (p *params) forsSigSize() uint32  { return p.k * p.forsPairSize() }
func (p *params) forsPairSize() uint32 { return p.forsSkSize() + p.a*p.n }

func (fs *forsSignature) fromBytes(p *params, c *cursor) {
	*fs = make([]forsPair, p.k)
	for i := range *fs {
		(*fs)[i].fromBytes(p, c)
	}
}

func (fp *forsPair) fromBytes(p *params, c *cursor) {
	fp.sk = c.Next(p.forsSkSize())
	fp.auth = make([][]byte, p.a)
	for i := range fp.auth {
		fp.auth[i] = c.Next(p.n)
	}
}

// See FIPS 205 -- Section 8.1 -- Algorithm 14.
func (s *statePriv) forsSkGen(addr address, idx uint32) forsPrivateKey {
	s.PRF.address.Set(addr)
	s.PRF.address.SetTypeAndClear(addressForsPrf)
	s.PRF.address.SetKeyPairAddress(addr.GetKeyPairAddress())
	s.PRF.address.SetTreeIndex(idx)

	return s.PRF.Final()
}

// See FIPS 205 -- Section 8.2 -- Algorithm 15 -- Iterative version.
//
// This is a stack-based implementation that computes the tree leaves
// in order (from the left to the right).
// Its recursive version can be found at fors_test.go file.
func (s *statePriv) forsNodeIter(
	stack stackNode, root []byte, i, z uint32, addr address,
) {
	if !(z <= s.a && i < s.k<<(s.a-z)) {
		panic(ErrTree)
	}

	s.F.address.Set(addr)
	s.F.address.SetTreeHeight(0)

	s.H.address.Set(addr)

	twoZ := uint32(1) << z
	iTwoZ := i << z
	for k := range twoZ {
		li := iTwoZ + k
		lz := uint32(0)

		sk := s.forsSkGen(addr, li)
		s.F.address.SetTreeIndex(li)
		s.F.SetMessage(sk)
		node := s.F.Final()

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

// See FIPS 205 -- Section 8.3 -- Algorithm 16.
func (s *statePriv) forsSign(sig forsSignature, digest []byte, addr address) {
	stack := s.NewStack(s.a - 1)
	defer stack.Clear()

	in, bits, total := 0, uint32(0), uint32(0)
	maskA := (uint32(1) << s.a) - 1

	for i := range s.k {
		for bits < s.a {
			total = (total << 8) + uint32(digest[in])
			in++
			bits += 8
		}

		bits -= s.a
		indicesI := (total >> bits) & maskA
		treeIdx := (i << s.a) + indicesI
		forsSk := s.forsSkGen(addr, treeIdx)
		copy(sig[i].sk, forsSk)

		for j := range s.a {
			shift := (indicesI >> j) ^ 1
			s.forsNodeIter(stack, sig[i].auth[j], (i<<(s.a-j))+shift, j, addr)
		}
	}
}

// See FIPS 205 -- Section 8.4 -- Algorithm 17.
func (s *state) forsPkFromSig(
	sig forsSignature, digest []byte, addr address,
) (pk forsPublicKey) {
	pk = make([]byte, s.forsPkSize())

	s.F.address.Set(addr)
	s.F.address.SetTreeHeight(0)

	s.H.address.Set(addr)

	s.T.address.Set(addr)
	s.T.address.SetTypeAndClear(addressForsRoots)
	s.T.address.SetKeyPairAddress(addr.GetKeyPairAddress())
	s.T.Reset()

	in, bits, total := 0, uint32(0), uint32(0)
	maskA := (uint32(1) << s.a) - 1

	for i := range s.k {
		for bits < s.a {
			total = (total << 8) + uint32(digest[in])
			in++
			bits += 8
		}

		bits -= s.a
		indicesI := (total >> bits) & maskA
		treeIdx := (i << s.a) + indicesI
		s.F.address.SetTreeIndex(treeIdx)
		s.F.SetMessage(sig[i].sk)
		node := s.F.Final()

		for j := range s.a {
			if (indicesI>>j)&0x1 == 0 {
				treeIdx = treeIdx >> 1
				s.H.SetMsgs(node, sig[i].auth[j])
			} else {
				treeIdx = (treeIdx - 1) >> 1
				s.H.SetMsgs(sig[i].auth[j], node)
			}

			s.H.address.SetTreeHeight(j + 1)
			s.H.address.SetTreeIndex(treeIdx)
			node = s.H.Final()
		}

		s.T.WriteMessage(node)
	}

	copy(pk, s.T.Final())
	return pk
}
