package protobuf_go_lite

import (
	"cmp"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"math/bits"
	"slices"
	"strconv"
	"strings"
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

// TextMarshaler is a message with a MarshalProtoText function.
type TextMarshaler interface {
	MarshalProtoText() string
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
	if s == nil {
		return nil
	}
	out := make([]E, len(s))
	var empty E
	for i := range s {
		if s[i] != empty {
			out[i] = s[i].CloneVT()
		}
	}
	return out
}

// ClonePtr clones one explicit scalar pointer.
func ClonePtr[T any](v *T) *T {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

// CloneBytes clones one byte slice.
func CloneBytes[S ~[]byte](v S) S {
	return slices.Clone(v)
}

// CloneSlice clones a scalar slice.
func CloneSlice[S ~[]E, E any](s S) S {
	return slices.Clone(s)
}

// CloneMap clones a map whose values do not need deep cloning.
func CloneMap[M ~map[K]V, K comparable, V any](m M) M {
	return maps.Clone(m)
}

// CloneBytesSlice clones a repeated bytes field.
func CloneBytesSlice[S ~[]E, E ~[]byte](s S) S {
	if s == nil {
		return nil
	}
	out := make(S, len(s))
	for i := range s {
		out[i] = slices.Clone(s[i])
	}
	return out
}

// CloneBytesMap clones a map whose values are bytes fields.
func CloneBytesMap[M ~map[K]V, K comparable, V ~[]byte](m M) M {
	if m == nil {
		return nil
	}
	out := make(M, len(m))
	for k, v := range m {
		out[k] = slices.Clone(v)
	}
	return out
}

// CloneVTValue clones one VT message value.
func CloneVTValue[T CloneVT[T]](v T) T {
	var empty T
	if v == empty {
		return empty
	}
	return v.CloneVT()
}

// CloneVTMap clones a map whose values are VT messages.
func CloneVTMap[M ~map[K]V, K comparable, V CloneVT[V]](m M) M {
	if m == nil {
		return nil
	}
	out := make(M, len(m))
	for k, v := range m {
		out[k] = CloneVTValue(v)
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

// EqualPtr compares two explicit scalar pointers.
func EqualPtr[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// EqualBytes compares two implicit bytes values.
func EqualBytes(a, b []byte) bool {
	return string(a) == string(b)
}

// EqualBytesPresent compares two explicit bytes values where nil and empty differ.
func EqualBytesPresent(a, b []byte) bool {
	return a == nil && b == nil || a != nil && b != nil && string(a) == string(b)
}

// EqualSlice compares repeated comparable values.
func EqualSlice[S ~[]E, E comparable](a, b S) bool {
	return slices.Equal(a, b)
}

// EqualMap compares maps with comparable values.
func EqualMap[M ~map[K]V, K comparable, V comparable](a, b M) bool {
	return maps.Equal(a, b)
}

// EqualBytesSlice compares repeated bytes values.
func EqualBytesSlice[S ~[]E, E ~[]byte](a, b S) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !EqualBytes(a[i], b[i]) {
			return false
		}
	}
	return true
}

// EqualBytesMap compares maps whose values are bytes fields.
func EqualBytesMap[M ~map[K]V, K comparable, V ~[]byte](a, b M) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !EqualBytes(av, bv) {
			return false
		}
	}
	return true
}

// EqualVTImplicit compares VT messages where nil is equivalent to an empty message.
func EqualVTImplicit[T EqualVT[T]](a, b T, empty func() T) bool {
	if a == b {
		return true
	}
	var zero T
	if a == zero {
		a = empty()
	}
	if b == zero {
		b = empty()
	}
	return a.EqualVT(b)
}

// EqualVTSliceImplicit compares repeated VT messages where nil elements are empty messages.
func EqualVTSliceImplicit[S ~[]E, E EqualVT[E]](a, b S, empty func() E) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !EqualVTImplicit(a[i], b[i], empty) {
			return false
		}
	}
	return true
}

// EqualVTMapImplicit compares map VT values where nil values are empty messages.
func EqualVTMapImplicit[M ~map[K]V, K comparable, V EqualVT[V]](a, b M, empty func() V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !EqualVTImplicit(av, bv, empty) {
			return false
		}
	}
	return true
}

