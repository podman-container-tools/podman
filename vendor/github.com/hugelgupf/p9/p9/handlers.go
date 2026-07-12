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
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hugelgupf/p9/linux"
)

// newErr returns a new error message from an error.
func newErr(err error) *rlerror {
	return &rlerror{Error: uint32(linux.ExtractErrno(err))}
}

// handler is implemented for server-handled messages.
//
// See server.go for call information.
type handler interface {
	// Handle handles the given message.
	//
	// This may modify the server state. The handle function must return a
	// message which will be sent back to the client. It may be useful to
	// use newErr to automatically extract an error message.
	handle(cs *connState) message
}

// handle implements handler.handle.
func (t *tversion) handle(cs *connState) message {
	// "If the server does not understand the client's version string, it
	// should respond with an Rversion message (not Rerror) with the
	// version string the 7 characters "unknown"".
	//
	// - 9P2000 spec.
	//
	// Makes sense, since there are two different kinds of errors depending on the version.
	unknown := &rversion{
		MSize:   0,
		Version: "unknown",
	}
	if t.MSize == 0 {
		return unknown
	}
	msize := t.MSize
	if t.MSize > maximumLength {
		msize = maximumLength
	}

	reqBaseVersion, reqVersion, ok := parseVersion(t.Version)
	if !ok {
		return unknown
	}
	var baseVersion baseVersion
	var version uint32

	switch reqBaseVersion {
	case version9P2000, version9P2000U:
		return unknown

	case version9P2000L:
		baseVersion = reqBaseVersion
		// The server cannot support newer versions that it doesn't know about.  In this
		// case we return EAGAIN to tell the client to try again with a lower version.
		if reqVersion > highestSupportedVersion {
			version = highestSupportedVersion
		} else {
			version = reqVersion
		}
	}

	// From Tversion(9P): "The server may respond with the client’s version
	// string, or a version string identifying an earlier defined protocol version".
	atomic.StoreUint32(&cs.messageSize, msize)
	atomic.StoreUint32(&cs.version, version)
	// This is not thread-safe. We're changing this into sessions anyway,
	// so who cares.
	cs.baseVersion = baseVersion

	// Initial a pool with msize-shaped buffers.
	cs.readBufPool = sync.Pool{
		New: func() interface{} {
			// These buffers are used for decoding without a payload.
			// We need to return a pointer to avoid unnecessary allocations
			// (see https://staticcheck.io/docs/checks#SA6002).
			b := make([]byte, msize)
			return &b
		},
	}
	// Buffer of zeros.
	cs.pristineZeros = make([]byte, msize)

	return &rversion{
		MSize:   msize,
		Version: versionString(baseVersion, version),
	}
}

// handle implements handler.handle.
func (t *tflush) handle(cs *connState) message {
	cs.WaitTag(t.OldTag)
	return &rflush{}
}

// checkSafeName validates the name and returns nil or returns an error.
func checkSafeName(name string) error {
	if name != "" && !strings.Contains(name, "/") && name != "." && name != ".." {
		return nil
	}
	return linux.EINVAL
}

func clunkHandleXattr(cs *connState, t *tclunk) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyRead(func() error {
		if ref.pendingXattr.op == xattrCreate {
			if len(ref.pendingXattr.buf) != int(ref.pendingXattr.size) {
				return linux.EINVAL
			}
			if ref.pendingXattr.flags == XattrReplace && ref.pendingXattr.size == 0 {
				return ref.file.RemoveXattr(ref.pendingXattr.name)
			}
			return ref.file.SetXattr(ref.pendingXattr.name, ref.pendingXattr.buf, ref.pendingXattr.flags)
		}
		return nil
	}); err != nil {
		return newErr(err)
	}
	return nil
}

// handle implements handler.handle.
func (t *tclunk) handle(cs *connState) message {
	cerr := clunkHandleXattr(cs, t)

	if err := cs.DeleteFID(t.fid); err != nil {
		return newErr(err)
	}
	if cerr != nil {
		return cerr
	}
	return &rclunk{}
}

