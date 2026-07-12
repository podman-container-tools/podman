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
	"io"
	"runtime"
	"sync/atomic"

	"github.com/hugelgupf/p9/linux"
)

// Attach attaches to a server.
//
// Note that authentication is not currently supported.
func (c *Client) Attach(name string) (File, error) {
	id, ok := c.fidPool.Get()
	if !ok {
		return nil, ErrOutOfFIDs
	}

	rattach := rattach{}
	if err := c.sendRecv(&tattach{fid: fid(id), Auth: tauth{AttachName: name, Authenticationfid: noFID, UID: NoUID}}, &rattach); err != nil {
		c.fidPool.Put(id)
		return nil, err
	}

	return c.newFile(fid(id)), nil
}

// newFile returns a new client file.
func (c *Client) newFile(fid fid) *clientFile {
	cf := &clientFile{
		client: c,
		fid:    fid,
	}

	// Make sure the file is closed.
	runtime.SetFinalizer(cf, (*clientFile).Close)

	return cf
}

// clientFile is provided to clients.
//
// This proxies all of the interfaces found in file.go.
type clientFile struct {
	// client is the originating client.
	client *Client

	// fid is the fid for this file.
	fid fid

	// closed indicates whether this file has been closed.
	closed uint32
}

// SetXattr implements p9.File.SetXattr.
func (c *clientFile) SetXattr(attr string, data []byte, flags XattrFlags) error {
	return linux.ENOSYS
}

// RemoveXattr implements p9.File.RemoveXattr.
func (c *clientFile) RemoveXattr(attr string) error {
	return linux.ENOSYS
}

// GetXattr implements p9.File.GetXattr.
func (c *clientFile) GetXattr(attr string) ([]byte, error) {
	return nil, linux.ENOSYS
}

// ListXattrs implements p9.File.ListXattrs.
func (c *clientFile) ListXattrs() ([]string, error) {
	return nil, linux.ENOSYS
}

// Walk implements File.Walk.
func (c *clientFile) Walk(names []string) ([]QID, File, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return nil, nil, linux.EBADF
	}

	id, ok := c.client.fidPool.Get()
	if !ok {
		return nil, nil, ErrOutOfFIDs
	}

	rwalk := rwalk{}
	if err := c.client.sendRecv(&twalk{fid: c.fid, newFID: fid(id), Names: names}, &rwalk); err != nil {
		c.client.fidPool.Put(id)
		return nil, nil, err
	}

	// Return a new client file.
	return rwalk.QIDs, c.client.newFile(fid(id)), nil
}

// WalkGetAttr implements File.WalkGetAttr.
func (c *clientFile) WalkGetAttr(components []string) ([]QID, File, AttrMask, Attr, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return nil, nil, AttrMask{}, Attr{}, linux.EBADF
	}

	if !versionSupportsTwalkgetattr(c.client.version) {
		qids, file, err := c.Walk(components)
		if err != nil {
			return nil, nil, AttrMask{}, Attr{}, err
		}
		_, valid, attr, err := file.GetAttr(AttrMaskAll)
		if err != nil {
			file.Close()
			return nil, nil, AttrMask{}, Attr{}, err
		}
		return qids, file, valid, attr, nil
	}

	id, ok := c.client.fidPool.Get()
	if !ok {
		return nil, nil, AttrMask{}, Attr{}, ErrOutOfFIDs
	}

	rwalkgetattr := rwalkgetattr{}
	if err := c.client.sendRecv(&twalkgetattr{fid: c.fid, newFID: fid(id), Names: components}, &rwalkgetattr); err != nil {
		c.client.fidPool.Put(id)
		return nil, nil, AttrMask{}, Attr{}, err
	}

	// Return a new client file.
	return rwalkgetattr.QIDs, c.client.newFile(fid(id)), rwalkgetattr.Valid, rwalkgetattr.Attr, nil
}

