// Copyright 2018 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package p9

import (
	"encoding/binary"
)

// encoder is used for messages and 9P primitives.
type encoder interface {
	// decode decodes from the given buffer. decode may be called more than once
	// to reuse the instance. It must clear any previous state.
	//
	// This may not fail, exhaustion will be recorded in the buffer.
	decode(b *buffer)

	// encode encodes to the given buffer.
	//
	// This may not fail.
	encode(b *buffer)
}

// order is the byte order used for encoding.
var order = binary.LittleEndian

// buffer is a slice that is consumed.
//
// This is passed to the encoder methods.
type buffer struct {
	// data is the underlying data. This may grow during encode.
	data []byte

	// overflow indicates whether an overflow has occurred.
	overflow bool
}

// append appends n bytes to the buffer and returns a slice pointing to the
// newly appended bytes.
func (b *buffer) append(n int) []byte {
	b.data = append(b.data, make([]byte, n)...)
	return b.data[len(b.data)-n:]
}

// consume consumes n bytes from the buffer.
func (b *buffer) consume(n int) ([]byte, bool) {
	if !b.has(n) {
		b.markOverrun()
		return nil, false
	}
	rval := b.data[:n]
	b.data = b.data[n:]
	return rval, true
}

// has returns true if n bytes are available.
func (b *buffer) has(n int) bool {
	return len(b.data) >= n
}

// markOverrun immediately marks this buffer as overrun.
//
// This is used by ReadString, since some invalid data implies the rest of the
// buffer is no longer valid either.
func (b *buffer) markOverrun() {
	b.overflow = true
}

// isOverrun returns true if this buffer has run past the end.
func (b *buffer) isOverrun() bool {
	return b.overflow
}

// Read8 reads a byte from the buffer.
func (b *buffer) Read8() uint8 {
	v, ok := b.consume(1)
	if !ok {
		return 0
	}
	return uint8(v[0])
}

// Read16 reads a 16-bit value from the buffer.
func (b *buffer) Read16() uint16 {
	v, ok := b.consume(2)
	if !ok {
		return 0
	}
	return order.Uint16(v)
}

// Read32 reads a 32-bit value from the buffer.
func (b *buffer) Read32() uint32 {
	v, ok := b.consume(4)
	if !ok {
		return 0
	}
	return order.Uint32(v)
}

// Read64 reads a 64-bit value from the buffer.
func (b *buffer) Read64() uint64 {
	v, ok := b.consume(8)
	if !ok {
		return 0
	}
	return order.Uint64(v)
}

// ReadQIDType reads a QIDType value.
func (b *buffer) ReadQIDType() QIDType {
	return QIDType(b.Read8())
}

// ReadTag reads a Tag value.
func (b *buffer) ReadTag() tag {
	return tag(b.Read16())
}

// ReadFID reads a FID value.
func (b *buffer) ReadFID() fid {
	return fid(b.Read32())
}

// ReadUID reads a UID value.
func (b *buffer) ReadUID() UID {
	return UID(b.Read32())
}

// ReadGID reads a GID value.
func (b *buffer) ReadGID() GID {
	return GID(b.Read32())
}

// ReadPermissions reads a file mode value and applies the mask for permissions.
func (b *buffer) ReadPermissions() FileMode {
	return b.ReadFileMode() & permissionsMask
}

// ReadFileMode reads a file mode value.
func (b *buffer) ReadFileMode() FileMode {
	return FileMode(b.Read32())
}

// ReadOpenFlags reads an OpenFlags.
func (b *buffer) ReadOpenFlags() OpenFlags {
	return OpenFlags(b.Read32())
}

// ReadMsgType writes a msgType.
func (b *buffer) ReadMsgType() msgType {
	return msgType(b.Read8())
}

// ReadString deserializes a string.
func (b *buffer) ReadString() string {
	l := b.Read16()
	if !b.has(int(l)) {
		// Mark the buffer as corrupted.
		b.markOverrun()
		return ""
	}

	bs := make([]byte, l)
	for i := 0; i < int(l); i++ {
		bs[i] = byte(b.Read8())
	}
	return string(bs)
}

// Write8 writes a byte to the buffer.
func (b *buffer) Write8(v uint8) {
	b.append(1)[0] = byte(v)
}

// Write16 writes a 16-bit value to the buffer.
func (b *buffer) Write16(v uint16) {
	order.PutUint16(b.append(2), v)
}

// Write32 writes a 32-bit value to the buffer.
func (b *buffer) Write32(v uint32) {
	order.PutUint32(b.append(4), v)
}

// Write64 writes a 64-bit value to the buffer.
func (b *buffer) Write64(v uint64) {
	order.PutUint64(b.append(8), v)
}

// WriteQIDType writes a QIDType value.
func (b *buffer) WriteQIDType(qidType QIDType) {
	b.Write8(uint8(qidType))
}

// WriteTag writes a Tag value.
func (b *buffer) WriteTag(tag tag) {
	b.Write16(uint16(tag))
}

// WriteFID writes a FID value.
func (b *buffer) WriteFID(fid fid) {
	b.Write32(uint32(fid))
}

// WriteUID writes a UID value.
func (b *buffer) WriteUID(uid UID) {
	b.Write32(uint32(uid))
}

// WriteGID writes a GID value.
func (b *buffer) WriteGID(gid GID) {
	b.Write32(uint32(gid))
}

// WritePermissions applies a permissions mask and writes the FileMode.
func (b *buffer) WritePermissions(perm FileMode) {
	b.WriteFileMode(perm & permissionsMask)
}

// WriteFileMode writes a FileMode.
func (b *buffer) WriteFileMode(mode FileMode) {
	b.Write32(uint32(mode))
}

// WriteOpenFlags writes an OpenFlags.
func (b *buffer) WriteOpenFlags(flags OpenFlags) {
	b.Write32(uint32(flags))
}

// WriteMsgType writes a MsgType.
func (b *buffer) WriteMsgType(t msgType) {
	b.Write8(uint8(t))
}

// WriteString serializes the given string.
func (b *buffer) WriteString(s string) {
	b.Write16(uint16(len(s)))
	for i := 0; i < len(s); i++ {
		b.Write8(byte(s[i]))
	}
}