// handle implements handler.handle.
func (t *tremove) handle(cs *connState) message {
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// Frustratingly, because we can't be guaranteed that a rename is not
	// occurring simultaneously with this removal, we need to acquire the
	// global rename lock for this kind of remove operation to ensure that
	// ref.parent does not change out from underneath us.
	//
	// This is why Tremove is a bad idea, and clients should generally use
	// Tunlinkat. All p9 clients will use Tunlinkat.
	err := ref.safelyGlobal(func() error {
		// Is this a root? Can't remove that.
		if ref.hasParent() {
			return linux.EINVAL
		}

		// N.B. this remove operation is permitted, even if the file is open.
		// See also rename below for reasoning.

		// Is this file already deleted?
		if ref.isDeleted() {
			return linux.EINVAL
		}

		// Retrieve the file's proper name.
		name := ref.parent.pathNode.nameFor(ref)

		// Attempt the removal.
		if err := ref.parent.file.UnlinkAt(name, 0); err != nil {
			return err
		}

		// Mark all relevant fids as deleted. We don't need to lock any
		// individual nodes because we already hold the global lock.
		ref.parent.markChildDeleted(name)
		return nil
	})

	// "The remove request asks the file server both to remove the file
	// represented by fid and to clunk the fid, even if the remove fails."
	//
	// "It is correct to consider remove to be a clunk with the side effect
	// of removing the file if permissions allow."
	// https://swtch.com/plan9port/man/man9/remove.html
	if fidErr := cs.DeleteFID(t.fid); fidErr != nil {
		return newErr(fidErr)
	}
	if err != nil {
		return newErr(err)
	}

	return &rremove{}
}

// handle implements handler.handle.
//
// We don't support authentication, so this just returns ENOSYS.
func (t *tauth) handle(cs *connState) message {
	return newErr(linux.ENOSYS)
}

// handle implements handler.handle.
func (t *tattach) handle(cs *connState) message {
	// Ensure no authentication fid is provided.
	if t.Auth.Authenticationfid != noFID {
		return newErr(linux.EINVAL)
	}

	// Must provide an absolute path.
	if path.IsAbs(t.Auth.AttachName) {
		// Trim off the leading / if the path is absolute. We always
		// treat attach paths as absolute and call attach with the root
		// argument on the server file for clarity.
		t.Auth.AttachName = t.Auth.AttachName[1:]
	}

	// Do the attach on the root.
	sf, err := cs.server.attacher.Attach()
	if err != nil {
		return newErr(err)
	}
	qid, valid, attr, err := sf.GetAttr(AttrMaskAll)
	if err != nil {
		sf.Close() // Drop file.
		return newErr(err)
	}
	if !valid.Mode {
		sf.Close() // Drop file.
		return newErr(linux.EINVAL)
	}

	// Build a transient reference.
	root := &fidRef{
		server:   cs.server,
		parent:   nil,
		file:     sf,
		refs:     1,
		mode:     attr.Mode.FileType(),
		pathNode: cs.server.pathTree,
	}
	defer root.DecRef()

	// Attach the root?
	if len(t.Auth.AttachName) == 0 {
		cs.InsertFID(t.fid, root)
		return &rattach{QID: qid}
	}

	// We want the same traversal checks to apply on attach, so always
	// attach at the root and use the regular walk paths.
	names := strings.Split(t.Auth.AttachName, "/")
	_, newRef, _, _, err := doWalk(cs, root, names, false)
	if err != nil {
		return newErr(err)
	}
	defer newRef.DecRef()

	// Insert the fid.
	cs.InsertFID(t.fid, newRef)
	return &rattach{QID: qid}
}

// CanOpen returns whether this file open can be opened, read and written to.
//
// This includes everything except symlinks and sockets.
func CanOpen(mode FileMode) bool {
	return mode.IsRegular() || mode.IsDir() || mode.IsNamedPipe() || mode.IsBlockDevice() || mode.IsCharacterDevice()
}

