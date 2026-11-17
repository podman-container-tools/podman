package slhdsa

import "bytes"

// See FIPS 205 -- Section 7
// SLH-DSA uses a hypertree to sign the FORS keys.

type hyperTreeSignature []xmssSignature // d*xmssSigSize() bytes

func (p *params) hyperTreeSigSize() uint32 { return p.d * p.xmssSigSize() }

func (hts *hyperTreeSignature) fromBytes(p *params, c *cursor) {
	*hts = make([]xmssSignature, p.d)
	for i := range *hts {
		(*hts)[i].fromBytes(p, c)
	}
}

func nextIndex(idxTree *[3]uint32, n uint32) (idxLeaf uint32) {
	idxLeaf = idxTree[0] & ((1 << n) - 1)
	idxTree[0] = (idxTree[0] >> n) | (idxTree[1] << (32 - n))
	idxTree[1] = (idxTree[1] >> n) | (idxTree[2] << (32 - n))
	idxTree[2] = (idxTree[2] >> n)

	return
}

// See FIPS 205 -- Section 7.1 -- Algorithm 12.
func (s *statePriv) htSign(
	sig hyperTreeSignature, msg []byte, idxTree [3]uint32, idxLeaf uint32,
) {
	addr := s.NewAddress()
	addr.SetTreeAddress(idxTree)
	stack := s.NewStack(s.hPrime - 1)
	defer stack.Clear()

	s.xmssSign(stack, sig[0], msg, idxLeaf, addr)

	root := make([]byte, s.xmssPkSize())
	copy(root, msg)
	for j := uint32(1); j < s.d; j++ {
		s.xmssPkFromSig(root, root, sig[j-1], idxLeaf, addr)
		idxLeaf = nextIndex(&idxTree, s.hPrime)
		addr.SetLayerAddress(j)
		addr.SetTreeAddress(idxTree)
		s.xmssSign(stack, sig[j], root, idxLeaf, addr)
	}
}

// See FIPS 205 -- Section 7.2 -- Algorithm 13.
func (s *state) htVerify(
	msg, root []byte, idxTree [3]uint32, idxLeaf uint32, sig hyperTreeSignature,
) bool {
	addr := s.NewAddress()
	addr.SetTreeAddress(idxTree)

	node := make([]byte, s.xmssPkSize())
	s.xmssPkFromSig(node, msg, sig[0], idxLeaf, addr)

	for j := uint32(1); j < s.d; j++ {
		idxLeaf = nextIndex(&idxTree, s.hPrime)
		addr.SetLayerAddress(j)
		addr.SetTreeAddress(idxTree)
		s.xmssPkFromSig(node, node, sig[j], idxLeaf, addr)
	}

	return bytes.Equal(node, root)
}
