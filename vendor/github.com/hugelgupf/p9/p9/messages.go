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
	"fmt"
	"math"
)

// ErrInvalidMsgType is returned when an unsupported message type is found.
type ErrInvalidMsgType struct {
	msgType
}

// Error returns a useful string.
func (e *ErrInvalidMsgType) Error() string {
	return fmt.Sprintf("invalid message type: %d", e.msgType)
}

// message is a generic 9P message.
type message interface {
	encoder
	fmt.Stringer

	// Type returns the message type number.
	typ() msgType
}

// payloader is a special message which may include an inline payload.
type payloader interface {
	// FixedSize returns the size of the fixed portion of this message.
	FixedSize() uint32

	// Payload returns the payload for sending.
	Payload() []byte

	// SetPayload returns the decoded message.
	//
	// This is going to be total message size - FixedSize. But this should
	// be validated during decode, which will be called after SetPayload.
	SetPayload([]byte)

	// PayloadCleanup is called after a payloader message is sent and
	// buffers can be reapt.
	PayloadCleanup()
}

// tversion is a version request.
type tversion struct {
	// MSize is the message size to use.
	MSize uint32

	// Version is the version string.
	//
	// For this implementation, this must be 9P2000.L.
	Version string
}

// decode implements encoder.decode.
func (t *tversion) decode(b *buffer) {
	t.MSize = b.Read32()
	t.Version = b.ReadString()
}

// encode implements encoder.encode.
func (t *tversion) encode(b *buffer) {
	b.Write32(t.MSize)
	b.WriteString(t.Version)
}

// typ implements message.typ.
func (*tversion) typ() msgType {
	return msgTversion
}

// String implements fmt.Stringer.
func (t *tversion) String() string {
	return fmt.Sprintf("Tversion{MSize: %d, Version: %s}", t.MSize, t.Version)
}

// rversion is a version response.
type rversion struct {
	// MSize is the negotiated size.
	MSize uint32

	// Version is the negotiated version.
	Version string
}

// decode implements encoder.decode.
func (r *rversion) decode(b *buffer) {
	r.MSize = b.Read32()
	r.Version = b.ReadString()
}

// encode implements encoder.encode.
func (r *rversion) encode(b *buffer) {
	b.Write32(r.MSize)
	b.WriteString(r.Version)
}

// typ implements message.typ.
func (*rversion) typ() msgType {
	return msgRversion
}

// String implements fmt.Stringer.
func (r *rversion) String() string {
	return fmt.Sprintf("Rversion{MSize: %d, Version: %s}", r.MSize, r.Version)
}

// tflush is a flush request.
type tflush struct {
	// OldTag is the tag to wait on.
	OldTag tag
}

// decode implements encoder.decode.
func (t *tflush) decode(b *buffer) {
	t.OldTag = b.ReadTag()
}

// encode implements encoder.encode.
func (t *tflush) encode(b *buffer) {
	b.WriteTag(t.OldTag)
}

// typ implements message.typ.
func (*tflush) typ() msgType {
	return msgTflush
}

// String implements fmt.Stringer.
func (t *tflush) String() string {
	return fmt.Sprintf("Tflush{OldTag: %d}", t.OldTag)
}

// rflush is a flush response.
type rflush struct {
}

// decode implements encoder.decode.
func (*rflush) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rflush) encode(b *buffer) {
}

// typ implements message.typ.
func (*rflush) typ() msgType {
	return msgRflush
}

// String implements fmt.Stringer.
func (r *rflush) String() string {
	return fmt.Sprintf("Rflush{}")
}

// twalk is a walk request.
type twalk struct {
	// fid is the fid to be walked.
	fid fid

	// newFID is the resulting fid.
	newFID fid

	// Names are the set of names to be walked.
	Names []string
}

// decode implements encoder.decode.
func (t *twalk) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.newFID = b.ReadFID()
	n := b.Read16()
	t.Names = t.Names[:0]
	for i := 0; i < int(n); i++ {
		t.Names = append(t.Names, b.ReadString())
	}
}

// encode implements encoder.encode.
func (t *twalk) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteFID(t.newFID)
	b.Write16(uint16(len(t.Names)))
	for _, name := range t.Names {
		b.WriteString(name)
	}
}

// typ implements message.typ.
func (*twalk) typ() msgType {
	return msgTwalk
}

// String implements fmt.Stringer.
func (t *twalk) String() string {
	return fmt.Sprintf("Twalk{FID: %d, newFID: %d, Names: %v}", t.fid, t.newFID, t.Names)
}

// rwalk is a walk response.
type rwalk struct {
	// QIDs are the set of QIDs returned.
	QIDs []QID
}

// decode implements encoder.decode.
func (r *rwalk) decode(b *buffer) {
	n := b.Read16()
	r.QIDs = r.QIDs[:0]
	for i := 0; i < int(n); i++ {
		var q QID
		q.decode(b)
		r.QIDs = append(r.QIDs, q)
	}
}

// encode implements encoder.encode.
func (r *rwalk) encode(b *buffer) {
	b.Write16(uint16(len(r.QIDs)))
	for _, q := range r.QIDs {
		q.encode(b)
	}
}

// typ implements message.typ.
func (*rwalk) typ() msgType {
	return msgRwalk
}

// String implements fmt.Stringer.
func (r *rwalk) String() string {
	return fmt.Sprintf("Rwalk{QIDs: %v}", r.QIDs)
}

// tclunk is a close request.
type tclunk struct {
	// fid is the fid to be closed.
	fid fid
}

// decode implements encoder.decode.
func (t *tclunk) decode(b *buffer) {
	t.fid = b.ReadFID()
}

// encode implements encoder.encode.
func (t *tclunk) encode(b *buffer) {
	b.WriteFID(t.fid)
}

// typ implements message.typ.
func (*tclunk) typ() msgType {
	return msgTclunk
}

// String implements fmt.Stringer.
func (t *tclunk) String() string {
	return fmt.Sprintf("Tclunk{FID: %d}", t.fid)
}

// rclunk is a close response.
type rclunk struct{}

// decode implements encoder.decode.
func (*rclunk) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rclunk) encode(b *buffer) {
}

// typ implements message.typ.
func (*rclunk) typ() msgType {
	return msgRclunk
}

// String implements fmt.Stringer.
func (r *rclunk) String() string {
	return fmt.Sprintf("Rclunk{}")
}

// tremove is a remove request.
type tremove struct {
	// fid is the fid to be removed.
	fid fid
}

// decode implements encoder.decode.
func (t *tremove) decode(b *buffer) {
	t.fid = b.ReadFID()
}

// encode implements encoder.encode.
func (t *tremove) encode(b *buffer) {
	b.WriteFID(t.fid)
}

// typ implements message.typ.
func (*tremove) typ() msgType {
	return msgTremove
}

// String implements fmt.Stringer.
func (t *tremove) String() string {
	return fmt.Sprintf("Tremove{FID: %d}", t.fid)
}

// rremove is a remove response.
type rremove struct {
}

// decode implements encoder.decode.
func (*rremove) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rremove) encode(b *buffer) {
}

// typ implements message.typ.
func (*rremove) typ() msgType {
	return msgRremove
}

// String implements fmt.Stringer.
func (r *rremove) String() string {
	return fmt.Sprintf("Rremove{}")
}

// rlerror is an error response.
//
// Note that this replaces the error code used in 9p.
type rlerror struct {
	Error uint32
}

// decode implements encoder.decode.
func (r *rlerror) decode(b *buffer) {
	r.Error = b.Read32()
}

// encode implements encoder.encode.
func (r *rlerror) encode(b *buffer) {
	b.Write32(r.Error)
}