// handle implements handler.handle.
func (t *tlopen) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	var (
		qid    QID
		ioUnit uint32
	)
	if err := ref.safelyRead(func() (err error) {
		// Has it been deleted already?
		if ref.isDeleted() {
			return linux.EINVAL
		}

		// Has it been opened already?
		if ref.opened || !CanOpen(ref.mode) {
			return linux.EINVAL
		}

		// Is this an attempt to open a directory as writable? Don't accept.
		if ref.mode.IsDir() && t.Flags.Mode() != ReadOnly {
			return linux.EISDIR
		}

		// Do the open.
		qid, ioUnit, err = ref.file.Open(t.Flags)
		return err
	}); err != nil {
		return newErr(err)
	}

	// Mark file as opened and set open mode.
	ref.opened = true
	ref.openFlags = t.Flags

	return &rlopen{QID: qid, IoUnit: ioUnit}
}

func (t *tlcreate) do(cs *connState, uid UID) (*rlcreate, error) {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return nil, err
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return nil, linux.EBADF
	}
	defer ref.DecRef()

	var (
		nsf    File
		qid    QID
		ioUnit uint32
		newRef *fidRef
	)
	if err := ref.safelyWrite(func() (err error) {
		// Don't allow creation from non-directories or deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Do the create.
		nsf, qid, ioUnit, err = ref.file.Create(t.Name, t.OpenFlags, t.Permissions, uid, t.GID)
		if err != nil {
			return err
		}

		newRef = &fidRef{
			server:    cs.server,
			parent:    ref,
			file:      nsf,
			opened:    true,
			openFlags: t.OpenFlags,
			mode:      ModeRegular,
			pathNode:  ref.pathNode.pathNodeFor(t.Name),
		}
		ref.pathNode.addChild(newRef, t.Name)
		ref.IncRef() // Acquire parent reference.
		return nil
	}); err != nil {
		return nil, err
	}

	// Replace the fid reference.
	cs.InsertFID(t.fid, newRef)

	return &rlcreate{rlopen: rlopen{QID: qid, IoUnit: ioUnit}}, nil
}

// handle implements handler.handle.
func (t *tlcreate) handle(cs *connState) message {
	rlcreate, err := t.do(cs, NoUID)
	if err != nil {
		return newErr(err)
	}
	return rlcreate
}

// handle implements handler.handle.
func (t *tsymlink) handle(cs *connState) message {
	rsymlink, err := t.do(cs, NoUID)
	if err != nil {
		return newErr(err)
	}
	return rsymlink
}

func (t *tsymlink) do(cs *connState, uid UID) (*rsymlink, error) {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return nil, err
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return nil, linux.EBADF
	}
	defer ref.DecRef()

	var qid QID
	if err := ref.safelyWrite(func() (err error) {
		// Don't allow symlinks from non-directories or deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Do the symlink.
		qid, err = ref.file.Symlink(t.Target, t.Name, uid, t.GID)
		return err
	}); err != nil {
		return nil, err
	}

	return &rsymlink{QID: qid}, nil
}

// handle implements handler.handle.
func (t *tlink) handle(cs *connState) message {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return newErr(err)
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// Lookup the other fid.
	refTarget, ok := cs.LookupFID(t.Target)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer refTarget.DecRef()

	if err := ref.safelyWrite(func() (err error) {
		// Don't allow create links from non-directories or deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Do the link.
		return ref.file.Link(refTarget.file, t.Name)
	}); err != nil {
		return newErr(err)
	}

	return &rlink{}
}

