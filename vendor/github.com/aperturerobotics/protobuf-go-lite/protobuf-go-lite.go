package protobuf_go_lite

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"slices"
	"unsafe"
)

var (
	// ErrInvalidLength is returned when decoding a negative length.
	ErrInvalidLength = errors.New("proto: negative length found during unmarshaling")
	// ErrIntOverflow is returned when decoding a varint representation of an integer that overflows 64 bits.
	ErrIntOverflow = errors.New("proto: integer overflow")
	// ErrUnexpectedEndOfGroup is returned when decoding a group end without a corresponding group start.
	ErrUnexpectedEndOfGroup = errors.New("proto: unexpected end of group")
)

// Message is the base vtprotobuf message marshal/unmarshal interface.
type Message interface {
	// SizeVT returns the size of the message when marshaled.
	SizeVT() int
	// MarshalToSizedBufferVT marshals to a buffer that already is SizeVT bytes long.
	MarshalToSizedBufferVT(dAtA []byte) (int, error)
	// MarshalVT marshals the message with vtprotobuf.
	MarshalVT() ([]byte, error)
	// UnmarshalVT unmarshals the message object with vtprotobuf.
	UnmarshalVT(data []byte) error
	// Reset resets the message.
	Reset()
}

// JSONMessage is a message with MarshalJSON and UnmarshalJSON.
type JSONMessage interface {
	// MarshalJSON marshals the message to JSON.
	MarshalJSON() ([]byte, error)
	// UnmarshalJSON unmarshals the message from JSON.
	UnmarshalJSON(data []byte) error
}

// CloneMessage is a message with a CloneMessage function.
type CloneMessage interface {
	// Message extends the base message type.
	Message
	// CloneMessageVT clones the object.
	CloneMessageVT() CloneMessage
}

// CloneVT is a message with a CloneVT function (VTProtobuf).
type CloneVT[T comparable] interface {
	comparable
	// CloneMessage is the non-generic clone interface.
	CloneMessage
	// CloneVT clones the object.
	CloneVT() T
}

// CloneVTSlice clones a slice of CloneVT messages.
func CloneVTSlice[S ~[]E, E CloneVT[E]](s S) S {
	out := make([]E, len(s))
	var empty E
	for i := range s {
		if s[i] != empty {
			out[i] = s[i].CloneVT()
		}
	}
	return out
}

// EqualVT is a message with a EqualVT function (VTProtobuf).
type EqualVT[T comparable] interface {
	comparable
	// EqualVT compares against the other message for equality.
	EqualVT(other T) bool
}

// CompareComparable returns a compare function to compare two comparable types.
func CompareComparable[T comparable]() func(t1, t2 T) bool {
	return func(t1, t2 T) bool {
		return t1 == t2
	}
}

// CompareEqualVT returns a compare function to compare two VTProtobuf messages.
func CompareEqualVT[T EqualVT[T]]() func(t1, t2 T) bool {
	return func(t1, t2 T) bool {
		return IsEqualVT(t1, t2)
	}
}

// IsEqualVT checks if two EqualVT objects are equal.
func IsEqualVT[T EqualVT[T]](t1, t2 T) bool {
	var empty T
	t1Empty, t2Empty := t1 == empty, t2 == empty
	if t1Empty != t2Empty {
		return false
	}
	if t1Empty {
		return true
	}
	return t1.EqualVT(t2)
}

// IsEqualVTSlice checks if two slices of EqualVT messages are equal.
func IsEqualVTSlice[S ~[]E, E EqualVT[E]](s1, s2 S) bool {
	return slices.EqualFunc(s1, s2, CompareEqualVT[E]())
}

// EncodeVarint encodes a uint64 into a varint-encoded byte slice and returns the offset of the encoded value.
// The provided offset is the offset after the last byte of the encoded value.
func EncodeVarint(dAtA []byte, offset int, v uint64) int {
	offset -= SizeOfVarint(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80) //nolint:gosec
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}