// StatFS implements File.StatFS.
func (c *clientFile) StatFS() (FSStat, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return FSStat{}, linux.EBADF
	}

	rstatfs := rstatfs{}
	if err := c.client.sendRecv(&tstatfs{fid: c.fid}, &rstatfs); err != nil {
		return FSStat{}, err
	}

	return rstatfs.FSStat, nil
}

// FSync implements File.FSync.
func (c *clientFile) FSync() error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	return c.client.sendRecv(&tfsync{fid: c.fid}, &rfsync{})
}

// GetAttr implements File.GetAttr.
func (c *clientFile) GetAttr(req AttrMask) (QID, AttrMask, Attr, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return QID{}, AttrMask{}, Attr{}, linux.EBADF
	}

	rgetattr := rgetattr{}
	if err := c.client.sendRecv(&tgetattr{fid: c.fid, AttrMask: req}, &rgetattr); err != nil {
		return QID{}, AttrMask{}, Attr{}, err
	}

	return rgetattr.QID, rgetattr.Valid, rgetattr.Attr, nil
}

// SetAttr implements File.SetAttr.
func (c *clientFile) SetAttr(valid SetAttrMask, attr SetAttr) error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	return c.client.sendRecv(&tsetattr{fid: c.fid, Valid: valid, SetAttr: attr}, &rsetattr{})
}

// Lock implements File.Lock
func (c *clientFile) Lock(pid int, locktype LockType, flags LockFlags, start, length uint64, client string) (LockStatus, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return LockStatusError, linux.EBADF
	}

	r := rlock{}
	err := c.client.sendRecv(&tlock{
		Type:   locktype,
		Flags:  flags,
		Start:  start,
		Length: length,
		PID:    int32(pid),
		Client: client,
	}, &r)
	return r.Status, err
}

// Remove implements File.Remove.
//
// N.B. This method is no longer part of the file interface and should be
// considered deprecated.
func (c *clientFile) Remove() error {
	// Avoid double close.
	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return linux.EBADF
	}
	runtime.SetFinalizer(c, nil)

	// Send the remove message.
	if err := c.client.sendRecv(&tremove{fid: c.fid}, &rremove{}); err != nil {
		return err
	}

	// "It is correct to consider remove to be a clunk with the side effect
	// of removing the file if permissions allow."
	// https://swtch.com/plan9port/man/man9/remove.html

	// Return the fid to the pool.
	c.client.fidPool.Put(uint64(c.fid))
	return nil
}

// Close implements File.Close.
func (c *clientFile) Close() error {
	// Avoid double close.
	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return linux.EBADF
	}
	runtime.SetFinalizer(c, nil)

	// Send the close message.
	if err := c.client.sendRecv(&tclunk{fid: c.fid}, &rclunk{}); err != nil {
		// If an error occurred, we toss away the fid. This isn't ideal,
		// but I'm not sure what else makes sense in this context.
		return err
	}

	// Return the fid to the pool.
	c.client.fidPool.Put(uint64(c.fid))
	return nil
}

// Open implements File.Open.
func (c *clientFile) Open(flags OpenFlags) (QID, uint32, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return QID{}, 0, linux.EBADF
	}

	rlopen := rlopen{}
	if err := c.client.sendRecv(&tlopen{fid: c.fid, Flags: flags}, &rlopen); err != nil {
		return QID{}, 0, err
	}

	return rlopen.QID, rlopen.IoUnit, nil
}

// chunk applies fn to p in chunkSize-sized chunks until fn returns a partial result, p is
// exhausted, or an error is encountered (which may be io.EOF).
func chunk(chunkSize uint32, fn func([]byte, int64) (int, error), p []byte, offset int64) (int, error) {
	// Some p9.Clients depend on executing fn on zero-byte buffers. Handle this
	// as a special case (normally it is fine to short-circuit and return (0, nil)).
	if len(p) == 0 {
		return fn(p, offset)
	}

	// total is the cumulative bytes processed.
	var total int
	for {
		var n int
		var err error

		// We're done, don't bother trying to do anything more.
		if total == len(p) {
			return total, nil
		}

		// Apply fn to a chunkSize-sized (or less) chunk of p.
		if len(p) < total+int(chunkSize) {
			n, err = fn(p[total:], offset)
		} else {
			n, err = fn(p[total:total+int(chunkSize)], offset)
		}
		total += n
		offset += int64(n)

		// Return whatever we have processed if we encounter an error. This error
		// could be io.EOF.
		if err != nil {
			return total, err
		}

		// Did we get a partial result? If so, return it immediately.
		if n < int(chunkSize) {
			return total, nil
		}

		// If we received more bytes than we ever requested, this is a problem.
		if total > len(p) {
			panic(fmt.Sprintf("bytes completed (%d)) > requested (%d)", total, len(p)))
		}
	}
}

