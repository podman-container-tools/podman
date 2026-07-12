package strongunits

// supported units

// B represents bytes.
type B uint64

// KiB represents KiB.
type KiB uint64

// MiB represents MiB.
type MiB uint64

// GiB represents GiB.
type GiB uint64

const (
	// kibToB is the math convert from bytes to KiB.
	kibToB = 1 << 10
	// mibToB is the math to convert from bytes to MiB.
	mibToB = 1 << 20
	// gibToB s the math to convert from bytes to GiB.
	gibToB = 1 << 30
)

// StorageUnits is an interface for converting disk/memory storage
// units amongst each other.
type StorageUnits interface {
	ToBytes() B
}

// ToBytes is a pass-through function for bytes.
func (b B) ToBytes() B {
	return b
}

// ToBytes converts KiB to bytes.
func (k KiB) ToBytes() B {
	return B(k * kibToB)
}

// ToBytes converts MiB to bytes.
func (m MiB) ToBytes() B {
	return B(m * mibToB)
}

// ToBytes converts GiB to bytes.
func (g GiB) ToBytes() B {
	return B(g * gibToB)
}

// ToKiB converts any StorageUnit type to KiB.
func ToKiB(b StorageUnits) KiB {
	return KiB(b.ToBytes() >> 10)
}

// ToMib converts any StorageUnit type to MiB.
func ToMib(b StorageUnits) MiB {
	return MiB(b.ToBytes() >> 20)
}

// ToGiB converts any StorageUnit type to GiB.
func ToGiB(b StorageUnits) GiB {
	return GiB(b.ToBytes() >> 30)
}