// AppendVarint appends v to b as a varint-encoded uint64.
func AppendVarint(b []byte, v uint64) []byte {
	switch {
	case v < 1<<7:
		b = append(b, byte(v))
	case v < 1<<14:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte(v>>7))
	case v < 1<<21:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte(v>>14))
	case v < 1<<28:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte(v>>21))
	case v < 1<<35:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte(v>>28))
	case v < 1<<42:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte(v>>35))
	case v < 1<<49:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte(v>>42))
	case v < 1<<56:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte(v>>49))
	case v < 1<<63:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte(v>>56))
	default:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte((v>>56)&0x7f|0x80),
			1)
	}
	return b
}

// ConsumeVarint parses b as a varint-encoded uint64, reporting its length.
// This returns -1 upon any error, -1 for parse error and -2 for overflow.
func ConsumeVarint(b []byte) (v uint64, n int) {
	var y uint64
	if len(b) <= 0 {
		return 0, -1
	}
	v = uint64(b[0])
	if v < 0x80 {
		return v, 1
	}
	v -= 0x80

	if len(b) <= 1 {
		return 0, -1
	}
	y = uint64(b[1])
	v += y << 7
	if y < 0x80 {
		return v, 2
	}
	v -= 0x80 << 7

	if len(b) <= 2 {
		return 0, -1
	}
	y = uint64(b[2])
	v += y << 14
	if y < 0x80 {
		return v, 3
	}
	v -= 0x80 << 14

	if len(b) <= 3 {
		return 0, -1
	}
	y = uint64(b[3])
	v += y << 21
	if y < 0x80 {
		return v, 4
	}
	v -= 0x80 << 21

	if len(b) <= 4 {
		return 0, -1
	}
	y = uint64(b[4])
	v += y << 28
	if y < 0x80 {
		return v, 5
	}
	v -= 0x80 << 28

	if len(b) <= 5 {
		return 0, -1
	}
	y = uint64(b[5])
	v += y << 35
	if y < 0x80 {
		return v, 6
	}
	v -= 0x80 << 35

	if len(b) <= 6 {
		return 0, -1
	}
	y = uint64(b[6])
	v += y << 42
	if y < 0x80 {
		return v, 7
	}
	v -= 0x80 << 42

	if len(b) <= 7 {
		return 0, -1
	}
	y = uint64(b[7])
	v += y << 49
	if y < 0x80 {
		return v, 8
	}
	v -= 0x80 << 49

	if len(b) <= 8 {
		return 0, -1
	}
	y = uint64(b[8])
	v += y << 56
	if y < 0x80 {
		return v, 9
	}
	v -= 0x80 << 56

	if len(b) <= 9 {
		return 0, -1
	}
	y = uint64(b[9])
	v += y << 63
	if y < 2 {
		return v, 10
	}
	return 0, -2
}

// SizeOfVarint returns the size of the varint-encoded value.
func SizeOfVarint(x uint64) (n int) {
	return (bits.Len64(x|1) + 6) / 7
}

// DecodeVarint decodes a varint at the given index, returning value, new index, and error.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeVarint(b []byte, idx int) (uint64, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return v, idx + n, nil
}

// DecodeVarintInt32 decodes a varint as int32.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeVarintInt32(b []byte, idx int) (int32, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return int32(v), idx + n, nil //nolint:gosec
}

// DecodeVarintInt64 decodes a varint as int64.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeVarintInt64(b []byte, idx int) (int64, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return int64(v), idx + n, nil //nolint:gosec
}

// DecodeVarintUint32 decodes a varint as uint32.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeVarintUint32(b []byte, idx int) (uint32, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return uint32(v), idx + n, nil //nolint:gosec
}

// DecodeVarintBool decodes a varint as bool.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeVarintBool(b []byte, idx int) (bool, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return false, 0, io.ErrUnexpectedEOF
		}
		return false, 0, ErrIntOverflow
	}
	return v != 0, idx + n, nil
}

// DecodeSint32 decodes a zigzag-encoded sint32.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeSint32(b []byte, idx int) (int32, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return int32((uint32(v) >> 1) ^ uint32((int32(v&1)<<31)>>31)), idx + n, nil //nolint:gosec
}

