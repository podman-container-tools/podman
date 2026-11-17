//go:build !windows

package stat

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// StatT type contains status of a file. It contains metadata
// like permission, owner, group, size, etc about a file.
//
// Note that this is public as pkg/system.StatT.
type StatT struct {
	mode uint32
	uid  uint32
	gid  uint32
	rdev uint64
	size int64
	mtim syscall.Timespec
	dev  uint64
	platformStatT
}

// Mode returns file's permission mode.
func (s StatT) Mode() uint32 {
	return s.mode
}

// UID returns file's user id of owner.
func (s StatT) UID() uint32 {
	return s.uid
}

// GID returns file's group id of owner.
func (s StatT) GID() uint32 {
	return s.gid
}

// Rdev returns file's device ID (if it's special file).
func (s StatT) Rdev() uint64 {
	return s.rdev
}

// Size returns file's size.
func (s StatT) Size() int64 {
	return s.size
}

// Mtim returns file's last modification time.
func (s StatT) Mtim() syscall.Timespec {
	return s.mtim
}

// Dev returns a unique identifier for owning filesystem
func (s StatT) Dev() uint64 {
	return s.dev
}

func (s StatT) IsDir() bool {
	return (s.mode & unix.S_IFDIR) != 0
}

func (s StatT) IsSymlink() bool {
	return (s.mode & unix.S_IFMT) == unix.S_IFLNK
}

// FromFileInfo converts a os.FileInfo type to a StatT type
func FromFileInfo(fi os.FileInfo) *StatT {
	return FromStatT(fi.Sys().(*syscall.Stat_t))
}