// typ implements message.typ.
func (*rlerror) typ() msgType {
	return msgRlerror
}

// String implements fmt.Stringer.
func (r *rlerror) String() string {
	return fmt.Sprintf("Rlerror{Error: %d}", r.Error)
}

// tauth is an authentication request.
type tauth struct {
	// Authenticationfid is the fid to attach the authentication result.
	Authenticationfid fid

	// UserName is the user to attach.
	UserName string

	// AttachName is the attach name.
	AttachName string

	// UserID is the numeric identifier for UserName.
	UID UID
}

// decode implements encoder.decode.
func (t *tauth) decode(b *buffer) {
	t.Authenticationfid = b.ReadFID()
	t.UserName = b.ReadString()
	t.AttachName = b.ReadString()
	t.UID = b.ReadUID()
}

// encode implements encoder.encode.
func (t *tauth) encode(b *buffer) {
	b.WriteFID(t.Authenticationfid)
	b.WriteString(t.UserName)
	b.WriteString(t.AttachName)
	b.WriteUID(t.UID)
}

// typ implements message.typ.
func (*tauth) typ() msgType {
	return msgTauth
}

// String implements fmt.Stringer.
func (t *tauth) String() string {
	return fmt.Sprintf("Tauth{AuthFID: %d, UserName: %s, AttachName: %s, UID: %d", t.Authenticationfid, t.UserName, t.AttachName, t.UID)
}

// rauth is an authentication response.
//
// encode, decode and Length are inherited directly from QID.
type rauth struct {
	QID
}

// typ implements message.typ.
func (*rauth) typ() msgType {
	return msgRauth
}

// String implements fmt.Stringer.
func (r *rauth) String() string {
	return fmt.Sprintf("Rauth{QID: %s}", r.QID)
}

// tattach is an attach request.
type tattach struct {
	// fid is the fid to be attached.
	fid fid

	// Auth is the embedded authentication request.
	//
	// See client.Attach for information regarding authentication.
	Auth tauth
}

// decode implements encoder.decode.
func (t *tattach) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Auth.decode(b)
}

// encode implements encoder.encode.
func (t *tattach) encode(b *buffer) {
	b.WriteFID(t.fid)
	t.Auth.encode(b)
}

// typ implements message.typ.
func (*tattach) typ() msgType {
	return msgTattach
}

// String implements fmt.Stringer.
func (t *tattach) String() string {
	return fmt.Sprintf("Tattach{FID: %d, AuthFID: %d, UserName: %s, AttachName: %s, UID: %d}", t.fid, t.Auth.Authenticationfid, t.Auth.UserName, t.Auth.AttachName, t.Auth.UID)
}

// rattach is an attach response.
type rattach struct {
	QID
}

// typ implements message.typ.
func (*rattach) typ() msgType {
	return msgRattach
}

// String implements fmt.Stringer.
func (r *rattach) String() string {
	return fmt.Sprintf("Rattach{QID: %s}", r.QID)
}

// tlopen is an open request.
type tlopen struct {
	// fid is the fid to be opened.
	fid fid

	// Flags are the open flags.
	Flags OpenFlags
}

// decode implements encoder.decode.
func (t *tlopen) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Flags = b.ReadOpenFlags()
}

// encode implements encoder.encode.
func (t *tlopen) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteOpenFlags(t.Flags)
}

// typ implements message.typ.
func (*tlopen) typ() msgType {
	return msgTlopen
}

// String implements fmt.Stringer.
func (t *tlopen) String() string {
	return fmt.Sprintf("Tlopen{FID: %d, Flags: %v}", t.fid, t.Flags)
}

// rlopen is a open response.
type rlopen struct {
	// QID is the file's QID.
	QID QID

	// IoUnit is the recommended I/O unit.
	IoUnit uint32
}

// decode implements encoder.decode.
func (r *rlopen) decode(b *buffer) {
	r.QID.decode(b)
	r.IoUnit = b.Read32()
}

// encode implements encoder.encode.
func (r *rlopen) encode(b *buffer) {
	r.QID.encode(b)
	b.Write32(r.IoUnit)
}

// typ implements message.typ.
func (*rlopen) typ() msgType {
	return msgRlopen
}

// String implements fmt.Stringer.
func (r *rlopen) String() string {
	return fmt.Sprintf("Rlopen{QID: %s, IoUnit: %d}", r.QID, r.IoUnit)
}

// tlcreate is a create request.
type tlcreate struct {
	// fid is the parent fid.
	//
	// This becomes the new file.
	fid fid

	// Name is the file name to create.
	Name string

	// Mode is the open mode (O_RDWR, etc.).
	//
	// Note that flags like O_TRUNC are ignored, as is O_EXCL. All
	// create operations are exclusive.
	OpenFlags OpenFlags

	// Permissions is the set of permission bits.
	Permissions FileMode

	// GID is the group ID to use for creating the file.
	GID GID
}

// decode implements encoder.decode.
func (t *tlcreate) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Name = b.ReadString()
	t.OpenFlags = b.ReadOpenFlags()
	t.Permissions = b.ReadPermissions()
	t.GID = b.ReadGID()
}

// encode implements encoder.encode.
func (t *tlcreate) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteString(t.Name)
	b.WriteOpenFlags(t.OpenFlags)
	b.WritePermissions(t.Permissions)
	b.WriteGID(t.GID)
}

// typ implements message.typ.
func (*tlcreate) typ() msgType {
	return msgTlcreate
}

// String implements fmt.Stringer.
func (t *tlcreate) String() string {
	return fmt.Sprintf("Tlcreate{FID: %d, Name: %s, OpenFlags: %s, Permissions: 0o%o, GID: %d}", t.fid, t.Name, t.OpenFlags, t.Permissions, t.GID)
}

// rlcreate is a create response.
//
// The encode, decode, etc. methods are inherited from Rlopen.
type rlcreate struct {
	rlopen
}

// typ implements message.typ.
func (*rlcreate) typ() msgType {
	return msgRlcreate
}

// String implements fmt.Stringer.
func (r *rlcreate) String() string {
	return fmt.Sprintf("Rlcreate{QID: %s, IoUnit: %d}", r.QID, r.IoUnit)
}

// tsymlink is a symlink request.
type tsymlink struct {
	// Directory is the directory fid.
	Directory fid

	// Name is the new in the directory.
	Name string

	// Target is the symlink target.
	Target string

	// GID is the owning group.
	GID GID
}

// decode implements encoder.decode.
func (t *tsymlink) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Name = b.ReadString()
	t.Target = b.ReadString()
	t.GID = b.ReadGID()
}

// encode implements encoder.encode.
func (t *tsymlink) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.WriteString(t.Name)
	b.WriteString(t.Target)
	b.WriteGID(t.GID)
}

// typ implements message.typ.
func (*tsymlink) typ() msgType {
	return msgTsymlink
}

// String implements fmt.Stringer.
func (t *tsymlink) String() string {
	return fmt.Sprintf("Tsymlink{DirectoryFID: %d, Name: %s, Target: %s, GID: %d}", t.Directory, t.Name, t.Target, t.GID)
}

// rsymlink is a symlink response.
type rsymlink struct {
	// QID is the new symlink's QID.
	QID QID
}

// decode implements encoder.decode.
func (r *rsymlink) decode(b *buffer) {
	r.QID.decode(b)
}

// encode implements encoder.encode.
func (r *rsymlink) encode(b *buffer) {
	r.QID.encode(b)
}

// typ implements message.typ.
func (*rsymlink) typ() msgType {
	return msgRsymlink
}