// TextBuilder is the builder used by generated proto text marshalers.
type TextBuilder = strings.Builder

// TextStartMessage writes the opening message label and returns its initial length.
func TextStartMessage(sb *TextBuilder, name string) int {
	sb.WriteString(name)
	sb.WriteString(" {")
	return len(name) + 2
}

// TextFinishMessage writes the closing message delimiter and returns the final text.
func TextFinishMessage(sb *TextBuilder) string {
	sb.WriteString("}")
	return sb.String()
}

// TextWriteFieldPrefix writes the optional separator, field name, and value separator.
func TextWriteFieldPrefix(sb *TextBuilder, initialLen int, name string) {
	if sb.Len() > initialLen {
		sb.WriteString(" ")
	}
	sb.WriteString(name)
	sb.WriteString(": ")
}

// TextWriteListStart writes a repeated field prefix and opening delimiter.
func TextWriteListStart(sb *TextBuilder, initialLen int, name string) {
	TextWriteFieldPrefix(sb, initialLen, name)
	sb.WriteString("[")
}

// TextWriteListSeparator writes the separator before non-first repeated values.
func TextWriteListSeparator(sb *TextBuilder, index int) {
	if index > 0 {
		sb.WriteString(", ")
	}
}

// TextWriteListEnd writes the repeated field closing delimiter.
func TextWriteListEnd(sb *TextBuilder) {
	sb.WriteString("]")
}

// TextWriteMapStart writes a map field prefix and opening delimiter.
func TextWriteMapStart(sb *TextBuilder, initialLen int, name string) {
	TextWriteFieldPrefix(sb, initialLen, name)
	sb.WriteString("{")
}

// TextWriteMapEntryPrefix writes the separator before one sorted map entry.
func TextWriteMapEntryPrefix(sb *TextBuilder) {
	sb.WriteString(" ")
}

// TextWriteMapKeyValueSeparator writes the separator between a map key and value.
func TextWriteMapKeyValueSeparator(sb *TextBuilder) {
	sb.WriteString(": ")
}

// TextWriteMapEnd writes the map field closing delimiter.
func TextWriteMapEnd(sb *TextBuilder) {
	sb.WriteString(" }")
}

// TextSortedMapKeys returns sorted keys for deterministic generated proto text maps.
func TextSortedMapKeys[M ~map[K]V, K cmp.Ordered, V any](m M) []K {
	return slices.Sorted(maps.Keys(m))
}

// TextWriteTextMarshaler writes a nested proto text marshaler value.
func TextWriteTextMarshaler(sb *TextBuilder, v TextMarshaler) {
	sb.WriteString(v.MarshalProtoText())
}

// TextWriteString writes a quoted string proto text value.
func TextWriteString(sb *TextBuilder, v string) {
	sb.WriteString(strconv.Quote(v))
}

// TextWriteBytes writes a quoted base64 proto text bytes value.
func TextWriteBytes[B ~[]byte](sb *TextBuilder, v B) {
	sb.WriteString("\"")
	sb.WriteString(base64.StdEncoding.EncodeToString(v))
	sb.WriteString("\"")
}

// TextWriteStringer writes a quoted String value.
func TextWriteStringer[T interface{ String() string }](sb *TextBuilder, v T) {
	sb.WriteString("\"")
	sb.WriteString(v.String())
	sb.WriteString("\"")
}

// TextWriteInt writes a signed integer proto text value.
func TextWriteInt[T ~int32 | ~int64](sb *TextBuilder, v T) {
	sb.WriteString(strconv.FormatInt(int64(v), 10))
}

// TextWriteUint writes an unsigned integer proto text value.
func TextWriteUint[T ~uint32 | ~uint64](sb *TextBuilder, v T) {
	sb.WriteString(strconv.FormatUint(uint64(v), 10))
}

// TextWriteFloat32 writes a float32 proto text value.
func TextWriteFloat32(sb *TextBuilder, v float32) {
	sb.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
}

// TextWriteFloat64 writes a float64 proto text value.
func TextWriteFloat64(sb *TextBuilder, v float64) {
	sb.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
}

// TextWriteBool writes a bool proto text value.
func TextWriteBool(sb *TextBuilder, v bool) {
	sb.WriteString(strconv.FormatBool(v))
}

