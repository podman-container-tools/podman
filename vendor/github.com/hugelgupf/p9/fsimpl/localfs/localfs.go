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

// Package localfs exposes the host's local file system as a p9.File.
package localfs

import (
	"os"
	"path"
	"path/filepath"

	"github.com/hugelgupf/p9/fsimpl/templatefs"
	"github.com/hugelgupf/p9/internal"
	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
)

type attacher struct {
	root string
}

var (
	_ p9.Attacher = &attacher{}
)

// RootAttacher attaches at the host file system's root.
func RootAttacher() p9.Attacher {
	return &attacher{root: "/"}
}

// Attacher returns an attacher that exposes files under root.
func Attacher(root string) p9.Attacher {
	if len(root) == 0 {
		root = "/"
	}
	return &attacher{root: root}
}

// Attach implements p9.Attacher.Attach.
func (a *attacher) Attach() (p9.File, error) {
	umask(0)
	return &Local{path: a.root}, nil
}

// Local is a p9.File.
type Local struct {
	p9.DefaultWalkGetAttr
	templatefs.NoopFile

	path string
	file *os.File
}

var (
	_ p9.File = &Local{}
)

// info constructs a QID for this file.
func (l *Local) info() (p9.QID, os.FileInfo, error) {
	var (
		qid p9.QID
		fi  os.FileInfo
		err error
	)

	// Stat the file.
	if l.file != nil {
		fi, err = l.file.Stat()
	} else {
		fi, err = os.Lstat(l.path)
	}
	if err != nil {
		return qid, nil, err
	}

	// Construct the QID type.
	qid.Type = p9.ModeFromOS(fi.Mode()).QIDType()

	// Save the path from the Ino.
	ninePath, err := localToQid(l.path, fi)
	if err != nil {
		return qid, nil, err
	}

	qid.Path = ninePath

	return qid, fi, nil
}

// Walk implements p9.File.Walk.
func (l *Local) Walk(names []string) ([]p9.QID, p9.File, error) {
	var qids []p9.QID
	last := &Local{path: l.path}

	// A walk with no names is a copy of self.
	if len(names) == 0 {
		return nil, last, nil
	}

	for _, name := range names {
		c := &Local{path: path.Join(last.path, name)}
		qid, _, err := c.info()
		if err != nil {
			return nil, nil, err
		}
		qids = append(qids, qid)
		last = c
	}
	return qids, last, nil
}

// FSync implements p9.File.FSync.
func (l *Local) FSync() error {
	return l.file.Sync()
}

// GetAttr implements p9.File.GetAttr.
//
// Not fully implemented.
func (l *Local) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	qid, fi, err := l.info()
	if err != nil {
		return qid, p9.AttrMask{}, p9.Attr{}, err
	}

	stat := internal.InfoToStat(fi)
	attr := &p9.Attr{
		Mode:             p9.FileMode(stat.Mode),
		UID:              p9.UID(stat.Uid),
		GID:              p9.GID(stat.Gid),
		NLink:            p9.NLink(stat.Nlink),
		RDev:             p9.Dev(stat.Rdev),
		Size:             uint64(stat.Size),
		BlockSize:        uint64(stat.Blksize),
		Blocks:           uint64(stat.Blocks),
		ATimeSeconds:     uint64(stat.Atim.Sec),
		ATimeNanoSeconds: uint64(stat.Atim.Nsec),
		MTimeSeconds:     uint64(stat.Mtim.Sec),
		MTimeNanoSeconds: uint64(stat.Mtim.Nsec),
		CTimeSeconds:     uint64(stat.Ctim.Sec),
		CTimeNanoSeconds: uint64(stat.Ctim.Nsec),
	}
	return qid, req, *attr, nil
}

// Close implements p9.File.Close.
func (l *Local) Close() error {
	if l.file != nil {
		// We don't set l.file = nil, as Close is called by servers
		// only in Clunk. Clunk should release the last (direct)
		// reference to this file.
		return l.file.Close()
	}
	return nil
}

// Open implements p9.File.Open.
func (l *Local) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	qid, _, err := l.info()
	if err != nil {
		return qid, 0, err
	}

	// Do the actual open.
	f, err := os.OpenFile(l.path, int(mode), 0)
	if err != nil {
		return qid, 0, err
	}
	l.file = f

	return qid, 0, nil
}

// ReadAt implements p9.File.ReadAt.
func (l *Local) ReadAt(p []byte, offset int64) (int, error) {
	return l.file.ReadAt(p, offset)
}

// Lock implements p9.File.Lock.
func (l *Local) Lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	return l.lock(pid, locktype, flags, start, length, client)
}

// WriteAt implements p9.File.WriteAt.
func (l *Local) WriteAt(p []byte, offset int64) (int, error) {
	return l.file.WriteAt(p, offset)
}

// Create implements p9.File.Create.
func (l *Local) Create(name string, mode p9.OpenFlags, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.File, p9.QID, uint32, error) {
	newName := path.Join(l.path, name)
	f, err := os.OpenFile(newName, int(mode)|os.O_CREATE|os.O_EXCL, os.FileMode(permissions))
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	l2 := &Local{path: newName, file: f}
	qid, _, err := l2.info()
	if err != nil {
		l2.Close()
		return nil, p9.QID{}, 0, err
	}
	return l2, qid, 0, nil
}

// Mkdir implements p9.File.Mkdir.
//
// Not properly implemented.
func (l *Local) Mkdir(name string, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := os.Mkdir(path.Join(l.path, name), os.FileMode(permissions)); err != nil {
		return p9.QID{}, err
	}

	// Blank QID.
	return p9.QID{}, nil
}

// Symlink implements p9.File.Symlink.
//
// Not properly implemented.
func (l *Local) Symlink(oldname string, newname string, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := os.Symlink(oldname, path.Join(l.path, newname)); err != nil {
		return p9.QID{}, err
	}

	// Blank QID.
	return p9.QID{}, nil
}

// Link implements p9.File.Link.
//
// Not properly implemented.
func (l *Local) Link(target p9.File, newname string) error {
	return os.Link(target.(*Local).path, path.Join(l.path, newname))
}

// RenameAt implements p9.File.RenameAt.
func (l *Local) RenameAt(oldName string, newDir p9.File, newName string) error {
	oldPath := path.Join(l.path, oldName)
	newPath := path.Join(newDir.(*Local).path, newName)

	return os.Rename(oldPath, newPath)
}

// Readlink implements p9.File.Readlink.
//
// Not properly implemented.
func (l *Local) Readlink() (string, error) {
	return os.Readlink(l.path)
}

// Renamed implements p9.File.Renamed.
func (l *Local) Renamed(parent p9.File, newName string) {
	l.path = path.Join(parent.(*Local).path, newName)
}

// SetAttr implements p9.File.SetAttr.
func (l *Local) SetAttr(valid p9.SetAttrMask, attr p9.SetAttr) error {
	// When truncate(2) is called on Linux, Linux will try to set time & size. Fake it. Sorry.
	supported := p9.SetAttrMask{Size: true, MTime: true, CTime: true, ATime: true}
	if !valid.IsSubsetOf(supported) {
		return linux.ENOSYS
	}

	if valid.Size {
		// If more than one thing is ever implemented, we can't just
		// return an error here.
		return os.Truncate(l.path, int64(attr.Size))
	}
	return nil
}

// UnlinkAt implements p9.File.UnlinkAt
func (l *Local) UnlinkAt(name string, flags uint32) error {
	// Construct the full path
	fullPath := filepath.Join(l.path, name)

	// Remove the file or directory
	return os.Remove(fullPath)
}