// handle implements handler.handle.
func (t *trenameat) handle(cs *connState) message {
	// Don't allow complex names.
	if err := checkSafeName(t.OldName); err != nil {
		return newErr(err)
	}
	if err := checkSafeName(t.NewName); err != nil {
		return newErr(err)
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.OldDirectory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// Lookup the other fid.
	refTarget, ok := cs.LookupFID(t.NewDirectory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer refTarget.DecRef()

	// Perform the rename holding the global lock.
	if err := ref.safelyGlobal(func() (err error) {
		// Don't allow renaming across deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() || refTarget.isDeleted() || !refTarget.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Is this the same file? If yes, short-circuit and return success.
		if ref.pathNode == refTarget.pathNode && t.OldName == t.NewName {
			return nil
		}

		// Attempt the actual rename.
		if err := ref.file.RenameAt(t.OldName, refTarget.file, t.NewName); err != nil {
			return err
		}

		// Update the path tree.
		ref.renameChildTo(t.OldName, refTarget, t.NewName)
		return nil
	}); err != nil {
		return newErr(err)
	}

	return &rrenameat{}
}

// handle implements handler.handle.
func (t *tunlinkat) handle(cs *connState) message {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return newErr(err)
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyWrite(func() (err error) {
		// Don't allow deletion from non-directories or deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Before we do the unlink itself, we need to ensure that there
		// are no operations in flight on associated path node. The
		// child's path node lock must be held to ensure that the
		// unlinkat marking the child deleted below is atomic with
		// respect to any other read or write operations.
		//
		// This is one case where we have a lock ordering issue, but
		// since we always acquire deeper in the hierarchy, we know
		// that we are free of lock cycles.
		childPathNode := ref.pathNode.pathNodeFor(t.Name)
		childPathNode.opMu.Lock()
		defer childPathNode.opMu.Unlock()

		// Do the unlink.
		err = ref.file.UnlinkAt(t.Name, t.Flags)
		if err != nil {
			return err
		}

		// Mark the path as deleted.
		ref.markChildDeleted(t.Name)
		return nil
	}); err != nil {
		return newErr(err)
	}

	return &runlinkat{}
}

// handle implements handler.handle.
func (t *trename) handle(cs *connState) message {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return newErr(err)
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// Lookup the target.
	refTarget, ok := cs.LookupFID(t.Directory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer refTarget.DecRef()

	if err := ref.safelyGlobal(func() (err error) {
		// Don't allow a root rename.
		if ref.hasParent() {
			return linux.EINVAL
		}

		// Don't allow renaming deleting entries, or target non-directories.
		if ref.isDeleted() || refTarget.isDeleted() || !refTarget.mode.IsDir() {
			return linux.EINVAL
		}

		// If the parent is deleted, but we not, something is seriously wrong.
		// It's fail to die at this point with an assertion failure.
		if ref.parent.isDeleted() {
			panic(fmt.Sprintf("parent %+v deleted, child %+v is not", ref.parent, ref))
		}

		// N.B. The rename operation is allowed to proceed on open files. It
		// does impact the state of its parent, but this is merely a sanity
		// check in any case, and the operation is safe. There may be other
		// files corresponding to the same path that are renamed anyways.

		// Check for the exact same file and short-circuit.
		oldName := ref.parent.pathNode.nameFor(ref)
		if ref.parent.pathNode == refTarget.pathNode && oldName == t.Name {
			return nil
		}

		// Call the rename method on the parent.
		if err := ref.parent.file.RenameAt(oldName, refTarget.file, t.Name); err != nil {
			return err
		}

		// Update the path tree.
		ref.parent.renameChildTo(oldName, refTarget, t.Name)
		return nil
	}); err != nil {
		return newErr(err)
	}

	return &rrename{}
}

// handle implements handler.handle.
func (t *treadlink) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	var target string
	if err := ref.safelyRead(func() (err error) {
		// Don't allow readlink on deleted files. There is no need to
		// check if this file is opened because symlinks cannot be
		// opened.
		if ref.isDeleted() || !ref.mode.IsSymlink() {
			return linux.EINVAL
		}

		// Do the read.
		target, err = ref.file.Readlink()
		return err
	}); err != nil {
		return newErr(err)
	}

	return &rreadlink{target}
}

// handle implements handler.handle.
func (t *tread) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// Constrain the size of the read buffer.
	if int(t.Count) > int(maximumLength) {
		return newErr(linux.ENOBUFS)
	}

	var n int
	data := cs.readBufPool.Get().(*[]byte)
	// Retain a reference to the full length of the buffer.
	dataBuf := (*data)
	if err := ref.safelyRead(func() (err error) {
		switch ref.pendingXattr.op {
		case xattrNone:
			// Has it been opened already?
			if !ref.opened {
				return linux.EINVAL
			}

			// Can it be read? Check permissions.
			if ref.openFlags&OpenFlagsModeMask == WriteOnly {
				return linux.EPERM
			}

			n, err = ref.file.ReadAt(dataBuf[:t.Count], int64(t.Offset))
			return err

		case xattrWalk:
			// Make sure we do not pass an empty buffer to GetXattr or ListXattrs.
			// Both of them will return the required buffer length if
			// the input buffer has length 0.
			// tread means the caller already knows the required buffer length
			// and wants to get the attribute value.
			if t.Count == 0 {
				if ref.pendingXattr.size == 0 {
					// the provided buffer has length 0 and
					// the attribute value is also empty.
					return nil
				}
				// buffer too small.
				return linux.EINVAL
			}

			if t.Offset+uint64(t.Count) > uint64(len(ref.pendingXattr.buf)) {
				return linux.EINVAL
			}

			n = copy(dataBuf[:t.Count], ref.pendingXattr.buf[t.Offset:])
			return nil
		default:
			return linux.EINVAL
		}
	}); err != nil && !errors.Is(err, io.EOF) {
		return newErr(err)
	}

	return &rreadServerPayloader{
		rread: rread{
			Data: dataBuf[:n],
		},
		cs:         cs,
		fullBuffer: dataBuf,
	}
}

// handle implements handler.handle.
func (t *twrite) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	var n int
	if err := ref.safelyRead(func() (err error) {
		switch ref.pendingXattr.op {
		case xattrNone:
			// Has it been opened already?
			if !ref.opened {
				return linux.EINVAL
			}

			// Can it be written? Check permissions.
			if ref.openFlags&OpenFlagsModeMask == ReadOnly {
				return linux.EPERM
			}

			n, err = ref.file.WriteAt(t.Data, int64(t.Offset))

		case xattrCreate:
			if uint64(len(ref.pendingXattr.buf)) != t.Offset {
				return linux.EINVAL
			}
			if t.Offset+uint64(len(t.Data)) > ref.pendingXattr.size {
				return linux.EINVAL
			}
			ref.pendingXattr.buf = append(ref.pendingXattr.buf, t.Data...)
			n = len(t.Data)

		default:
			return linux.EINVAL
		}
		return err
	}); err != nil {
		return newErr(err)
	}

	return &rwrite{Count: uint32(n)}
}

