package stat

import (
	"os"
	"time"
)

// StatT type contains status of a file. It contains metadata
// like permission, size, etc about a file.
//
// Note that this is public as pkg/system.StatT.
type StatT struct {
	mode os.FileMode
	size int64
	mtim time.Time
	platformStatT
}

// Size returns file's size.
func (s StatT) Size() int64 {
	return s.size
}

// Mode returns file's permission mode.
func (s StatT) Mode() os.FileMode {
	return os.FileMode(s.mode)
}

// Mtim returns file's last modification time.
func (s StatT) Mtim() time.Time {
	return time.Time(s.mtim)
}

// UID returns file's user id of owner.
//
// on windows this is always 0 because there is no concept of UID
func (s StatT) UID() uint32 {
	return 0
}

// GID returns file's group id of owner.
//
// on windows this is always 0 because there is no concept of GID
func (s StatT) GID() uint32 {
	return 0
}

// Dev returns a unique identifier for owning filesystem
func (s StatT) Dev() uint64 {
	return 0
}

func (s StatT) IsDir() bool {
	return s.Mode().IsDir()
}

func (s StatT) IsSymlink() bool {
	return s.Mode()&os.ModeSymlink != 0
}

// FromFileInfo converts a os.FileInfo type to a StatT type
func FromFileInfo(fi os.FileInfo) *StatT {
	return &StatT{
		size: fi.Size(),
		mode: fi.Mode(),
		mtim: fi.ModTime(),
	}
}