// ReadAt proxies File.ReadAt.
func (c *clientFile) ReadAt(p []byte, offset int64) (int, error) {
	return chunk(c.client.payloadSize, c.readAt, p, offset)
}

func (c *clientFile) readAt(p []byte, offset int64) (int, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return 0, linux.EBADF
	}

	rread := rread{Data: p}
	if err := c.client.sendRecv(&tread{fid: c.fid, Offset: uint64(offset), Count: uint32(len(p))}, &rread); err != nil {
		return 0, err
	}

	// The message may have been truncated, or for some reason a new buffer
	// allocated. This isn't the common path, but we make sure that if the
	// payload has changed we copy it. See transport.go for more information.
	if len(p) > 0 && len(rread.Data) > 0 && &rread.Data[0] != &p[0] {
		copy(p, rread.Data)
	}

	// io.EOF is not an error that a p9 server can return. Use POSIX semantics to
	// return io.EOF manually: zero bytes were returned and a non-zero buffer was used.
	if len(rread.Data) == 0 && len(p) > 0 {
		return 0, io.EOF
	}

	return len(rread.Data), nil
}

// WriteAt proxies File.WriteAt.
func (c *clientFile) WriteAt(p []byte, offset int64) (int, error) {
	return chunk(c.client.payloadSize, c.writeAt, p, offset)
}

func (c *clientFile) writeAt(p []byte, offset int64) (int, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return 0, linux.EBADF
	}

	rwrite := rwrite{}
	if err := c.client.sendRecv(&twrite{fid: c.fid, Offset: uint64(offset), Data: p}, &rwrite); err != nil {
		return 0, err
	}

	return int(rwrite.Count), nil
}

// Rename implements File.Rename.
func (c *clientFile) Rename(dir File, name string) error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	clientDir, ok := dir.(*clientFile)
	if !ok {
		return linux.EBADF
	}

	return c.client.sendRecv(&trename{fid: c.fid, Directory: clientDir.fid, Name: name}, &rrename{})
}

// Create implements File.Create.
func (c *clientFile) Create(name string, openFlags OpenFlags, permissions FileMode, uid UID, gid GID) (File, QID, uint32, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return nil, QID{}, 0, linux.EBADF
	}

	msg := tlcreate{
		fid:         c.fid,
		Name:        name,
		OpenFlags:   openFlags,
		Permissions: permissions,
		GID:         NoGID,
	}

	if versionSupportsTucreation(c.client.version) {
		msg.GID = gid
		rucreate := rucreate{}
		if err := c.client.sendRecv(&tucreate{tlcreate: msg, UID: uid}, &rucreate); err != nil {
			return nil, QID{}, 0, err
		}
		return c, rucreate.QID, rucreate.IoUnit, nil
	}

	rlcreate := rlcreate{}
	if err := c.client.sendRecv(&msg, &rlcreate); err != nil {
		return nil, QID{}, 0, err
	}

	return c, rlcreate.QID, rlcreate.IoUnit, nil
}