// handle implements handler.handle.
func (t *tmknod) handle(cs *connState) message {
	rmknod, err := t.do(cs, NoUID)
	if err != nil {
		return newErr(err)
	}
	return rmknod
}

func (t *tmknod) do(cs *connState, uid UID) (*rmknod, error) {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return nil, err
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return nil, linux.EBADF
	}
	defer ref.DecRef()

	var qid QID
	if err := ref.safelyWrite(func() (err error) {
		// Don't allow mknod on deleted files.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Do the mknod.
		qid, err = ref.file.Mknod(t.Name, t.Mode, t.Major, t.Minor, uid, t.GID)
		return err
	}); err != nil {
		return nil, err
	}

	return &rmknod{QID: qid}, nil
}

// handle implements handler.handle.
func (t *tmkdir) handle(cs *connState) message {
	rmkdir, err := t.do(cs, NoUID)
	if err != nil {
		return newErr(err)
	}
	return rmkdir
}

func (t *tmkdir) do(cs *connState, uid UID) (*rmkdir, error) {
	// Don't allow complex names.
	if err := checkSafeName(t.Name); err != nil {
		return nil, err
	}

	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return nil, linux.EBADF
	}
	defer ref.DecRef()

	var qid QID
	if err := ref.safelyWrite(func() (err error) {
		// Don't allow mkdir on deleted files.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Not allowed on open directories.
		if ref.opened {
			return linux.EINVAL
		}

		// Do the mkdir.
		qid, err = ref.file.Mkdir(t.Name, t.Permissions, uid, t.GID)
		return err
	}); err != nil {
		return nil, err
	}

	return &rmkdir{QID: qid}, nil
}

