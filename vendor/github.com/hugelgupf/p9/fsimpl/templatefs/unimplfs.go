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

// Package templatefs provides template p9.Files.
//
// NoopFile can be used to leave some methods unimplemented in incomplete
// p9.File implementations.
//
// NilCloser, ReadOnlyFile, NotDirectoryFile, and NotSymlinkFile can be used as
// templates for commonly implemented file types. They are careful not to
// conflict with each others' methods, so they can be embedded together.
package templatefs

import (
	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
)

// NilCloser returns nil for Close.
type NilCloser struct{}

// Close implements p9.File.Close.
func (NilCloser) Close() error {
	return nil
}

// NilSyncer returns nil for FSync.
type NilSyncer struct{}

// FSync implements p9.File.FSync.
func (NilSyncer) FSync() error {
	return nil
}

// NoopRenamed does nothing when the file is renamed.
type NoopRenamed struct{}

// Renamed implements p9.File.Renamed.
func (NoopRenamed) Renamed(parent p9.File, newName string) {}

// NotImplementedFile is a p9.File that returns ENOSYS for every listed method.
//
// Compatible with NoopRenamed, NilCloser, and NilSyncer.
type NotImplementedFile struct {
	p9.DefaultWalkGetAttr
	NotLockable
	XattrUnimplemented
}

// NoopFile is a p9.File with every method unimplemented.
type NoopFile struct {
	NotImplementedFile
	NilCloser
	NilSyncer
	NoopRenamed
}

var (
	_ p9.File = &NoopFile{}
)

// Walk implements p9.File.Walk.
func (NotImplementedFile) Walk(names []string) ([]p9.QID, p9.File, error) {
	return nil, nil, linux.ENOSYS
}

// StatFS implements p9.File.StatFS.
//
// Not implemented.
func (NotImplementedFile) StatFS() (p9.FSStat, error) {
	return p9.FSStat{}, linux.ENOSYS
}

// Open implements p9.File.Open.
func (NotImplementedFile) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	return p9.QID{}, 0, linux.ENOSYS
}

// ReadAt implements p9.File.ReadAt.
func (NotImplementedFile) ReadAt(p []byte, offset int64) (int, error) {
	return 0, linux.ENOSYS
}

// GetAttr implements p9.File.GetAttr.
func (NotImplementedFile) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	return p9.QID{}, p9.AttrMask{}, p9.Attr{}, linux.ENOSYS
}

// SetAttr implements p9.File.SetAttr.
func (NotImplementedFile) SetAttr(valid p9.SetAttrMask, attr p9.SetAttr) error {
	return linux.ENOSYS
}

// Remove implements p9.File.Remove.
func (NotImplementedFile) Remove() error {
	return linux.ENOSYS
}

// Rename implements p9.File.Rename.
func (NotImplementedFile) Rename(directory p9.File, name string) error {
	return linux.ENOSYS
}

// WriteAt implements p9.File.WriteAt.
func (NotImplementedFile) WriteAt(p []byte, offset int64) (int, error) {
	return 0, linux.ENOSYS
}

// Create implements p9.File.Create.
func (NotImplementedFile) Create(name string, mode p9.OpenFlags, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.File, p9.QID, uint32, error) {
	return nil, p9.QID{}, 0, linux.ENOSYS
}

// Mkdir implements p9.File.Mkdir.
func (NotImplementedFile) Mkdir(name string, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.QID, error) {
	return p9.QID{}, linux.ENOSYS
}

// Symlink implements p9.File.Symlink.
func (NotImplementedFile) Symlink(oldname string, newname string, _ p9.UID, _ p9.GID) (p9.QID, error) {
	return p9.QID{}, linux.ENOSYS
}

// Link implements p9.File.Link.
func (NotImplementedFile) Link(target p9.File, newname string) error {
	return linux.ENOSYS
}

// Mknod implements p9.File.Mknod.
func (NotImplementedFile) Mknod(name string, mode p9.FileMode, major uint32, minor uint32, _ p9.UID, _ p9.GID) (p9.QID, error) {
	return p9.QID{}, linux.ENOSYS
}

// RenameAt implements p9.File.RenameAt.
func (NotImplementedFile) RenameAt(oldname string, newdir p9.File, newname string) error {
	return linux.ENOSYS
}

// UnlinkAt implements p9.File.UnlinkAt.
func (NotImplementedFile) UnlinkAt(name string, flags uint32) error {
	return linux.ENOSYS
}

// Readdir implements p9.File.Readdir.
func (NotImplementedFile) Readdir(offset uint64, count uint32) (p9.Dirents, error) {
	return nil, linux.ENOSYS
}

// Readlink implements p9.File.Readlink.
func (NotImplementedFile) Readlink() (string, error) {
	return "", linux.ENOSYS
}

// XattrUnimplemented implements Xattr methods returning ENOSYS.
type XattrUnimplemented struct{}

// SetXattr implements p9.File.SetXattr.
func (XattrUnimplemented) SetXattr(attr string, data []byte, flags p9.XattrFlags) error {
	return linux.ENOSYS
}

// GetXattr implements p9.File.GetXattr.
func (XattrUnimplemented) GetXattr(attr string) ([]byte, error) {
	return nil, linux.ENOSYS
}

// ListXattrs implements p9.File.ListXattrs.
func (XattrUnimplemented) ListXattrs() ([]string, error) {
	return nil, linux.ENOSYS
}

// RemoveXattr implements p9.File.RemoveXattr.
func (XattrUnimplemented) RemoveXattr(attr string) error {
	return linux.ENOSYS
}

type NotLockable struct{}

// Lock implements p9.File.Lock.
func (NotLockable) Lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	return p9.LockStatusOK, linux.ENOSYS
}