// String implements fmt.Stringer.
func (r *rsymlink) String() string {
	return fmt.Sprintf("Rsymlink{QID: %s}", r.QID)
}

// tlink is a link request.
type tlink struct {
	// Directory is the directory to contain the link.
	Directory fid

	// fid is the target.
	Target fid

	// Name is the new source name.
	Name string
}

// decode implements encoder.decode.
func (t *tlink) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Target = b.ReadFID()
	t.Name = b.ReadString()
}

// encode implements encoder.encode.
func (t *tlink) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.WriteFID(t.Target)
	b.WriteString(t.Name)
}

// typ implements message.typ.
func (*tlink) typ() msgType {
	return msgTlink
}

// String implements fmt.Stringer.
func (t *tlink) String() string {
	return fmt.Sprintf("Tlink{DirectoryFID: %d, TargetFID: %d, Name: %s}", t.Directory, t.Target, t.Name)
}

// rlink is a link response.
type rlink struct {
}

// typ implements message.typ.
func (*rlink) typ() msgType {
	return msgRlink
}

// decode implements encoder.decode.
func (*rlink) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rlink) encode(b *buffer) {
}

// String implements fmt.Stringer.
func (r *rlink) String() string {
	return fmt.Sprintf("Rlink{}")
}

// trenameat is a rename request.
type trenameat struct {
	// OldDirectory is the source directory.
	OldDirectory fid

	// OldName is the source file name.
	OldName string

	// NewDirectory is the target directory.
	NewDirectory fid

	// NewName is the new file name.
	NewName string
}

// decode implements encoder.decode.
func (t *trenameat) decode(b *buffer) {
	t.OldDirectory = b.ReadFID()
	t.OldName = b.ReadString()
	t.NewDirectory = b.ReadFID()
	t.NewName = b.ReadString()
}

// encode implements encoder.encode.
func (t *trenameat) encode(b *buffer) {
	b.WriteFID(t.OldDirectory)
	b.WriteString(t.OldName)
	b.WriteFID(t.NewDirectory)
	b.WriteString(t.NewName)
}

// typ implements message.typ.
func (*trenameat) typ() msgType {
	return msgTrenameat
}

// String implements fmt.Stringer.
func (t *trenameat) String() string {
	return fmt.Sprintf("TrenameAt{OldDirectoryFID: %d, OldName: %s, NewDirectoryFID: %d, NewName: %s}", t.OldDirectory, t.OldName, t.NewDirectory, t.NewName)
}

// rrenameat is a rename response.
type rrenameat struct {
}

// decode implements encoder.decode.
func (*rrenameat) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rrenameat) encode(b *buffer) {
}

// typ implements message.typ.
func (*rrenameat) typ() msgType {
	return msgRrenameat
}

// String implements fmt.Stringer.
func (r *rrenameat) String() string {
	return fmt.Sprintf("Rrenameat{}")
}

// tunlinkat is an unlink request.
type tunlinkat struct {
	// Directory is the originating directory.
	Directory fid

	// Name is the name of the entry to unlink.
	Name string

	// Flags are extra flags (e.g. O_DIRECTORY). These are not interpreted by p9.
	Flags uint32
}

// decode implements encoder.decode.
func (t *tunlinkat) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Name = b.ReadString()
	t.Flags = b.Read32()
}

// encode implements encoder.encode.
func (t *tunlinkat) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.WriteString(t.Name)
	b.Write32(t.Flags)
}

// typ implements message.typ.
func (*tunlinkat) typ() msgType {
	return msgTunlinkat
}

// String implements fmt.Stringer.
func (t *tunlinkat) String() string {
	return fmt.Sprintf("Tunlinkat{DirectoryFID: %d, Name: %s, Flags: 0x%X}", t.Directory, t.Name, t.Flags)
}

// runlinkat is an unlink response.
type runlinkat struct {
}

// decode implements encoder.decode.
func (*runlinkat) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*runlinkat) encode(b *buffer) {
}

// typ implements message.typ.
func (*runlinkat) typ() msgType {
	return msgRunlinkat
}

// String implements fmt.Stringer.
func (r *runlinkat) String() string {
	return fmt.Sprintf("Runlinkat{}")
}

// trename is a rename request.
type trename struct {
	// fid is the fid to rename.
	fid fid

	// Directory is the target directory.
	Directory fid

	// Name is the new file name.
	Name string
}

// decode implements encoder.decode.
func (t *trename) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Directory = b.ReadFID()
	t.Name = b.ReadString()
}

// encode implements encoder.encode.
func (t *trename) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteFID(t.Directory)
	b.WriteString(t.Name)
}

// typ implements message.typ.
func (*trename) typ() msgType {
	return msgTrename
}

// String implements fmt.Stringer.
func (t *trename) String() string {
	return fmt.Sprintf("Trename{FID: %d, DirectoryFID: %d, Name: %s}", t.fid, t.Directory, t.Name)
}

// rrename is a rename response.
type rrename struct {
}

// decode implements encoder.decode.
func (*rrename) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rrename) encode(b *buffer) {
}

// typ implements message.typ.
func (*rrename) typ() msgType {
	return msgRrename
}

// String implements fmt.Stringer.
func (r *rrename) String() string {
	return fmt.Sprintf("Rrename{}")
}

// treadlink is a readlink request.
type treadlink struct {
	// fid is the symlink.
	fid fid
}

// decode implements encoder.decode.
func (t *treadlink) decode(b *buffer) {
	t.fid = b.ReadFID()
}

// encode implements encoder.encode.
func (t *treadlink) encode(b *buffer) {
	b.WriteFID(t.fid)
}

// typ implements message.typ.
func (*treadlink) typ() msgType {
	return msgTreadlink
}

// String implements fmt.Stringer.
func (t *treadlink) String() string {
	return fmt.Sprintf("Treadlink{FID: %d}", t.fid)
}

// rreadlink is a readlink response.
type rreadlink struct {
	// Target is the symlink target.
	Target string
}

// decode implements encoder.decode.
func (r *rreadlink) decode(b *buffer) {
	r.Target = b.ReadString()
}

// encode implements encoder.encode.
func (r *rreadlink) encode(b *buffer) {
	b.WriteString(r.Target)
}

// typ implements message.typ.
func (*rreadlink) typ() msgType {
	return msgRreadlink
}

// String implements fmt.Stringer.
func (r *rreadlink) String() string {
	return fmt.Sprintf("Rreadlink{Target: %s}", r.Target)
}

// tread is a read request.
type tread struct {
	// fid is the fid to read.
	fid fid

	// Offset indicates the file offset.
	Offset uint64

	// Count indicates the number of bytes to read.
	Count uint32
}

// decode implements encoder.decode.
func (t *tread) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Offset = b.Read64()
	t.Count = b.Read32()
}

// encode implements encoder.encode.
func (t *tread) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.Write64(t.Offset)
	b.Write32(t.Count)
}

// typ implements message.typ.
func (*tread) typ() msgType {
	return msgTread
}

// String implements fmt.Stringer.
func (t *tread) String() string {
	return fmt.Sprintf("Tread{FID: %d, Offset: %d, Count: %d}", t.fid, t.Offset, t.Count)
}

// rreadServerPayloader is the response for a Tread by p9 servers.
//
// rreadServerPayloader exists so the fuzzer can fuzz rread -- however,
// PayloadCleanup causes it to panic, and putting connState in the fuzzer seems
// excessive.
type rreadServerPayloader struct {
	rread

	fullBuffer []byte
	cs         *connState
}

// rread is the response for a Tread.
type rread struct {
	// Data is the resulting data.
	Data []byte
}