// EncodeVarint encodes a uint64 into a varint-encoded byte slice and returns the offset of the encoded value.
// The provided offset is the offset after the last byte of the encoded value.
func EncodeVarint(dAtA []byte, offset int, v uint64) int {
	offset -= SizeOfVarint(v)
	base := offset
	putVarintAt(dAtA, offset, v)
	return base
}

// EncodeRawBytes writes raw bytes before offset and returns the new offset.
func EncodeRawBytes[S ~[]byte](dAtA []byte, offset int, v S) int {
	offset -= len(v)
	copy(dAtA[offset:], v)
	return offset
}

// EncodeFixed32 writes a fixed-width 32-bit value before offset and returns the new offset.
func EncodeFixed32(dAtA []byte, offset int, v uint32) int {
	offset -= 4
	binary.LittleEndian.PutUint32(dAtA[offset:], v)
	return offset
}

// EncodeFixed64 writes a fixed-width 64-bit value before offset and returns the new offset.
func EncodeFixed64(dAtA []byte, offset int, v uint64) int {
	offset -= 8
	binary.LittleEndian.PutUint64(dAtA[offset:], v)
	return offset
}

// EncodeBool writes a bool value before offset and returns the new offset.
func EncodeBool(dAtA []byte, offset int, v bool) int {
	offset--
	b := byte(0)
	if v {
		b = 1
	}
	dAtA[offset] = b
	return offset
}

// EncodeString writes a length-delimited string before offset and returns the new offset.
func EncodeString[S ~string](dAtA []byte, offset int, v S) int {
	offset -= len(v)
	copy(dAtA[offset:], string(v))
	return EncodeVarint(dAtA, offset, uint64Len(len(v)))
}

// EncodeBytes writes a length-delimited byte slice before offset and returns the new offset.
func EncodeBytes[S ~[]byte](dAtA []byte, offset int, v S) int {
	offset = EncodeRawBytes(dAtA, offset, v)
	return EncodeVarint(dAtA, offset, uint64Len(len(v)))
}

// EncodeZigzag32 writes a zigzag-encoded 32-bit value before offset and returns the new offset.
func EncodeZigzag32[T ~int32](dAtA []byte, offset int, v T) int {
	return EncodeVarint(dAtA, offset, uint64((uint32(v)<<1)^uint32(v>>31)))
}

// EncodeZigzag64 writes a zigzag-encoded 64-bit value before offset and returns the new offset.
func EncodeZigzag64[T ~int64](dAtA []byte, offset int, v T) int {
	return EncodeVarint(dAtA, offset, (uint64(v)<<1)^uint64(v>>63))
}

type marshalVarintNumber interface {
	~int32 | ~int64 | ~uint32 | ~uint64
}

// EncodeVarintPacked writes packed varint values before offset and returns the new offset.
func EncodeVarintPacked[S ~[]E, E marshalVarintNumber](dAtA []byte, offset int, vals S) int {
	total := 0
	for _, v := range vals {
		total += SizeOfVarint(uint64(v))
	}
	offset -= total
	j := offset
	for _, v := range vals {
		j = putVarintAt(dAtA, j, uint64(v))
	}
	return EncodeVarint(dAtA, offset, uint64Len(total))
}

// EncodeZigzag32Packed writes packed zigzag-encoded 32-bit values before offset and returns the new offset.
func EncodeZigzag32Packed[S ~[]E, E ~int32](dAtA []byte, offset int, vals S) int {
	total := 0
	for _, v := range vals {
		total += SizeOfZigzag(uint64(v))
	}
	offset -= total
	j := offset
	for _, v := range vals {
		j = putVarintAt(dAtA, j, uint64((uint32(v)<<1)^uint32(v>>31)))
	}
	return EncodeVarint(dAtA, offset, uint64Len(total))
}

// EncodeZigzag64Packed writes packed zigzag-encoded 64-bit values before offset and returns the new offset.
func EncodeZigzag64Packed[S ~[]E, E ~int64](dAtA []byte, offset int, vals S) int {
	total := 0
	for _, v := range vals {
		total += SizeOfZigzag(uint64(v))
	}
	offset -= total
	j := offset
	for _, v := range vals {
		j = putVarintAt(dAtA, j, (uint64(v)<<1)^uint64(v>>63))
	}
	return EncodeVarint(dAtA, offset, uint64Len(total))
}