// DecodeSint64 decodes a zigzag-encoded sint64.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeSint64(b []byte, idx int) (int64, int, error) {
	v, n := ConsumeVarint(b[idx:])
	if n < 0 {
		if n == -1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return 0, 0, ErrIntOverflow
	}
	return int64((v >> 1) ^ uint64((int64(v&1)<<63)>>63)), idx + n, nil //nolint:gosec
}

// DecodeFixed32 decodes a fixed 32-bit value.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeFixed32(b []byte, idx int) (uint32, int, error) {
	if idx+4 > len(b) {
		return 0, 0, io.ErrUnexpectedEOF
	}
	v := uint32(b[idx]) | uint32(b[idx+1])<<8 | uint32(b[idx+2])<<16 | uint32(b[idx+3])<<24
	return v, idx + 4, nil
}

// DecodeFixed64 decodes a fixed 64-bit value.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeFixed64(b []byte, idx int) (uint64, int, error) {
	if idx+8 > len(b) {
		return 0, 0, io.ErrUnexpectedEOF
	}
	v := uint64(b[idx]) | uint64(b[idx+1])<<8 | uint64(b[idx+2])<<16 | uint64(b[idx+3])<<24 |
		uint64(b[idx+4])<<32 | uint64(b[idx+5])<<40 | uint64(b[idx+6])<<48 | uint64(b[idx+7])<<56
	return v, idx + 8, nil
}

// DecodeFloat32 decodes a 32-bit float.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeFloat32(b []byte, idx int) (float32, int, error) {
	v, idx, err := DecodeFixed32(b, idx)
	if err != nil {
		return 0, 0, err
	}
	return math.Float32frombits(v), idx, nil
}

// DecodeFloat64 decodes a 64-bit float.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeFloat64(b []byte, idx int) (float64, int, error) {
	v, idx, err := DecodeFixed64(b, idx)
	if err != nil {
		return 0, 0, err
	}
	return math.Float64frombits(v), idx, nil
}

// DecodeBytes decodes a length-prefixed byte slice. If copy is false, returns a sub-slice.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeBytes(b []byte, idx int, cp bool) ([]byte, int, error) {
	length, idx, err := DecodeVarint(b, idx)
	if err != nil {
		return nil, 0, err
	}
	l := int(length) //nolint:gosec
	if l < 0 {
		return nil, 0, ErrInvalidLength
	}
	end := idx + l
	if end < idx || end > len(b) {
		return nil, 0, io.ErrUnexpectedEOF
	}
	if cp {
		out := make([]byte, l)
		copy(out, b[idx:end])
		return out, end, nil
	}
	return b[idx:end], end, nil
}

// DecodeString decodes a length-prefixed string (with copy).
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeString(b []byte, idx int) (string, int, error) {
	length, idx, err := DecodeVarint(b, idx)
	if err != nil {
		return "", 0, err
	}
	l := int(length) //nolint:gosec
	if l < 0 {
		return "", 0, ErrInvalidLength
	}
	end := idx + l
	if end < idx || end > len(b) {
		return "", 0, io.ErrUnexpectedEOF
	}
	return string(b[idx:end]), end, nil
}

// DecodeStringUnsafe decodes a length-prefixed string without copying.
// The returned string shares memory with the input slice.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeStringUnsafe(b []byte, idx int) (string, int, error) {
	length, idx, err := DecodeVarint(b, idx)
	if err != nil {
		return "", 0, err
	}
	l := int(length) //nolint:gosec
	if l < 0 {
		return "", 0, ErrInvalidLength
	}
	end := idx + l
	if end < idx || end > len(b) {
		return "", 0, io.ErrUnexpectedEOF
	}
	if l == 0 {
		return "", end, nil
	}
	return unsafe.String(&b[idx], l), end, nil
}

// SizeOfZigzag returns the size of the zigzag-encoded value.
func SizeOfZigzag(x uint64) (n int) {
	return SizeOfVarint(uint64((x << 1) ^ uint64((int64(x) >> 63)))) //nolint
}

// Skip the first record of the byte slice and return the offset of the next record.
func Skip(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflow
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7) //nolint:gosec
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflow
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflow
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLength
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroup
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLength
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}