// decode implements encoder.decode.
//
// Data is automatically decoded via Payload.
func (r *rread) decode(b *buffer) {
	count := b.Read32()
	if count != uint32(len(r.Data)) {
		b.markOverrun()
	}
}

// encode implements encoder.encode.
//
// Data is automatically encoded via Payload.
func (r *rread) encode(b *buffer) {
	b.Write32(uint32(len(r.Data)))
}

// typ implements message.typ.
func (*rread) typ() msgType {
	return msgRread
}

// FixedSize implements payloader.FixedSize.
func (*rread) FixedSize() uint32 {
	return 4
}

// Payload implements payloader.Payload.
func (r *rread) Payload() []byte {
	return r.Data
}

// SetPayload implements payloader.SetPayload.
func (r *rread) SetPayload(p []byte) {
	r.Data = p
}

func (*rread) PayloadCleanup() {}

// FixedSize implements payloader.FixedSize.
func (*rreadServerPayloader) FixedSize() uint32 {
	return 4
}

// Payload implements payloader.Payload.
func (r *rreadServerPayloader) Payload() []byte {
	return r.Data
}

// SetPayload implements payloader.SetPayload.
func (r *rreadServerPayloader) SetPayload(p []byte) {
	r.Data = p
}

// PayloadCleanup implements payloader.PayloadCleanup.
func (r *rreadServerPayloader) PayloadCleanup() {
	// Fill it with zeros to not risk leaking previous files' data.
	copy(r.Data, r.cs.pristineZeros)
	r.cs.readBufPool.Put(&r.fullBuffer)
}

// String implements fmt.Stringer.
func (r *rread) String() string {
	return fmt.Sprintf("Rread{len(Data): %d}", len(r.Data))
}

// twrite is a write request.
type twrite struct {
	// fid is the fid to read.
	fid fid

	// Offset indicates the file offset.
	Offset uint64

	// Data is the data to be written.
	Data []byte
}

// decode implements encoder.decode.
func (t *twrite) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Offset = b.Read64()
	count := b.Read32()
	if count != uint32(len(t.Data)) {
		b.markOverrun()
	}
}

// encode implements encoder.encode.
//
// This uses the buffer payload to avoid a copy.
func (t *twrite) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.Write64(t.Offset)
	b.Write32(uint32(len(t.Data)))
}

// typ implements message.typ.
func (*twrite) typ() msgType {
	return msgTwrite
}

// FixedSize implements payloader.FixedSize.
func (*twrite) FixedSize() uint32 {
	return 16
}

// Payload implements payloader.Payload.
func (t *twrite) Payload() []byte {
	return t.Data
}

func (t *twrite) PayloadCleanup() {}

// SetPayload implements payloader.SetPayload.
func (t *twrite) SetPayload(p []byte) {
	t.Data = p
}

// String implements fmt.Stringer.
func (t *twrite) String() string {
	return fmt.Sprintf("Twrite{FID: %v, Offset %d, len(Data): %d}", t.fid, t.Offset, len(t.Data))
}

// rwrite is the response for a Twrite.
type rwrite struct {
	// Count indicates the number of bytes successfully written.
	Count uint32
}

// decode implements encoder.decode.
func (r *rwrite) decode(b *buffer) {
	r.Count = b.Read32()
}

// encode implements encoder.encode.
func (r *rwrite) encode(b *buffer) {
	b.Write32(r.Count)
}

// typ implements message.typ.
func (*rwrite) typ() msgType {
	return msgRwrite
}

// String implements fmt.Stringer.
func (r *rwrite) String() string {
	return fmt.Sprintf("Rwrite{Count: %d}", r.Count)
}

// tmknod is a mknod request.
type tmknod struct {
	// Directory is the parent directory.
	Directory fid

	// Name is the device name.
	Name string

	// Mode is the device mode and permissions.
	Mode FileMode

	// Major is the device major number.
	Major uint32

	// Minor is the device minor number.
	Minor uint32

	// GID is the device GID.
	GID GID
}

// decode implements encoder.decode.
func (t *tmknod) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Name = b.ReadString()
	t.Mode = b.ReadFileMode()
	t.Major = b.Read32()
	t.Minor = b.Read32()
	t.GID = b.ReadGID()
}

// encode implements encoder.encode.
func (t *tmknod) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.WriteString(t.Name)
	b.WriteFileMode(t.Mode)
	b.Write32(t.Major)
	b.Write32(t.Minor)
	b.WriteGID(t.GID)
}

// typ implements message.typ.
func (*tmknod) typ() msgType {
	return msgTmknod
}

// String implements fmt.Stringer.
func (t *tmknod) String() string {
	return fmt.Sprintf("Tmknod{DirectoryFID: %d, Name: %s, Mode: 0o%o, Major: %d, Minor: %d, GID: %d}", t.Directory, t.Name, t.Mode, t.Major, t.Minor, t.GID)
}

// rmknod is a mknod response.
type rmknod struct {
	// QID is the resulting QID.
	QID QID
}

// decode implements encoder.decode.
func (r *rmknod) decode(b *buffer) {
	r.QID.decode(b)
}

// encode implements encoder.encode.
func (r *rmknod) encode(b *buffer) {
	r.QID.encode(b)
}

// typ implements message.typ.
func (*rmknod) typ() msgType {
	return msgRmknod
}

// String implements fmt.Stringer.
func (r *rmknod) String() string {
	return fmt.Sprintf("Rmknod{QID: %s}", r.QID)
}

// tmkdir is a mkdir request.
type tmkdir struct {
	// Directory is the parent directory.
	Directory fid

	// Name is the new directory name.
	Name string

	// Permissions is the set of permission bits.
	Permissions FileMode

	// GID is the owning group.
	GID GID
}

// decode implements encoder.decode.
func (t *tmkdir) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Name = b.ReadString()
	t.Permissions = b.ReadPermissions()
	t.GID = b.ReadGID()
}

// encode implements encoder.encode.
func (t *tmkdir) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.WriteString(t.Name)
	b.WritePermissions(t.Permissions)
	b.WriteGID(t.GID)
}

// typ implements message.typ.
func (*tmkdir) typ() msgType {
	return msgTmkdir
}

// String implements fmt.Stringer.
func (t *tmkdir) String() string {
	return fmt.Sprintf("Tmkdir{DirectoryFID: %d, Name: %s, Permissions: 0o%o, GID: %d}", t.Directory, t.Name, t.Permissions, t.GID)
}

// rmkdir is a mkdir response.
type rmkdir struct {
	// QID is the resulting QID.
	QID QID
}

// decode implements encoder.decode.
func (r *rmkdir) decode(b *buffer) {
	r.QID.decode(b)
}

// encode implements encoder.encode.
func (r *rmkdir) encode(b *buffer) {
	r.QID.encode(b)
}

// typ implements message.typ.
func (*rmkdir) typ() msgType {
	return msgRmkdir
}

// String implements fmt.Stringer.
func (r *rmkdir) String() string {
	return fmt.Sprintf("Rmkdir{QID: %s}", r.QID)
}

// tgetattr is a getattr request.
type tgetattr struct {
	// fid is the fid to get attributes for.
	fid fid

	// AttrMask is the set of attributes to get.
	AttrMask AttrMask
}

// decode implements encoder.decode.
func (t *tgetattr) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.AttrMask.decode(b)
}

// encode implements encoder.encode.
func (t *tgetattr) encode(b *buffer) {
	b.WriteFID(t.fid)
	t.AttrMask.encode(b)
}

// typ implements message.typ.
func (*tgetattr) typ() msgType {
	return msgTgetattr
}