func putVarintAt(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80) //nolint:gosec
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}

func uint64Len(v int) uint64 {
	return uint64(v) //nolint:gosec
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

// DecodeLengthDelimited decodes a length-delimited payload and returns its start and end offsets.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeLengthDelimited(b []byte, idx int) (int, int, error) {
	length, idx, err := DecodeVarint(b, idx)
	if err != nil {
		return 0, 0, err
	}
	l := int(length) //nolint:gosec
	if l < 0 {
		return 0, 0, ErrInvalidLength
	}
	end := idx + l
	if end < 0 {
		return 0, 0, ErrInvalidLength
	}
	if end > len(b) {
		return 0, 0, io.ErrUnexpectedEOF
	}
	return idx, end, nil
}

// DecodeBytes decodes a length-prefixed byte slice. If copy is false, returns a sub-slice.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeBytes(b []byte, idx int, cp bool) ([]byte, int, error) {
	start, end, err := DecodeLengthDelimited(b, idx)
	if err != nil {
		return nil, 0, err
	}
	if cp {
		out := make([]byte, end-start)
		copy(out, b[start:end])
		return out, end, nil
	}
	return b[start:end], end, nil
}

// DecodeBytesAppend decodes a length-prefixed byte slice into dst and returns the new offset.
// It preserves generated singular bytes behavior by reusing dst and making empty values non-nil.
func DecodeBytesAppend(dst []byte, b []byte, idx int) ([]byte, int, error) {
	start, end, err := DecodeLengthDelimited(b, idx)
	if err != nil {
		return dst, 0, err
	}
	dst = append(dst[:0], b[start:end]...)
	if dst == nil {
		dst = []byte{}
	}
	return dst, end, nil
}

// DecodeString decodes a length-prefixed string (with copy).
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeString(b []byte, idx int) (string, int, error) {
	start, end, err := DecodeLengthDelimited(b, idx)
	if err != nil {
		return "", 0, err
	}
	return string(b[start:end]), end, nil
}

// DecodeStringUnsafe decodes a length-prefixed string without copying.
// The returned string shares memory with the input slice.
// Assumes idx is within bounds (0 <= idx <= len(b)); generated code maintains this invariant.
func DecodeStringUnsafe(b []byte, idx int) (string, int, error) {
	start, end, err := DecodeLengthDelimited(b, idx)
	if err != nil {
		return "", 0, err
	}
	if start == end {
		return "", end, nil
	}
	return unsafe.String(&b[start], end-start), end, nil
}

// PackedVarintElementCount returns the number of varint elements in a packed payload.
func PackedVarintElementCount(b []byte) (n int) {
	for _, v := range b {
		if v < 0x80 {
			n++
		}
	}
	return n
}

// PackedFixedElementCount returns the number of fixed-width elements in a packed payload.
func PackedFixedElementCount(b []byte, width int) int {
	if width <= 0 {
		return 0
	}
	return len(b) / width
}

// SizeOfZigzag returns the size of the zigzag-encoded value.
func SizeOfZigzag(x uint64) (n int) {
	return SizeOfVarint(uint64((x << 1) ^ uint64((int64(x) >> 63)))) //nolint
}

type sizeVarintNumber interface {
	~int32 | ~int64 | ~uint32 | ~uint64
}

type sizeZigzagNumber interface {
	~int32 | ~int64
}

type sizeFixed32Number interface {
	~int32 | ~uint32 | ~float32
}

type sizeFixed64Number interface {
	~int64 | ~uint64 | ~float64
}

// SizeVarintValue returns the encoded field size for one present varint value.
func SizeVarintValue[T sizeVarintNumber](keySize int, v T) int {
	return keySize + SizeOfVarint(uint64(v))
}

// SizeVarintNonZero returns the encoded field size for one implicit varint value.
func SizeVarintNonZero[T sizeVarintNumber](keySize int, v T) int {
	if v == 0 {
		return 0
	}
	return SizeVarintValue(keySize, v)
}