// handle implements handler.handle.
func (t *tgetattr) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	// We allow getattr on deleted files. Depending on the backing
	// implementation, it's possible that races exist that might allow
	// fetching attributes of other files. But we need to generally allow
	// refreshing attributes and this is a minor leak, if at all.

	var (
		qid   QID
		valid AttrMask
		attr  Attr
	)
	if err := ref.safelyRead(func() (err error) {
		qid, valid, attr, err = ref.file.GetAttr(t.AttrMask)
		return err
	}); err != nil {
		return newErr(err)
	}

	return &rgetattr{QID: qid, Valid: valid, Attr: attr}
}

// handle implements handler.handle.
func (t *tsetattr) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyWrite(func() error {
		// We don't allow setattr on files that have been deleted.
		// This might be technically incorrect, as it's possible that
		// there were multiple links and you can still change the
		// corresponding inode information.
		if ref.isDeleted() {
			return linux.EINVAL
		}

		// Set the attributes.
		return ref.file.SetAttr(t.Valid, t.SetAttr)
	}); err != nil {
		return newErr(err)
	}

	return &rsetattr{}
}

// handle implements handler.handle.
func (t *txattrwalk) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	size := 0
	if err := ref.safelyRead(func() error {
		if ref.isDeleted() {
			return linux.EINVAL
		}
		var buf []byte
		var err error
		if len(t.Name) > 0 {
			buf, err = ref.file.GetXattr(t.Name)
		} else {
			var xattrs []string
			xattrs, err = ref.file.ListXattrs()
			if err == nil {
				buf = []byte(strings.Join(xattrs, "\000") + "\000")
			}
		}
		if err != nil || uint32(len(buf)) > maximumLength {
			return linux.EINVAL
		}
		size = len(buf)
		newRef := &fidRef{
			server: cs.server,
			file:   ref.file,
			pendingXattr: pendingXattr{
				op:   xattrWalk,
				name: t.Name,
				size: uint64(size),
				buf:  buf,
			},
			pathNode: ref.pathNode,
		}
		cs.InsertFID(t.newFID, newRef)
		return nil
	}); err != nil {
		return newErr(err)
	}
	return &rxattrwalk{Size: uint64(size)}
}

// handle implements handler.handle.
func (t *txattrcreate) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()
	if err := ref.safelyWrite(func() error {
		if ref.isDeleted() {
			return linux.EINVAL
		}
		ref.pendingXattr = pendingXattr{
			op:    xattrCreate,
			name:  t.Name,
			size:  t.AttrSize,
			flags: XattrFlags(t.Flags),
		}
		return nil
	}); err != nil {
		return newErr(err)
	}
	return &rxattrcreate{}
}

// handle implements handler.handle.
func (t *treaddir) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.Directory)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	var entries []Dirent
	if err := ref.safelyRead(func() (err error) {
		// Don't allow reading deleted directories.
		if ref.isDeleted() || !ref.mode.IsDir() {
			return linux.EINVAL
		}

		// Has it been opened already?
		if !ref.opened {
			return linux.EINVAL
		}

		// Read the entries.
		entries, err = ref.file.Readdir(t.Offset, t.Count)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	}); err != nil {
		return newErr(err)
	}

	return &rreaddir{Count: t.Count, Entries: entries}
}

// handle implements handler.handle.
func (t *tfsync) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyRead(func() (err error) {
		// Has it been opened already?
		if !ref.opened {
			return linux.EINVAL
		}

		// Perform the sync.
		return ref.file.FSync()
	}); err != nil {
		return newErr(err)
	}

	return &rfsync{}
}

// handle implements handler.handle.
func (t *tstatfs) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	st, err := ref.file.StatFS()
	if err != nil {
		return newErr(err)
	}

	return &rstatfs{st}
}