// String implements fmt.Stringer.
func (t *tgetattr) String() string {
	return fmt.Sprintf("Tgetattr{FID: %d, AttrMask: %s}", t.fid, t.AttrMask)
}

// rgetattr is a getattr response.
type rgetattr struct {
	// Valid indicates which fields are valid.
	Valid AttrMask

	// QID is the QID for this file.
	QID

	// Attr is the set of attributes.
	Attr Attr
}

// decode implements encoder.decode.
func (r *rgetattr) decode(b *buffer) {
	r.Valid.decode(b)
	r.QID.decode(b)
	r.Attr.decode(b)
}

// encode implements encoder.encode.
func (r *rgetattr) encode(b *buffer) {
	r.Valid.encode(b)
	r.QID.encode(b)
	r.Attr.encode(b)
}

// typ implements message.typ.
func (*rgetattr) typ() msgType {
	return msgRgetattr
}

// String implements fmt.Stringer.
func (r *rgetattr) String() string {
	return fmt.Sprintf("Rgetattr{Valid: %v, QID: %s, Attr: %s}", r.Valid, r.QID, r.Attr)
}

// tsetattr is a setattr request.
type tsetattr struct {
	// fid is the fid to change.
	fid fid

	// Valid is the set of bits which will be used.
	Valid SetAttrMask

	// SetAttr is the set request.
	SetAttr SetAttr
}

// decode implements encoder.decode.
func (t *tsetattr) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Valid.decode(b)
	t.SetAttr.decode(b)
}

// encode implements encoder.encode.
func (t *tsetattr) encode(b *buffer) {
	b.WriteFID(t.fid)
	t.Valid.encode(b)
	t.SetAttr.encode(b)
}

// typ implements message.typ.
func (*tsetattr) typ() msgType {
	return msgTsetattr
}

// String implements fmt.Stringer.
func (t *tsetattr) String() string {
	return fmt.Sprintf("Tsetattr{FID: %d, Valid: %v, SetAttr: %s}", t.fid, t.Valid, t.SetAttr)
}

// rsetattr is a setattr response.
type rsetattr struct {
}

// decode implements encoder.decode.
func (*rsetattr) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rsetattr) encode(b *buffer) {
}

// typ implements message.typ.
func (*rsetattr) typ() msgType {
	return msgRsetattr
}

// String implements fmt.Stringer.
func (r *rsetattr) String() string {
	return fmt.Sprintf("Rsetattr{}")
}

// txattrwalk walks extended attributes.
type txattrwalk struct {
	// fid is the fid to check for attributes.
	fid fid

	// newFID is the new fid associated with the attributes.
	newFID fid

	// Name is the attribute name.
	Name string
}

// decode implements encoder.decode.
func (t *txattrwalk) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.newFID = b.ReadFID()
	t.Name = b.ReadString()
}

// encode implements encoder.encode.
func (t *txattrwalk) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteFID(t.newFID)
	b.WriteString(t.Name)
}

// typ implements message.typ.
func (*txattrwalk) typ() msgType {
	return msgTxattrwalk
}

// String implements fmt.Stringer.
func (t *txattrwalk) String() string {
	return fmt.Sprintf("Txattrwalk{FID: %d, newFID: %d, Name: %s}", t.fid, t.newFID, t.Name)
}

// rxattrwalk is a xattrwalk response.
type rxattrwalk struct {
	// Size is the size of the extended attribute.
	Size uint64
}

// decode implements encoder.decode.
func (r *rxattrwalk) decode(b *buffer) {
	r.Size = b.Read64()
}

// encode implements encoder.encode.
func (r *rxattrwalk) encode(b *buffer) {
	b.Write64(r.Size)
}

// typ implements message.typ.
func (*rxattrwalk) typ() msgType {
	return msgRxattrwalk
}

// String implements fmt.Stringer.
func (r *rxattrwalk) String() string {
	return fmt.Sprintf("Rxattrwalk{Size: %d}", r.Size)
}

// txattrcreate prepare to set extended attributes.
type txattrcreate struct {
	// fid is input/output parameter, it identifies the file on which
	// extended attributes will be set but after successful Rxattrcreate
	// it is used to write the extended attribute value.
	fid fid

	// Name is the attribute name.
	Name string

	// Size of the attribute value. When the fid is clunked it has to match
	// the number of bytes written to the fid.
	AttrSize uint64

	// Linux setxattr(2) flags.
	Flags uint32
}

// decode implements encoder.decode.
func (t *txattrcreate) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Name = b.ReadString()
	t.AttrSize = b.Read64()
	t.Flags = b.Read32()
}

// encode implements encoder.encode.
func (t *txattrcreate) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteString(t.Name)
	b.Write64(t.AttrSize)
	b.Write32(t.Flags)
}

// typ implements message.typ.
func (*txattrcreate) typ() msgType {
	return msgTxattrcreate
}

// String implements fmt.Stringer.
func (t *txattrcreate) String() string {
	return fmt.Sprintf("Txattrcreate{FID: %d, Name: %s, AttrSize: %d, Flags: %d}", t.fid, t.Name, t.AttrSize, t.Flags)
}

// rxattrcreate is a xattrcreate response.
type rxattrcreate struct {
}

// decode implements encoder.decode.
func (r *rxattrcreate) decode(b *buffer) {
}

// encode implements encoder.encode.
func (r *rxattrcreate) encode(b *buffer) {
}

// typ implements message.typ.
func (*rxattrcreate) typ() msgType {
	return msgRxattrcreate
}

// String implements fmt.Stringer.
func (r *rxattrcreate) String() string {
	return fmt.Sprintf("Rxattrcreate{}")
}

// treaddir is a readdir request.
type treaddir struct {
	// Directory is the directory fid to read.
	Directory fid

	// Offset is the offset to read at.
	Offset uint64

	// Count is the number of bytes to read.
	Count uint32
}

// decode implements encoder.decode.
func (t *treaddir) decode(b *buffer) {
	t.Directory = b.ReadFID()
	t.Offset = b.Read64()
	t.Count = b.Read32()
}

// encode implements encoder.encode.
func (t *treaddir) encode(b *buffer) {
	b.WriteFID(t.Directory)
	b.Write64(t.Offset)
	b.Write32(t.Count)
}

// typ implements message.typ.
func (*treaddir) typ() msgType {
	return msgTreaddir
}

// String implements fmt.Stringer.
func (t *treaddir) String() string {
	return fmt.Sprintf("Treaddir{DirectoryFID: %d, Offset: %d, Count: %d}", t.Directory, t.Offset, t.Count)
}

// rreaddir is a readdir response.
type rreaddir struct {
	// Count is the byte limit.
	//
	// This should always be set from the Treaddir request.
	Count uint32

	// Entries are the resulting entries.
	//
	// This may be constructed in decode.
	Entries []Dirent

	// payload is the encoded payload.
	//
	// This is constructed by encode.
	payload []byte
}

// decode implements encoder.decode.
func (r *rreaddir) decode(b *buffer) {
	r.Count = b.Read32()
	entriesBuf := buffer{data: r.payload}
	r.Entries = r.Entries[:0]
	for {
		var d Dirent
		d.decode(&entriesBuf)
		if entriesBuf.isOverrun() {
			// Couldn't decode a complete entry.
			break
		}
		r.Entries = append(r.Entries, d)
	}
}

// encode implements encoder.encode.
func (r *rreaddir) encode(b *buffer) {
	entriesBuf := buffer{}
	payloadSize := 0
	for _, d := range r.Entries {
		d.encode(&entriesBuf)
		if len(entriesBuf.data) > int(r.Count) {
			break
		}
		payloadSize = len(entriesBuf.data)
	}
	r.Count = uint32(payloadSize)
	r.payload = entriesBuf.data[:payloadSize]
	b.Write32(r.Count)
}