// SizeVarintPtr returns the encoded field size for one explicit varint value.
func SizeVarintPtr[T sizeVarintNumber](keySize int, v *T) int {
	if v == nil {
		return 0
	}
	return SizeVarintValue(keySize, *v)
}

// SizeVarintSlice returns the encoded field size for unpacked repeated varints.
func SizeVarintSlice[S ~[]E, E sizeVarintNumber](keySize int, vals S) (n int) {
	for _, v := range vals {
		n += SizeVarintValue(keySize, v)
	}
	return n
}

// SizeVarintPacked returns the encoded field size for packed repeated varints.
func SizeVarintPacked[S ~[]E, E sizeVarintNumber](keySize int, vals S) int {
	if len(vals) == 0 {
		return 0
	}
	l := 0
	for _, v := range vals {
		l += SizeOfVarint(uint64(v))
	}
	return SizeBytesValue(keySize, l)
}

// SizeZigzagValue returns the encoded field size for one present zigzag value.
func SizeZigzagValue[T sizeZigzagNumber](keySize int, v T) int {
	return keySize + SizeOfZigzag(uint64(v))
}

// SizeZigzagNonZero returns the encoded field size for one implicit zigzag value.
func SizeZigzagNonZero[T sizeZigzagNumber](keySize int, v T) int {
	if v == 0 {
		return 0
	}
	return SizeZigzagValue(keySize, v)
}

// SizeZigzagPtr returns the encoded field size for one explicit zigzag value.
func SizeZigzagPtr[T sizeZigzagNumber](keySize int, v *T) int {
	if v == nil {
		return 0
	}
	return SizeZigzagValue(keySize, *v)
}

// SizeZigzagSlice returns the encoded field size for unpacked repeated zigzags.
func SizeZigzagSlice[S ~[]E, E sizeZigzagNumber](keySize int, vals S) (n int) {
	for _, v := range vals {
		n += SizeZigzagValue(keySize, v)
	}
	return n
}

// SizeZigzagPacked returns the encoded field size for packed repeated zigzags.
func SizeZigzagPacked[S ~[]E, E sizeZigzagNumber](keySize int, vals S) int {
	if len(vals) == 0 {
		return 0
	}
	l := 0
	for _, v := range vals {
		l += SizeOfZigzag(uint64(v))
	}
	return SizeBytesValue(keySize, l)
}

// SizeFixed32Value returns the encoded field size for one present fixed32 value.
func SizeFixed32Value(keySize int) int {
	return keySize + 4
}

// SizeFixed32NonZero returns the encoded field size for one implicit fixed32 value.
func SizeFixed32NonZero[T sizeFixed32Number](keySize int, v T) int {
	if v == 0 {
		return 0
	}
	return SizeFixed32Value(keySize)
}

// SizeFixed32Ptr returns the encoded field size for one explicit fixed32 value.
func SizeFixed32Ptr[T sizeFixed32Number](keySize int, v *T) int {
	if v == nil {
		return 0
	}
	return SizeFixed32Value(keySize)
}

// SizeFixed32Slice returns the encoded field size for unpacked repeated fixed32 values.
func SizeFixed32Slice[S ~[]E, E sizeFixed32Number](keySize int, vals S) int {
	return len(vals) * SizeFixed32Value(keySize)
}

// SizeFixed32Packed returns the encoded field size for packed repeated fixed32 values.
func SizeFixed32Packed[S ~[]E, E sizeFixed32Number](keySize int, vals S) int {
	if len(vals) == 0 {
		return 0
	}
	return SizeBytesValue(keySize, len(vals)*4)
}

// SizeFixed64Value returns the encoded field size for one present fixed64 value.
func SizeFixed64Value(keySize int) int {
	return keySize + 8
}

// SizeFixed64NonZero returns the encoded field size for one implicit fixed64 value.
func SizeFixed64NonZero[T sizeFixed64Number](keySize int, v T) int {
	if v == 0 {
		return 0
	}
	return SizeFixed64Value(keySize)
}

// SizeFixed64Ptr returns the encoded field size for one explicit fixed64 value.
func SizeFixed64Ptr[T sizeFixed64Number](keySize int, v *T) int {
	if v == nil {
		return 0
	}
	return SizeFixed64Value(keySize)
}