// handle implements handler.handle.
func (t *tlock) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	status, err := ref.file.Lock(int(t.PID), t.Type, t.Flags, t.Start, t.Length, t.Client)
	if err != nil {
		return newErr(err)
	}
	return &rlock{Status: status}
}

// walkOne walks zero or one path elements.
//
// The slice passed as qids is append and returned.
func walkOne(qids []QID, from File, names []string, getattr bool) ([]QID, File, AttrMask, Attr, error) {
	nwname := len(names)
	if nwname > 1 {
		// We require exactly zero or one elements.
		return nil, nil, AttrMask{}, Attr{}, linux.EINVAL
	}
	var (
		localQIDs []QID
		sf        File
		valid     AttrMask
		attr      Attr
		err       error
	)
	switch {
	case getattr:
		localQIDs, sf, valid, attr, err = from.WalkGetAttr(names)
		// Can't put fallthrough in the if because Go.
		if !errors.Is(err, linux.ENOSYS) {
			break
		}
		fallthrough
	default:
		localQIDs, sf, err = from.Walk(names)
		if err != nil {
			// No way to walk this element.
			break
		}
		if getattr {
			_, valid, attr, err = sf.GetAttr(AttrMaskAll)
			if err != nil {
				// Don't leak the file.
				sf.Close()
			}
		}
	}
	if err != nil {
		// Error walking, don't return anything.
		return nil, nil, AttrMask{}, Attr{}, err
	}
	if nwname == 1 && len(localQIDs) != 1 {
		// Expected a single QID.
		sf.Close()
		return nil, nil, AttrMask{}, Attr{}, linux.EINVAL
	}
	return append(qids, localQIDs...), sf, valid, attr, nil
}

// doWalk walks from a given fidRef.
//
// This enforces that all intermediate nodes are walkable (directories). The
// fidRef returned (newRef) has a reference associated with it that is now
// owned by the caller and must be handled appropriately.
func doWalk(cs *connState, ref *fidRef, names []string, getattr bool) (qids []QID, newRef *fidRef, valid AttrMask, attr Attr, err error) {
	// Check the names.
	for _, name := range names {
		err = checkSafeName(name)
		if err != nil {
			return
		}
	}

	// validate anything since this is always permitted.
	if len(names) == 0 {
		var sf File // Temporary.
		if err := ref.maybeParent().safelyRead(func() (err error) {
			// Clone the single element.
			qids, sf, valid, attr, err = walkOne(nil, ref.file, nil, getattr)
			if err != nil {
				return err
			}

			newRef = &fidRef{
				server:   cs.server,
				parent:   ref.parent,
				file:     sf,
				mode:     ref.mode,
				pathNode: ref.pathNode,
			}
			if !ref.hasParent() {
				if !newRef.isDeleted() {
					// Add only if a non-root node; the same node.
					ref.parent.pathNode.addChild(newRef, ref.parent.pathNode.nameFor(ref))
				}
				ref.parent.IncRef() // Acquire parent reference.
			}
			// doWalk returns a reference.
			newRef.IncRef()
			return nil
		}); err != nil {
			return nil, nil, AttrMask{}, Attr{}, err
		}

		// Do not return the new QID.
		// walk(5) "nwqid will always be less than or equal to nwname"
		return nil, newRef, valid, attr, nil
	}

	// Do the walk, one element at a time.
	walkRef := ref
	walkRef.IncRef()
	for i := 0; i < len(names); i++ {
		// We won't allow beyond past symlinks; stop here if this isn't
		// a proper directory and we have additional paths to walk.
		if !walkRef.mode.IsDir() {
			walkRef.DecRef() // Drop walk reference; no lock required.
			return nil, nil, AttrMask{}, Attr{}, linux.EINVAL
		}

		var sf File // Temporary.
		if err := walkRef.safelyRead(func() (err error) {
			// It is not safe to walk on a deleted directory. It
			// could have been replaced with a malicious symlink.
			if walkRef.isDeleted() {
				// Fail this operation as the result will not
				// be meaningful if walkRef is deleted.
				return linux.ENOENT
			}

			// Pass getattr = true to walkOne since we need the file type for
			// newRef.
			qids, sf, valid, attr, err = walkOne(qids, walkRef.file, names[i:i+1], true)
			if err != nil {
				return err
			}

			// Note that we don't need to acquire a lock on any of
			// these individual instances. That's because they are
			// not actually addressable via a fid. They are
			// anonymous. They exist in the tree for tracking
			// purposes.
			newRef := &fidRef{
				server:   cs.server,
				parent:   walkRef,
				file:     sf,
				mode:     attr.Mode.FileType(),
				pathNode: walkRef.pathNode.pathNodeFor(names[i]),
			}
			walkRef.pathNode.addChild(newRef, names[i])
			// We allow our walk reference to become the new parent
			// reference here and so we don't IncRef. Instead, just
			// set walkRef to the newRef above and acquire a new
			// walk reference.
			walkRef = newRef
			walkRef.IncRef()
			return nil
		}); err != nil {
			walkRef.DecRef() // Drop the old walkRef.
			return nil, nil, AttrMask{}, Attr{}, err
		}
	}

	// Success.
	return qids, walkRef, valid, attr, nil
}