// typ implements message.typ.
func (*rreaddir) typ() msgType {
	return msgRreaddir
}

// FixedSize implements payloader.FixedSize.
func (*rreaddir) FixedSize() uint32 {
	return 4
}

// Payload implements payloader.Payload.
func (r *rreaddir) Payload() []byte {
	return r.payload
}

func (r *rreaddir) PayloadCleanup() {}

// SetPayload implements payloader.SetPayload.
func (r *rreaddir) SetPayload(p []byte) {
	r.payload = p
}

// String implements fmt.Stringer.
func (r *rreaddir) String() string {
	return fmt.Sprintf("Rreaddir{Count: %d, Entries: %s}", r.Count, r.Entries)
}

// Tfsync is an fsync request.
type tfsync struct {
	// fid is the fid to sync.
	fid fid
}

// decode implements encoder.decode.
func (t *tfsync) decode(b *buffer) {
	t.fid = b.ReadFID()
}

// encode implements encoder.encode.
func (t *tfsync) encode(b *buffer) {
	b.WriteFID(t.fid)
}

// typ implements message.typ.
func (*tfsync) typ() msgType {
	return msgTfsync
}

// String implements fmt.Stringer.
func (t *tfsync) String() string {
	return fmt.Sprintf("Tfsync{FID: %d}", t.fid)
}

// rfsync is an fsync response.
type rfsync struct {
}

// decode implements encoder.decode.
func (*rfsync) decode(b *buffer) {
}

// encode implements encoder.encode.
func (*rfsync) encode(b *buffer) {
}

// typ implements message.typ.
func (*rfsync) typ() msgType {
	return msgRfsync
}

// String implements fmt.Stringer.
func (r *rfsync) String() string {
	return fmt.Sprintf("Rfsync{}")
}

// tstatfs is a stat request.
type tstatfs struct {
	// fid is the root.
	fid fid
}

// decode implements encoder.decode.
func (t *tstatfs) decode(b *buffer) {
	t.fid = b.ReadFID()
}

// encode implements encoder.encode.
func (t *tstatfs) encode(b *buffer) {
	b.WriteFID(t.fid)
}

// typ implements message.typ.
func (*tstatfs) typ() msgType {
	return msgTstatfs
}

// String implements fmt.Stringer.
func (t *tstatfs) String() string {
	return fmt.Sprintf("Tstatfs{FID: %d}", t.fid)
}

// rstatfs is the response for a Tstatfs.
type rstatfs struct {
	// FSStat is the stat result.
	FSStat FSStat
}

// decode implements encoder.decode.
func (r *rstatfs) decode(b *buffer) {
	r.FSStat.decode(b)
}

// encode implements encoder.encode.
func (r *rstatfs) encode(b *buffer) {
	r.FSStat.encode(b)
}

// typ implements message.typ.
func (*rstatfs) typ() msgType {
	return msgRstatfs
}

// String implements fmt.Stringer.
func (r *rstatfs) String() string {
	return fmt.Sprintf("Rstatfs{FSStat: %v}", r.FSStat)
}

// twalkgetattr is a walk request.
type twalkgetattr struct {
	// fid is the fid to be walked.
	fid fid

	// newFID is the resulting fid.
	newFID fid

	// Names are the set of names to be walked.
	Names []string
}

// decode implements encoder.decode.
func (t *twalkgetattr) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.newFID = b.ReadFID()
	n := b.Read16()
	t.Names = t.Names[:0]
	for i := 0; i < int(n); i++ {
		t.Names = append(t.Names, b.ReadString())
	}
}

// encode implements encoder.encode.
func (t *twalkgetattr) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.WriteFID(t.newFID)
	b.Write16(uint16(len(t.Names)))
	for _, name := range t.Names {
		b.WriteString(name)
	}
}

// typ implements message.typ.
func (*twalkgetattr) typ() msgType {
	return msgTwalkgetattr
}

// String implements fmt.Stringer.
func (t *twalkgetattr) String() string {
	return fmt.Sprintf("Twalkgetattr{FID: %d, newFID: %d, Names: %v}", t.fid, t.newFID, t.Names)
}

// rwalkgetattr is a walk response.
type rwalkgetattr struct {
	// Valid indicates which fields are valid in the Attr below.
	Valid AttrMask

	// Attr is the set of attributes for the last QID (the file walked to).
	Attr Attr

	// QIDs are the set of QIDs returned.
	QIDs []QID
}

// decode implements encoder.decode.
func (r *rwalkgetattr) decode(b *buffer) {
	r.Valid.decode(b)
	r.Attr.decode(b)
	n := b.Read16()
	r.QIDs = r.QIDs[:0]
	for i := 0; i < int(n); i++ {
		var q QID
		q.decode(b)
		r.QIDs = append(r.QIDs, q)
	}
}

// encode implements encoder.encode.
func (r *rwalkgetattr) encode(b *buffer) {
	r.Valid.encode(b)
	r.Attr.encode(b)
	b.Write16(uint16(len(r.QIDs)))
	for _, q := range r.QIDs {
		q.encode(b)
	}
}

// typ implements message.typ.
func (*rwalkgetattr) typ() msgType {
	return msgRwalkgetattr
}

// String implements fmt.Stringer.
func (r *rwalkgetattr) String() string {
	return fmt.Sprintf("Rwalkgetattr{Valid: %s, Attr: %s, QIDs: %v}", r.Valid, r.Attr, r.QIDs)
}

// tucreate is a tlcreate message that includes a UID.
type tucreate struct {
	tlcreate

	// UID is the UID to use as the effective UID in creation messages.
	UID UID
}

// decode implements encoder.decode.
func (t *tucreate) decode(b *buffer) {
	t.tlcreate.decode(b)
	t.UID = b.ReadUID()
}

// encode implements encoder.encode.
func (t *tucreate) encode(b *buffer) {
	t.tlcreate.encode(b)
	b.WriteUID(t.UID)
}

// typ implements message.typ.
func (t *tucreate) typ() msgType {
	return msgTucreate
}

// String implements fmt.Stringer.
func (t *tucreate) String() string {
	return fmt.Sprintf("Tucreate{Tlcreate: %v, UID: %d}", &t.tlcreate, t.UID)
}

// rucreate is a file creation response.
type rucreate struct {
	rlcreate
}

// typ implements message.typ.
func (*rucreate) typ() msgType {
	return msgRucreate
}

// String implements fmt.Stringer.
func (r *rucreate) String() string {
	return fmt.Sprintf("Rucreate{%v}", &r.rlcreate)
}

// tumkdir is a Tmkdir message that includes a UID.
type tumkdir struct {
	tmkdir

	// UID is the UID to use as the effective UID in creation messages.
	UID UID
}

// decode implements encoder.decode.
func (t *tumkdir) decode(b *buffer) {
	t.tmkdir.decode(b)
	t.UID = b.ReadUID()
}

// encode implements encoder.encode.
func (t *tumkdir) encode(b *buffer) {
	t.tmkdir.encode(b)
	b.WriteUID(t.UID)
}

// typ implements message.typ.
func (t *tumkdir) typ() msgType {
	return msgTumkdir
}

// String implements fmt.Stringer.
func (t *tumkdir) String() string {
	return fmt.Sprintf("Tumkdir{Tmkdir: %v, UID: %d}", &t.tmkdir, t.UID)
}

// rumkdir is a umkdir response.
type rumkdir struct {
	rmkdir
}

