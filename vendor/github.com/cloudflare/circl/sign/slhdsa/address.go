package slhdsa

import "encoding/binary"

// See FIPS 205 -- Section 4.2
// Functions and Addressing

type addrType = uint32

const (
	addressWotsHash = addrType(iota)
	addressWotsPk
	addressTree
	addressForsTree
	addressForsRoots
	addressWotsPrf
	addressForsPrf
)

const (
	addressSizeCompressed    = 22
	addressSizeNonCompressed = 32
)

type address struct {
	b []byte
	o int
}

func (p *params) addressSize() uint32 {
	if p.isSHA2 {
		return addressSizeCompressed
	} else {
		return addressSizeNonCompressed
	}
}

func (p *params) addressOffset() int {
	if p.isSHA2 {
		return 0
	} else {
		return 10
	}
}

func (p *params) NewAddress() (a address) {
	var m [addressSizeNonCompressed]byte
	a.b = m[:p.addressSize()]
	a.o = p.addressOffset()
	return
}

func (a *address) fromBytes(p *params, c *cursor) {
	a.b = c.Next(p.addressSize())
	a.o = p.addressOffset()
}

func (a *address) Set(x address)              { copy(a.b, x.b); a.o = x.o }
func (a *address) Clear()                     { clearSlice(&a.b); a.o = 0 }
func (a *address) SetKeyPairAddress(i uint32) { binary.BigEndian.PutUint32(a.b[a.o+10:], i) }
func (a *address) SetChainAddress(i uint32)   { binary.BigEndian.PutUint32(a.b[a.o+14:], i) }
func (a *address) SetTreeHeight(i uint32)     { binary.BigEndian.PutUint32(a.b[a.o+14:], i) }
func (a *address) SetHashAddress(i uint32)    { binary.BigEndian.PutUint32(a.b[a.o+18:], i) }
func (a *address) SetTreeIndex(i uint32)      { binary.BigEndian.PutUint32(a.b[a.o+18:], i) }
func (a *address) GetKeyPairAddress() uint32  { return binary.BigEndian.Uint32(a.b[a.o+10:]) }
func (a *address) SetLayerAddress(l addrType) {
	if a.o == 0 {
		a.b[0] = byte(l & 0xFF)
	} else {
		binary.BigEndian.PutUint32(a.b[0:], l)
	}
}

func (a *address) SetTreeAddress(t [3]uint32) {
	if a.o == 0 {
		binary.BigEndian.PutUint32(a.b[1:], t[1])
		binary.BigEndian.PutUint32(a.b[5:], t[0])
	} else {
		binary.BigEndian.PutUint32(a.b[4:], t[2])
		binary.BigEndian.PutUint32(a.b[8:], t[1])
		binary.BigEndian.PutUint32(a.b[12:], t[0])
	}
}

func (a *address) SetTypeAndClear(t uint32) {
	if a.o == 0 {
		a.b[9] = byte(t)
	} else {
		binary.BigEndian.PutUint32(a.b[16:], t)
	}
	clear(a.b[a.o+10:])
}