// handle implements handler.handle.
func (t *twalk) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyRead(func() error {
		// Has it been opened already?
		//
		// That as OK as long as newFID is different. Note this
		// violates the spec, but the Linux client does too, so we have
		// little choice.
		if ref.opened && t.fid == t.newFID {
			return linux.EBUSY
		}
		return nil
	}); err != nil {
		return newErr(err)
	}

	// Is this an empty list? Handle specially. We don't actually need to
	// Do the walk.
	qids, newRef, _, _, err := doWalk(cs, ref, t.Names, false)
	if err != nil {
		return newErr(err)
	}
	defer newRef.DecRef()

	// Install the new fid.
	cs.InsertFID(t.newFID, newRef)
	return &rwalk{QIDs: qids}
}

// handle implements handler.handle.
func (t *twalkgetattr) handle(cs *connState) message {
	// Lookup the fid.
	ref, ok := cs.LookupFID(t.fid)
	if !ok {
		return newErr(linux.EBADF)
	}
	defer ref.DecRef()

	if err := ref.safelyRead(func() error {
		// Has it been opened already?
		//
		// That as OK as long as newFID is different. Note this
		// violates the spec, but the Linux client does too, so we have
		// little choice.
		if ref.opened && t.fid == t.newFID {
			return linux.EBUSY
		}
		return nil
	}); err != nil {
		return newErr(err)
	}

	// Is this an empty list? Handle specially. We don't actually need to
	// Do the walk.
	qids, newRef, valid, attr, err := doWalk(cs, ref, t.Names, true)
	if err != nil {
		return newErr(err)
	}
	defer newRef.DecRef()

	// Install the new fid.
	cs.InsertFID(t.newFID, newRef)
	return &rwalkgetattr{QIDs: qids, Valid: valid, Attr: attr}
}

// handle implements handler.handle.
func (t *tucreate) handle(cs *connState) message {
	rlcreate, err := t.tlcreate.do(cs, t.UID)
	if err != nil {
		return newErr(err)
	}
	return &rucreate{*rlcreate}
}

// handle implements handler.handle.
func (t *tumkdir) handle(cs *connState) message {
	rmkdir, err := t.tmkdir.do(cs, t.UID)
	if err != nil {
		return newErr(err)
	}
	return &rumkdir{*rmkdir}
}

// handle implements handler.handle.
func (t *tusymlink) handle(cs *connState) message {
	rsymlink, err := t.tsymlink.do(cs, t.UID)
	if err != nil {
		return newErr(err)
	}
	return &rusymlink{*rsymlink}
}

// handle implements handler.handle.
func (t *tumknod) handle(cs *connState) message {
	rmknod, err := t.tmknod.do(cs, t.UID)
	if err != nil {
		return newErr(err)
	}
	return &rumknod{*rmknod}
}