// SizeFixed64Slice returns the encoded field size for unpacked repeated fixed64 values.
func SizeFixed64Slice[S ~[]E, E sizeFixed64Number](keySize int, vals S) int {
	return len(vals) * SizeFixed64Value(keySize)
}

// SizeFixed64Packed returns the encoded field size for packed repeated fixed64 values.
func SizeFixed64Packed[S ~[]E, E sizeFixed64Number](keySize int, vals S) int {
	if len(vals) == 0 {
		return 0
	}
	return SizeBytesValue(keySize, len(vals)*8)
}

// SizeBoolValue returns the encoded field size for one present bool value.
func SizeBoolValue(keySize int) int {
	return keySize + 1
}

// SizeBoolNonZero returns the encoded field size for one implicit bool value.
func SizeBoolNonZero(keySize int, v bool) int {
	if !v {
		return 0
	}
	return SizeBoolValue(keySize)
}

// SizeBoolPtr returns the encoded field size for one explicit bool value.
func SizeBoolPtr(keySize int, v *bool) int {
	if v == nil {
		return 0
	}
	return SizeBoolValue(keySize)
}

// SizeBoolSlice returns the encoded field size for unpacked repeated bool values.
func SizeBoolSlice(keySize int, vals []bool) int {
	return len(vals) * SizeBoolValue(keySize)
}

// SizeBoolPacked returns the encoded field size for packed repeated bool values.
func SizeBoolPacked(keySize int, vals []bool) int {
	if len(vals) == 0 {
		return 0
	}
	return SizeBytesValue(keySize, len(vals))
}

// SizeStringValue returns the encoded field size for one present string value.
func SizeStringValue(keySize int, v string) int {
	l := len(v)
	return keySize + l + SizeOfVarint(uint64Len(l))
}

// SizeStringNonEmpty returns the encoded field size for one implicit string value.
func SizeStringNonEmpty(keySize int, v string) int {
	if v == "" {
		return 0
	}
	return SizeStringValue(keySize, v)
}

// SizeStringPtr returns the encoded field size for one explicit string value.
func SizeStringPtr(keySize int, v *string) int {
	if v == nil {
		return 0
	}
	return SizeStringValue(keySize, *v)
}

// SizeStringSlice returns the encoded field size for repeated strings.
func SizeStringSlice[S ~[]E, E ~string](keySize int, vals S) (n int) {
	for _, v := range vals {
		n += SizeStringValue(keySize, string(v))
	}
	return n
}

// SizeBytesValue returns the encoded field size for one present length-delimited value.
func SizeBytesValue(keySize, l int) int {
	return keySize + l + SizeOfVarint(uint64Len(l))
}

// SizeBytesNonEmpty returns the encoded field size for one implicit bytes value.
func SizeBytesNonEmpty(keySize int, v []byte) int {
	if len(v) == 0 {
		return 0
	}
	return SizeBytesValue(keySize, len(v))
}

// SizeBytesPresent returns the encoded field size for one explicit bytes value.
func SizeBytesPresent(keySize int, v []byte) int {
	if v == nil {
		return 0
	}
	return SizeBytesValue(keySize, len(v))
}

// SizeBytesSlice returns the encoded field size for repeated bytes values.
func SizeBytesSlice[S ~[]E, E ~[]byte](keySize int, vals S) (n int) {
	for _, v := range vals {
		n += SizeBytesValue(keySize, len(v))
	}
	return n
}

// SizeMessage returns the encoded field size for one length-delimited message or map entry.
func SizeMessage(keySize, msgSize int) int {
	return SizeBytesValue(keySize, msgSize)
}

// SizeGroup returns the encoded field size for one group value.
func SizeGroup(keySize, msgSize int) int {
	return msgSize + 2*keySize
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

// SkipWithin skips one encoded field at idx and verifies it stays within limit.
func SkipWithin(dAtA []byte, idx, limit int) (int, error) {
	skippy, err := Skip(dAtA[idx:])
	if err != nil {
		return 0, err
	}
	next := idx + skippy
	if skippy < 0 || next < 0 {
		return 0, ErrInvalidLength
	}
	if next > limit {
		return 0, io.ErrUnexpectedEOF
	}
	return next, nil
}