// typ implements message.typ.
func (*rumkdir) typ() msgType {
	return msgRumkdir
}

// String implements fmt.Stringer.
func (r *rumkdir) String() string {
	return fmt.Sprintf("Rumkdir{%v}", &r.rmkdir)
}

// tumknod is a Tmknod message that includes a UID.
type tumknod struct {
	tmknod

	// UID is the UID to use as the effective UID in creation messages.
	UID UID
}

// decode implements encoder.decode.
func (t *tumknod) decode(b *buffer) {
	t.tmknod.decode(b)
	t.UID = b.ReadUID()
}

// encode implements encoder.encode.
func (t *tumknod) encode(b *buffer) {
	t.tmknod.encode(b)
	b.WriteUID(t.UID)
}

// typ implements message.typ.
func (t *tumknod) typ() msgType {
	return msgTumknod
}

// String implements fmt.Stringer.
func (t *tumknod) String() string {
	return fmt.Sprintf("Tumknod{Tmknod: %v, UID: %d}", &t.tmknod, t.UID)
}

// rumknod is a umknod response.
type rumknod struct {
	rmknod
}

// typ implements message.typ.
func (*rumknod) typ() msgType {
	return msgRumknod
}

// String implements fmt.Stringer.
func (r *rumknod) String() string {
	return fmt.Sprintf("Rumknod{%v}", &r.rmknod)
}

// tusymlink is a Tsymlink message that includes a UID.
type tusymlink struct {
	tsymlink

	// UID is the UID to use as the effective UID in creation messages.
	UID UID
}

// decode implements encoder.decode.
func (t *tusymlink) decode(b *buffer) {
	t.tsymlink.decode(b)
	t.UID = b.ReadUID()
}

// encode implements encoder.encode.
func (t *tusymlink) encode(b *buffer) {
	t.tsymlink.encode(b)
	b.WriteUID(t.UID)
}

// typ implements message.typ.
func (t *tusymlink) typ() msgType {
	return msgTusymlink
}

// String implements fmt.Stringer.
func (t *tusymlink) String() string {
	return fmt.Sprintf("Tusymlink{Tsymlink: %v, UID: %d}", &t.tsymlink, t.UID)
}

// rusymlink is a usymlink response.
type rusymlink struct {
	rsymlink
}

// typ implements message.typ.
func (*rusymlink) typ() msgType {
	return msgRusymlink
}

// String implements fmt.Stringer.
func (r *rusymlink) String() string {
	return fmt.Sprintf("Rusymlink{%v}", &r.rsymlink)
}

// LockType is lock type for Tlock
type LockType uint8

// These constants define Lock operations: Read, Write, and Un(lock)
// They map to Linux values of F_RDLCK, F_WRLCK, F_UNLCK.
// If that seems a little Linux-centric, recall that the "L"
// in 9P2000.L means "Linux" :-)
const (
	ReadLock LockType = iota
	WriteLock
	Unlock
)

func (l LockType) String() string {
	switch l {
	case ReadLock:
		return "ReadLock"
	case WriteLock:
		return "WriteLock"
	case Unlock:
		return "Unlock"
	}
	return "unknown lock type"
}

// LockFlags are flags for the lock. Currently, and possibly forever, only one
// is really used: LockFlagsBlock
type LockFlags uint32

const (
	// LockFlagsBlock indicates a blocking request.
	LockFlagsBlock LockFlags = 1

	// LockFlagsReclaim is "Reserved for future use."
	// It's been some time since 9P2000.L came about,
	// I suspect "future" in this case is "never"?
	LockFlagsReclaim LockFlags = 2
)

// LockStatus contains lock status result.
type LockStatus uint8

// These are the four current return values for Rlock.
const (
	LockStatusOK LockStatus = iota
	LockStatusBlocked
	LockStatusError
	LockStatusGrace
)

func (s LockStatus) String() string {
	switch s {
	case LockStatusOK:
		return "LockStatusOK"
	case LockStatusBlocked:
		return "LockStatusBlocked"
	case LockStatusError:
		return "LockStatusError"
	case LockStatusGrace:
		return "LockStatusGrace"
	}
	return "unknown lock status"
}

// tlock is a Tlock message
type tlock struct {
	// fid is the fid to lock.
	fid fid

	Type   LockType  // Type of lock: F_RDLCK, F_WRLCK, F_UNLCK */
	Flags  LockFlags // flags, not whence, docs are wrong.
	Start  uint64    // Starting offset for lock
	Length uint64    // Number of bytes to lock
	PID    int32     // PID of process blocking our lock (F_GETLK only)

	// "client_id is an additional mechanism for uniquely
	// identifying the lock requester and is set to the nodename
	// by the Linux v9fs client."
	// https://github.com/chaos/diod/blob/master/protocol.md#lock---acquire-or-release-a-posix-record-lock
	Client string // Client id -- but technically can be anything.
}

// decode implements encoder.decode.
func (t *tlock) decode(b *buffer) {
	t.fid = b.ReadFID()
	t.Type = LockType(b.Read8())
	t.Flags = LockFlags(b.Read32())
	t.Start = b.Read64()
	t.Length = b.Read64()
	t.PID = int32(b.Read32())
	t.Client = b.ReadString()
}

// encode implements encoder.encode.
func (t *tlock) encode(b *buffer) {
	b.WriteFID(t.fid)
	b.Write8(uint8(t.Type))
	b.Write32(uint32(t.Flags))
	b.Write64(t.Start)
	b.Write64(t.Length)
	b.Write32(uint32(t.PID))
	b.WriteString(t.Client)
}

// typ implements message.typ.
func (*tlock) typ() msgType {
	return msgTlock
}

// String implements fmt.Stringer.
func (t *tlock) String() string {
	return fmt.Sprintf("Tlock{Type: %s, Flags: %#x, Start: %d, Length: %d, PID: %d, Client: %s}", t.Type.String(), t.Flags, t.Start, t.Length, t.PID, t.Client)
}

// rlock is a lock response.
type rlock struct {
	Status LockStatus
}

// decode implements encoder.decode.
func (r *rlock) decode(b *buffer) {
	r.Status = LockStatus(b.Read8())
}

// encode implements encoder.encode.
func (r *rlock) encode(b *buffer) {
	b.Write8(uint8(r.Status))
}

// typ implements message.typ.
func (*rlock) typ() msgType {
	return msgRlock
}

// String implements fmt.Stringer.
func (r *rlock) String() string {
	return fmt.Sprintf("Rlock{Status: %s}", r.Status)
}

// Let's wait until we need this? POSIX locks over a network make 0 sense.
// getlock - test for the existence of a POSIX record lock
// size[4] Tgetlock tag[2] fid[4] type[1] start[8] length[8] proc_id[4] client_id[s]
// size[4] Rgetlock tag[2] type[1] start[8] length[8] proc_id[4] client_id[s]
// getlock tests for the existence of a POSIX record lock and has semantics similar to Linux fcntl(F_GETLK).

// As with lock, type has one of the values defined above, and start,
// length, and proc_id correspond to the analogous fields in struct
// flock passed to Linux fcntl(F_GETLK), and client_Id is an
// additional mechanism for uniquely identifying the lock requester
// and is set to the nodename by the Linux v9fs client.  tusymlink is
// a Tsymlink message that includes a UID.

/// END LOCK

const maxCacheSize = 3

// msgFactory is used to reduce allocations by caching messages for reuse.
type msgFactory struct {
	create func() message
	cache  chan message
}

// msgDotLRegistry indexes all 9P2000.L(.Google.N) message factories by type.
var msgDotLRegistry registry