// Mkdir implements File.Mkdir.
func (c *clientFile) Mkdir(name string, permissions FileMode, uid UID, gid GID) (QID, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return QID{}, linux.EBADF
	}

	msg := tmkdir{
		Directory:   c.fid,
		Name:        name,
		Permissions: permissions,
		GID:         NoGID,
	}

	if versionSupportsTucreation(c.client.version) {
		msg.GID = gid
		rumkdir := rumkdir{}
		if err := c.client.sendRecv(&tumkdir{tmkdir: msg, UID: uid}, &rumkdir); err != nil {
			return QID{}, err
		}
		return rumkdir.QID, nil
	}

	rmkdir := rmkdir{}
	if err := c.client.sendRecv(&msg, &rmkdir); err != nil {
		return QID{}, err
	}

	return rmkdir.QID, nil
}

// Symlink implements File.Symlink.
func (c *clientFile) Symlink(oldname string, newname string, uid UID, gid GID) (QID, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return QID{}, linux.EBADF
	}

	msg := tsymlink{
		Directory: c.fid,
		Name:      newname,
		Target:    oldname,
		GID:       NoGID,
	}

	if versionSupportsTucreation(c.client.version) {
		msg.GID = gid
		rusymlink := rusymlink{}
		if err := c.client.sendRecv(&tusymlink{tsymlink: msg, UID: uid}, &rusymlink); err != nil {
			return QID{}, err
		}
		return rusymlink.QID, nil
	}

	rsymlink := rsymlink{}
	if err := c.client.sendRecv(&msg, &rsymlink); err != nil {
		return QID{}, err
	}

	return rsymlink.QID, nil
}

// Link implements File.Link.
func (c *clientFile) Link(target File, newname string) error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	targetFile, ok := target.(*clientFile)
	if !ok {
		return linux.EBADF
	}

	return c.client.sendRecv(&tlink{Directory: c.fid, Name: newname, Target: targetFile.fid}, &rlink{})
}

// Mknod implements File.Mknod.
func (c *clientFile) Mknod(name string, mode FileMode, major uint32, minor uint32, uid UID, gid GID) (QID, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return QID{}, linux.EBADF
	}

	msg := tmknod{
		Directory: c.fid,
		Name:      name,
		Mode:      mode,
		Major:     major,
		Minor:     minor,
		GID:       NoGID,
	}

	if versionSupportsTucreation(c.client.version) {
		msg.GID = gid
		rumknod := rumknod{}
		if err := c.client.sendRecv(&tumknod{tmknod: msg, UID: uid}, &rumknod); err != nil {
			return QID{}, err
		}
		return rumknod.QID, nil
	}

	rmknod := rmknod{}
	if err := c.client.sendRecv(&msg, &rmknod); err != nil {
		return QID{}, err
	}

	return rmknod.QID, nil
}

// RenameAt implements File.RenameAt.
func (c *clientFile) RenameAt(oldname string, newdir File, newname string) error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	clientNewDir, ok := newdir.(*clientFile)
	if !ok {
		return linux.EBADF
	}

	return c.client.sendRecv(&trenameat{OldDirectory: c.fid, OldName: oldname, NewDirectory: clientNewDir.fid, NewName: newname}, &rrenameat{})
}

// UnlinkAt implements File.UnlinkAt.
func (c *clientFile) UnlinkAt(name string, flags uint32) error {
	if atomic.LoadUint32(&c.closed) != 0 {
		return linux.EBADF
	}

	return c.client.sendRecv(&tunlinkat{Directory: c.fid, Name: name, Flags: flags}, &runlinkat{})
}

// Readdir implements File.Readdir.
func (c *clientFile) Readdir(offset uint64, count uint32) (Dirents, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return nil, linux.EBADF
	}

	rreaddir := rreaddir{}
	if err := c.client.sendRecv(&treaddir{Directory: c.fid, Offset: offset, Count: count}, &rreaddir); err != nil {
		return nil, err
	}

	return rreaddir.Entries, nil
}

// Readlink implements File.Readlink.
func (c *clientFile) Readlink() (string, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return "", linux.EBADF
	}

	rreadlink := rreadlink{}
	if err := c.client.sendRecv(&treadlink{fid: c.fid}, &rreadlink); err != nil {
		return "", err
	}

	return rreadlink.Target, nil
}

// Renamed implements File.Renamed.
func (c *clientFile) Renamed(newDir File, newName string) {}