type registry struct {
	factories [math.MaxUint8 + 1]msgFactory

	// largestFixedSize is computed so that given some message size M, you can
	// compute the maximum payload size (e.g. for Twrite, Rread) with
	// M-largestFixedSize. You could do this individual on a per-message basis,
	// but it's easier to compute a single maximum safe payload.
	largestFixedSize uint32
}

// get returns a new message by type.
//
// An error is returned in the case of an unknown message.
//
// This takes, and ignores, a message tag so that it may be used directly as a
// lookuptagAndType function for recv (by design).
func (r *registry) get(_ tag, t msgType) (message, error) {
	entry := &r.factories[t]
	if entry.create == nil {
		return nil, &ErrInvalidMsgType{t}
	}

	select {
	case msg := <-entry.cache:
		return msg, nil
	default:
		return entry.create(), nil
	}
}

func (r *registry) put(msg message) {
	if p, ok := msg.(payloader); ok {
		p.SetPayload(nil)
	}

	entry := &r.factories[msg.typ()]
	select {
	case entry.cache <- msg:
	default:
	}
}

// register registers the given message type.
//
// This may cause panic on failure and should only be used from init.
func (r *registry) register(t msgType, fn func() message) {
	if int(t) >= len(r.factories) {
		panic(fmt.Sprintf("message type %d is too large. It must be smaller than %d", t, len(r.factories)))
	}
	if r.factories[t].create != nil {
		panic(fmt.Sprintf("duplicate message type %d: first is %T, second is %T", t, r.factories[t].create(), fn()))
	}
	r.factories[t] = msgFactory{
		create: fn,
		cache:  make(chan message, maxCacheSize),
	}

	if size := calculateSize(fn()); size > r.largestFixedSize {
		r.largestFixedSize = size
	}
}

func calculateSize(m message) uint32 {
	if p, ok := m.(payloader); ok {
		return p.FixedSize()
	}
	var dataBuf buffer
	m.encode(&dataBuf)
	return uint32(len(dataBuf.data))
}

func init() {
	msgDotLRegistry.register(msgRlerror, func() message { return &rlerror{} })
	msgDotLRegistry.register(msgTstatfs, func() message { return &tstatfs{} })
	msgDotLRegistry.register(msgRstatfs, func() message { return &rstatfs{} })
	msgDotLRegistry.register(msgTlopen, func() message { return &tlopen{} })
	msgDotLRegistry.register(msgRlopen, func() message { return &rlopen{} })
	msgDotLRegistry.register(msgTlcreate, func() message { return &tlcreate{} })
	msgDotLRegistry.register(msgRlcreate, func() message { return &rlcreate{} })
	msgDotLRegistry.register(msgTsymlink, func() message { return &tsymlink{} })
	msgDotLRegistry.register(msgRsymlink, func() message { return &rsymlink{} })
	msgDotLRegistry.register(msgTmknod, func() message { return &tmknod{} })
	msgDotLRegistry.register(msgRmknod, func() message { return &rmknod{} })
	msgDotLRegistry.register(msgTrename, func() message { return &trename{} })
	msgDotLRegistry.register(msgRrename, func() message { return &rrename{} })
	msgDotLRegistry.register(msgTreadlink, func() message { return &treadlink{} })
	msgDotLRegistry.register(msgRreadlink, func() message { return &rreadlink{} })
	msgDotLRegistry.register(msgTgetattr, func() message { return &tgetattr{} })
	msgDotLRegistry.register(msgRgetattr, func() message { return &rgetattr{} })
	msgDotLRegistry.register(msgTsetattr, func() message { return &tsetattr{} })
	msgDotLRegistry.register(msgRsetattr, func() message { return &rsetattr{} })
	msgDotLRegistry.register(msgTxattrwalk, func() message { return &txattrwalk{} })
	msgDotLRegistry.register(msgRxattrwalk, func() message { return &rxattrwalk{} })
	msgDotLRegistry.register(msgTxattrcreate, func() message { return &txattrcreate{} })
	msgDotLRegistry.register(msgRxattrcreate, func() message { return &rxattrcreate{} })
	msgDotLRegistry.register(msgTreaddir, func() message { return &treaddir{} })
	msgDotLRegistry.register(msgRreaddir, func() message { return &rreaddir{} })
	msgDotLRegistry.register(msgTfsync, func() message { return &tfsync{} })
	msgDotLRegistry.register(msgRfsync, func() message { return &rfsync{} })
	msgDotLRegistry.register(msgTlink, func() message { return &tlink{} })
	msgDotLRegistry.register(msgRlink, func() message { return &rlink{} })
	msgDotLRegistry.register(msgTlock, func() message { return &tlock{} })
	msgDotLRegistry.register(msgRlock, func() message { return &rlock{} })
	msgDotLRegistry.register(msgTmkdir, func() message { return &tmkdir{} })
	msgDotLRegistry.register(msgRmkdir, func() message { return &rmkdir{} })
	msgDotLRegistry.register(msgTrenameat, func() message { return &trenameat{} })
	msgDotLRegistry.register(msgRrenameat, func() message { return &rrenameat{} })
	msgDotLRegistry.register(msgTunlinkat, func() message { return &tunlinkat{} })
	msgDotLRegistry.register(msgRunlinkat, func() message { return &runlinkat{} })
	msgDotLRegistry.register(msgTversion, func() message { return &tversion{} })
	msgDotLRegistry.register(msgRversion, func() message { return &rversion{} })
	msgDotLRegistry.register(msgTauth, func() message { return &tauth{} })
	msgDotLRegistry.register(msgRauth, func() message { return &rauth{} })
	msgDotLRegistry.register(msgTattach, func() message { return &tattach{} })
	msgDotLRegistry.register(msgRattach, func() message { return &rattach{} })
	msgDotLRegistry.register(msgTflush, func() message { return &tflush{} })
	msgDotLRegistry.register(msgRflush, func() message { return &rflush{} })
	msgDotLRegistry.register(msgTwalk, func() message { return &twalk{} })
	msgDotLRegistry.register(msgRwalk, func() message { return &rwalk{} })
	msgDotLRegistry.register(msgTread, func() message { return &tread{} })
	msgDotLRegistry.register(msgRread, func() message { return &rread{} })
	msgDotLRegistry.register(msgTwrite, func() message { return &twrite{} })
	msgDotLRegistry.register(msgRwrite, func() message { return &rwrite{} })
	msgDotLRegistry.register(msgTclunk, func() message { return &tclunk{} })
	msgDotLRegistry.register(msgRclunk, func() message { return &rclunk{} })
	msgDotLRegistry.register(msgTremove, func() message { return &tremove{} })
	msgDotLRegistry.register(msgRremove, func() message { return &rremove{} })
	msgDotLRegistry.register(msgTwalkgetattr, func() message { return &twalkgetattr{} })
	msgDotLRegistry.register(msgRwalkgetattr, func() message { return &rwalkgetattr{} })
	msgDotLRegistry.register(msgTucreate, func() message { return &tucreate{} })
	msgDotLRegistry.register(msgRucreate, func() message { return &rucreate{} })
	msgDotLRegistry.register(msgTumkdir, func() message { return &tumkdir{} })
	msgDotLRegistry.register(msgRumkdir, func() message { return &rumkdir{} })
	msgDotLRegistry.register(msgTumknod, func() message { return &tumknod{} })
	msgDotLRegistry.register(msgRumknod, func() message { return &rumknod{} })
	msgDotLRegistry.register(msgTusymlink, func() message { return &tusymlink{} })
	msgDotLRegistry.register(msgRusymlink, func() message { return &rusymlink{} })
}
